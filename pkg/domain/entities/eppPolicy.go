package entities

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
)

// EPP policies: the limits the EPP server holds each registrar to
// (docs/epp/ARCHITECTURE.md §3–§5, ADR 0019). They are data, changed
// through the admin API and reloaded by every EPP node within its refresh
// interval, not configuration.
//
//   - A policy is a named set of limits. Two are built in and always exist:
//     "default", for every registrar not assigned another, and "probation",
//     for new registrars. Any other is a custom policy. The built-in
//     policies' limits can be changed but they cannot be deleted; their
//     starting values are the code constants below.
//   - A registrar is assigned one policy, or none for "default". There are
//     no per-registrar overrides: a registrar that needs other limits gets a
//     policy of its own.
//   - A registrar family groups affiliated registrars (often the
//     accreditations of one company) so that limits apply to them together
//     as well as to each. A registrar is in at most one family. A family is
//     assigned a policy too, whose family limits apply to the family; its
//     members' own policies apply to each of them.
//   - A login's own session cap (EPPLogin.SessionCap), when set, replaces
//     its registrar's policy's per-login cap.

// Outbox events for policy changes. Each carries the object after the
// change as its data, and before and after as its states.
const (
	EventEPPPolicySaved         = "epp.policy.saved"
	EventEPPPolicyDeleted       = "epp.policy.deleted"
	EventEPPPolicyAssigned      = "epp.policy.assigned"
	EventRegistrarFamilySaved   = "epp.family.saved"
	EventRegistrarFamilyDeleted = "epp.family.deleted"
)

// EPPPolicyAssignment is a registrar's assigned policy, as its
// epp.policy.assigned event records it; an empty Policy is "default".
type EPPPolicyAssignment struct {
	ClID   ClIDType `json:"clid"`
	Policy string   `json:"policy"`
}

// Built-in policy names.
const (
	EPPPolicyDefault   = "default"
	EPPPolicyProbation = "probation"
)

// Policy errors.
var (
	ErrInvalidEPPPolicy        = errors.New("invalid EPP policy")
	ErrEPPPolicyNotFound       = errors.New("EPP policy not found")
	ErrEPPPolicyBuiltIn        = errors.New("a built-in EPP policy cannot be deleted")
	ErrEPPPolicyInUse          = errors.New("the EPP policy is assigned")
	ErrInvalidRegistrarFamily  = errors.New("invalid registrar family")
	ErrRegistrarFamilyNotFound = errors.New("registrar family not found")
	ErrRegistrarInFamily       = errors.New("the registrar is in another family")
)

// policyNamePattern is a policy's or family's name: lower-case letters,
// digits and hyphens, starting with a letter.
var policyNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// EPPPolicyLimits are a policy's limits. Every one is at least 1.
type EPPPolicyLimits struct {
	// SessionsPerRegistrar caps a registrar's logged-in sessions, across
	// every node and login.
	SessionsPerRegistrar int `json:"sessions_per_registrar"`
	// SessionsPerLogin caps one login's sessions, unless the login has a
	// session cap of its own.
	SessionsPerLogin int `json:"sessions_per_login"`
	// SessionsPerFamily caps a family's sessions, across all its members.
	// It applies through the family's policy only.
	SessionsPerFamily int `json:"sessions_per_family"`
	// CheckNames is the most names one <check> may carry (2306 over it).
	// Availability is metered per name by the rate limits, so this bounds
	// round trips, not capacity.
	CheckNames int `json:"check_names"`
	// Rates are the registrar's command rates, one token bucket per class.
	Rates EPPCommandRates `json:"rates"`
	// FamilyRates are a family's command rates, shared by all its members.
	// They apply through the family's policy only.
	FamilyRates EPPCommandRates `json:"family_rates"`
	// CreatesPerDay caps the registrar's domain creates per UTC day; 0 is
	// no cap. Only creates that succeed count.
	CreatesPerDay int `json:"creates_per_day"`
	// FailedCreates is when the registrar's domain creates of names that
	// are already registered (2302) are a dropcatch run, and escalated.
	FailedCreates EPPFailedCreateBudget `json:"failed_creates"`
	// FamilyFailedCreates is the same for a family, across its members. It
	// applies through the family's policy only.
	FamilyFailedCreates EPPFailedCreateBudget `json:"family_failed_creates"`
}

// EPPFailedCreateBudget is when domain creates refused because the name is
// registered (2302) are over budget, over the last five minutes: at least
// Failures of them, making up at least RatioPercent of the creates that
// were answered 2302 or created a domain. Both must hold, so a busy
// registrar with some failures, or a quiet one with a few, is within it.
type EPPFailedCreateBudget struct {
	Failures     int `json:"failures"`
	RatioPercent int `json:"ratio_percent"`
}

// Over reports whether failed and succeeded creates are over the budget.
func (b EPPFailedCreateBudget) Over(failed, succeeded int) bool {
	return failed >= b.Failures && failed*100 >= b.RatioPercent*(failed+succeeded)
}

func (b EPPFailedCreateBudget) validate(prefix string) []string {
	var bad []string
	if b.Failures < 1 {
		bad = append(bad, prefix+".failures must be at least 1")
	}
	if b.RatioPercent < 1 || b.RatioPercent > 100 {
		bad = append(bad, prefix+".ratio_percent must be 1 to 100")
	}
	return bad
}

// EPPRate is a token bucket: PerSecond tokens a second, holding at most
// Burst. A full bucket lets Burst tokens through at once. Both are at
// least 1.
type EPPRate struct {
	PerSecond int `json:"per_second"`
	Burst     int `json:"burst"`
}

// EPPCommandRates are a bucket per command class. A check costs one token
// per name it carries; every other command costs one.
type EPPCommandRates struct {
	Check     EPPRate `json:"check"`
	Info      EPPRate `json:"info"`
	Transform EPPRate `json:"transform"`
	Poll      EPPRate `json:"poll"`
}

// Times is r with every rate and burst multiplied by n.
func (r EPPCommandRates) Times(n int) EPPCommandRates {
	m := func(x EPPRate) EPPRate { return EPPRate{PerSecond: x.PerSecond * n, Burst: x.Burst * n} }
	return EPPCommandRates{Check: m(r.Check), Info: m(r.Info), Transform: m(r.Transform), Poll: m(r.Poll)}
}

// validate names each rate below 1, and a check burst that cannot hold a
// full check.
func (r EPPCommandRates) validate(prefix string, checkNames int) []string {
	var bad []string
	for _, c := range []struct {
		name string
		r    EPPRate
	}{{"check", r.Check}, {"info", r.Info}, {"transform", r.Transform}, {"poll", r.Poll}} {
		if c.r.PerSecond < 1 {
			bad = append(bad, prefix+"."+c.name+".per_second must be at least 1")
		}
		if c.r.Burst < 1 {
			bad = append(bad, prefix+"."+c.name+".burst must be at least 1")
		}
	}
	if r.Check.Burst >= 1 && checkNames >= 1 && r.Check.Burst < checkNames {
		bad = append(bad, fmt.Sprintf("%s.check.burst must be at least check_names (%d), or a full check could never pass", prefix, checkNames))
	}
	return bad
}

// DefaultEPPPolicyLimits are the built-in "default" policy's starting
// limits (docs/epp/POLICY_REFERENCE.md).
func DefaultEPPPolicyLimits() EPPPolicyLimits {
	rates := EPPCommandRates{
		Check: EPPRate{PerSecond: 250, Burst: 1000}, Info: EPPRate{PerSecond: 20, Burst: 100},
		Transform: EPPRate{PerSecond: 10, Burst: 50}, Poll: EPPRate{PerSecond: 5, Burst: 20},
	}
	return EPPPolicyLimits{
		SessionsPerRegistrar: 10, SessionsPerLogin: 10, SessionsPerFamily: 40, CheckNames: 50,
		Rates: rates, FamilyRates: rates.Times(3),
		FailedCreates:       EPPFailedCreateBudget{Failures: 100, RatioPercent: 90},
		FamilyFailedCreates: EPPFailedCreateBudget{Failures: 300, RatioPercent: 90},
	}
}

// ProbationEPPPolicyLimits are the built-in "probation" policy's starting
// limits: a new registrar's, until it has shown how it uses the registry.
// Its sessions and check names are the default's; its family cap, command
// rates and failed-create budget are lower, and it may create 100 domains
// a day.
func ProbationEPPPolicyLimits() EPPPolicyLimits {
	rates := EPPCommandRates{
		Check: EPPRate{PerSecond: 100, Burst: 500}, Info: EPPRate{PerSecond: 10, Burst: 50},
		Transform: EPPRate{PerSecond: 5, Burst: 25}, Poll: EPPRate{PerSecond: 5, Burst: 20},
	}
	return EPPPolicyLimits{
		SessionsPerRegistrar: 10, SessionsPerLogin: 10, SessionsPerFamily: 30, CheckNames: 50,
		Rates: rates, FamilyRates: rates.Times(3), CreatesPerDay: 100,
		FailedCreates:       EPPFailedCreateBudget{Failures: 50, RatioPercent: 90},
		FamilyFailedCreates: EPPFailedCreateBudget{Failures: 150, RatioPercent: 90},
	}
}

// Validate checks every limit is at least 1, except CreatesPerDay, which is
// at least 0, and that each check burst holds a full check.
func (l EPPPolicyLimits) Validate() error {
	var bad []string
	for _, f := range []struct {
		name string
		v    int
	}{
		{"sessions_per_registrar", l.SessionsPerRegistrar}, {"sessions_per_login", l.SessionsPerLogin},
		{"sessions_per_family", l.SessionsPerFamily}, {"check_names", l.CheckNames},
	} {
		if f.v < 1 {
			bad = append(bad, f.name+" must be at least 1")
		}
	}
	bad = append(bad, l.Rates.validate("rates", l.CheckNames)...)
	bad = append(bad, l.FamilyRates.validate("family_rates", l.CheckNames)...)
	if l.CreatesPerDay < 0 {
		bad = append(bad, "creates_per_day must be at least 0")
	}
	bad = append(bad, l.FailedCreates.validate("failed_creates")...)
	bad = append(bad, l.FamilyFailedCreates.validate("family_failed_creates")...)
	if len(bad) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidEPPPolicy, strings.Join(bad, "; "))
	}
	return nil
}

// EPPPolicy is a named set of limits.
type EPPPolicy struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Limits      EPPPolicyLimits `json:"limits"`
	UpdatedAt   time.Time       `json:"updated_at"`
	UpdatedBy   string          `json:"updated_by,omitempty"`
}

// NewEPPPolicy returns a validated policy.
func NewEPPPolicy(name, description string, limits EPPPolicyLimits) (*EPPPolicy, error) {
	if !policyNamePattern.MatchString(name) {
		return nil, fmt.Errorf("%w: the name must be 1 to 32 lower-case letters, digits and hyphens, starting with a letter", ErrInvalidEPPPolicy)
	}
	if err := limits.Validate(); err != nil {
		return nil, err
	}
	return &EPPPolicy{Name: name, Description: strings.TrimSpace(description), Limits: limits}, nil
}

// BuiltIn reports whether p is one of the built-in policies.
func (p *EPPPolicy) BuiltIn() bool { return IsBuiltInEPPPolicy(p.Name) }

// IsBuiltInEPPPolicy reports whether name is a built-in policy's.
func IsBuiltInEPPPolicy(name string) bool {
	return name == EPPPolicyDefault || name == EPPPolicyProbation
}

// BuiltInEPPPolicies are the built-in policies with their starting limits.
func BuiltInEPPPolicies() []*EPPPolicy {
	return []*EPPPolicy{
		{Name: EPPPolicyDefault, Description: "Every registrar not assigned another policy", Limits: DefaultEPPPolicyLimits()},
		{Name: EPPPolicyProbation, Description: "New registrars, until they have shown how they use the registry", Limits: ProbationEPPPolicyLimits()},
	}
}

// RegistrarFamily is a group of affiliated registrars, limited together by
// its policy's family limits.
type RegistrarFamily struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Policy is the family's policy; empty is "default".
	Policy    string     `json:"policy,omitempty"`
	Members   []ClIDType `json:"members"`
	UpdatedAt time.Time  `json:"updated_at"`
	UpdatedBy string     `json:"updated_by,omitempty"`
}

// NewRegistrarFamily returns a validated family, its members sorted and
// without repeats.
func NewRegistrarFamily(name, description, policy string, members []ClIDType) (*RegistrarFamily, error) {
	if !policyNamePattern.MatchString(name) {
		return nil, fmt.Errorf("%w: the name must be 1 to 32 lower-case letters, digits and hyphens, starting with a letter", ErrInvalidRegistrarFamily)
	}
	if policy != "" && !policyNamePattern.MatchString(policy) {
		return nil, fmt.Errorf("%w: no such policy name %q", ErrInvalidRegistrarFamily, policy)
	}
	out := make([]ClIDType, 0, len(members))
	for _, m := range members {
		if err := m.Validate(); err != nil {
			return nil, fmt.Errorf("%w: member %q: %w", ErrInvalidRegistrarFamily, m, err)
		}
		out = append(out, m)
	}
	slices.Sort(out)
	out = slices.Compact(out)
	return &RegistrarFamily{Name: name, Description: strings.TrimSpace(description), Policy: policy, Members: out}, nil
}

// PolicyName is the family's policy, "default" when none is assigned.
func (f *RegistrarFamily) PolicyName() string {
	if f.Policy == "" {
		return EPPPolicyDefault
	}
	return f.Policy
}

// EPPRegistrarPolicy is what applies to one registrar: its policy, and its
// family with that family's policy, if it is in one.
type EPPRegistrarPolicy struct {
	Policy       *EPPPolicy       `json:"policy"`
	Family       *RegistrarFamily `json:"family,omitempty"`
	FamilyPolicy *EPPPolicy       `json:"family_policy,omitempty"`
}

// EPPPolicySet is every policy, assignment and family, as an EPP node holds
// them between refreshes. Build it with NewEPPPolicySet.
type EPPPolicySet struct {
	policies map[string]*EPPPolicy
	assigned map[ClIDType]string
	families map[string]*RegistrarFamily
	familyOf map[ClIDType]*RegistrarFamily
	LoadedAt time.Time
}

// NewEPPPolicySet indexes policies, the registrars' assignments and the
// families. A built-in policy missing from policies takes its starting
// limits, and an assignment to a policy that does not exist falls back to
// "default", so the set always answers.
func NewEPPPolicySet(policies []*EPPPolicy, assignments map[ClIDType]string, families []*RegistrarFamily, loadedAt time.Time) *EPPPolicySet {
	s := &EPPPolicySet{
		policies: map[string]*EPPPolicy{}, assigned: map[ClIDType]string{},
		families: map[string]*RegistrarFamily{}, familyOf: map[ClIDType]*RegistrarFamily{}, LoadedAt: loadedAt,
	}
	for _, p := range BuiltInEPPPolicies() {
		s.policies[p.Name] = p
	}
	for _, p := range policies {
		s.policies[p.Name] = p
	}
	for clid, name := range assignments {
		s.assigned[clid] = name
	}
	for _, f := range families {
		s.families[f.Name] = f
		for _, m := range f.Members {
			s.familyOf[m] = f
		}
	}
	return s
}

// DefaultEPPPolicySet is the set before anything has been loaded: the
// built-in policies with their starting limits, no assignments, no families.
func DefaultEPPPolicySet() *EPPPolicySet { return NewEPPPolicySet(nil, nil, nil, time.Time{}) }

// policy returns the named policy, or "default" if there is none.
func (s *EPPPolicySet) policy(name string) *EPPPolicy {
	if p, ok := s.policies[name]; ok {
		return p
	}
	return s.policies[EPPPolicyDefault]
}

// ForRegistrar is what applies to clid.
func (s *EPPPolicySet) ForRegistrar(clid ClIDType) EPPRegistrarPolicy {
	out := EPPRegistrarPolicy{Policy: s.policy(s.assigned[clid])}
	if f, ok := s.familyOf[clid]; ok {
		out.Family, out.FamilyPolicy = f, s.policy(f.PolicyName())
	}
	return out
}

// Policies lists the set's policies by name.
func (s *EPPPolicySet) Policies() []*EPPPolicy {
	out := make([]*EPPPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b *EPPPolicy) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// EPPSessionLimits are the session caps for one login: its registrar's and
// its own, and its family's when it is in one (Family empty otherwise).
type EPPSessionLimits struct {
	PerRegistrar int
	PerLogin     int
	Family       string
	PerFamily    int
}

// SessionLimits are the caps a login of clid with its own session cap
// loginCap (0 for none) logs in under.
func (r EPPRegistrarPolicy) SessionLimits(loginCap int) EPPSessionLimits {
	l := EPPSessionLimits{PerRegistrar: r.Policy.Limits.SessionsPerRegistrar, PerLogin: r.Policy.Limits.SessionsPerLogin}
	if loginCap > 0 {
		l.PerLogin = loginCap
	}
	if r.Family != nil {
		l.Family, l.PerFamily = r.Family.Name, r.FamilyPolicy.Limits.SessionsPerFamily
	}
	return l
}
