package entities

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"
)

// An RDAP publication policy is a TLD's field-access matrix (ADR 0021,
// docs/rdap/PUBLICATION_REFERENCE.md): for each contact role it publishes,
// and each field of that role, whether the field is shown and, if not, how
// it is redacted (RFC 9537). Policies are named, stored, and assigned to
// TLDs, one per TLD; a TLD with none has the built-in for its type. Which
// roles a TLD collects at all is its current GA phase's ContactDataPolicy,
// not this.

// RDAPRole is a contact's role on a domain, as RDAP names entity roles.
type RDAPRole string

const (
	RDAPRoleRegistrant RDAPRole = "registrant"
	RDAPRoleAdmin      RDAPRole = "administrative"
	RDAPRoleTech       RDAPRole = "technical"
	RDAPRoleBilling    RDAPRole = "billing"
)

// RDAPRoles is every contact role, in the order a response lists them.
var RDAPRoles = []RDAPRole{RDAPRoleRegistrant, RDAPRoleAdmin, RDAPRoleTech, RDAPRoleBilling}

// RDAPField is one published element of a contact.
type RDAPField string

const (
	RDAPFieldHandle RDAPField = "handle"
	RDAPFieldName   RDAPField = "name"
	RDAPFieldOrg    RDAPField = "org"
	RDAPFieldStreet RDAPField = "street"
	RDAPFieldCity   RDAPField = "city"
	RDAPFieldSP     RDAPField = "sp"
	RDAPFieldPC     RDAPField = "pc"
	RDAPFieldCC     RDAPField = "cc"
	RDAPFieldVoice  RDAPField = "voice"
	RDAPFieldFax    RDAPField = "fax"
	RDAPFieldEmail  RDAPField = "email"
)

// RDAPFields is every contact field, in the order a response lists them.
var RDAPFields = []RDAPField{RDAPFieldHandle, RDAPFieldName, RDAPFieldOrg, RDAPFieldStreet, RDAPFieldCity, RDAPFieldSP, RDAPFieldPC, RDAPFieldCC, RDAPFieldVoice, RDAPFieldFax, RDAPFieldEmail}

// RDAPRule is what a publication policy says about one field.
type RDAPRule string

const (
	// RDAPPublish publishes the field.
	RDAPPublish RDAPRule = "publish"
	// RDAPPublishIfDisclosed publishes the field only if the contact's RFC
	// 5733 disclose flag for it is set.
	RDAPPublishIfDisclosed RDAPRule = "publish-if-disclosed"
	// RDAPWithhold never publishes the field, and lists it as redacted.
	RDAPWithhold RDAPRule = "withhold"
)

// RDAPRedactionMethod is how a withheld field is redacted (RFC 9537 §3).
type RDAPRedactionMethod string

const (
	RDAPRemoval          RDAPRedactionMethod = "removal"
	RDAPEmptyValue       RDAPRedactionMethod = "emptyValue"
	RDAPReplacementValue RDAPRedactionMethod = "replacementValue"
)

// RDAPFieldRule is a field's rule and, when withheld, how.
type RDAPFieldRule struct {
	Rule   RDAPRule            `json:"rule"`
	Method RDAPRedactionMethod `json:"method,omitempty"`
}

// RDAPPublicationPolicy is a named field-access matrix.
type RDAPPublicationPolicy struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Roles are the roles the policy publishes, each with a rule for every
	// field. A role it does not list is treated as not collected: no entity
	// is served for it and nothing about it is listed as redacted.
	Roles map[RDAPRole]map[RDAPField]RDAPFieldRule `json:"roles"`
	// ContactFormURI replaces a withheld email whose method is
	// replacementValue. Without one, such an email is removed instead.
	ContactFormURI string    `json:"contactFormUri,omitempty"`
	UpdatedAt      time.Time `json:"updatedAt"`
	UpdatedBy      string    `json:"updatedBy,omitempty"`
}

// Built-in RDAP publication policies.
const (
	RDAPPolicyGTLDRDP2024  = "gtld-rdp-2024"
	RDAPPolicyCCTLDMinimal = "cctld-minimal"
)

// Outbox events for RDAP publication policy changes.
const (
	EventRDAPPublicationPolicySaved    = "rdap.publication_policy.saved"
	EventRDAPPublicationPolicyDeleted  = "rdap.publication_policy.deleted"
	EventRDAPPublicationPolicyAssigned = "rdap.publication_policy.assigned"
)

// RDAP publication policy errors.
var (
	ErrInvalidRDAPPublicationPolicy  = errors.New("invalid RDAP publication policy")
	ErrRDAPPublicationPolicyNotFound = errors.New("RDAP publication policy not found")
	ErrRDAPPublicationPolicyBuiltIn  = errors.New("a built-in RDAP publication policy cannot be deleted")
	ErrRDAPPublicationPolicyInUse    = errors.New("the RDAP publication policy is assigned")
)

// RDAPPublicationAssignment is a TLD's assigned policy, as its event records
// it; an empty Policy is the built-in for the TLD's type.
type RDAPPublicationAssignment struct {
	TLD    DomainName `json:"tld"`
	Policy string     `json:"policy"`
}

// RDAPTLDPublication is the publication policy that applies to a TLD.
type RDAPTLDPublication struct {
	TLD DomainName `json:"tld"`
	// Policy is the policy that applies: the assigned one, or the built-in
	// for the TLD's type.
	Policy string `json:"policy"`
	// Assigned is false when the TLD has the built-in by default.
	Assigned bool `json:"assigned"`
}

// NewRDAPPublicationPolicy returns a validated policy.
func NewRDAPPublicationPolicy(name, description string, roles map[RDAPRole]map[RDAPField]RDAPFieldRule, contactFormURI string) (*RDAPPublicationPolicy, error) {
	p := &RDAPPublicationPolicy{Name: name, Description: description, Roles: roles, ContactFormURI: contactFormURI}
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return p, nil
}

// Validate checks the policy. Every published role must give every field a
// rule, so nothing is published or withheld by omission.
func (p *RDAPPublicationPolicy) Validate() error {
	invalid := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidRDAPPublicationPolicy, fmt.Sprintf(format, args...))
	}
	if !policyNamePattern.MatchString(p.Name) {
		return invalid("name %q must be 1 to 32 lower-case letters, digits and hyphens, starting with a letter", p.Name)
	}
	if p.ContactFormURI != "" {
		u, err := url.Parse(p.ContactFormURI)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return invalid("contact form URI %q is not an absolute https URL", p.ContactFormURI)
		}
	}
	for role, fields := range p.Roles {
		if !knownRole(role) {
			return invalid("unknown role %q", role)
		}
		for f := range fields {
			if !knownField(f) {
				return invalid("%s: unknown field %q", role, f)
			}
		}
		for _, f := range RDAPFields {
			r, ok := fields[f]
			if !ok {
				return invalid("%s: no rule for %s", role, f)
			}
			if err := r.validate(f); err != nil {
				return invalid("%s %s: %v", role, f, err)
			}
		}
	}
	return nil
}

func (r RDAPFieldRule) validate(f RDAPField) error {
	switch r.Rule {
	case RDAPPublish:
		if r.Method != "" {
			return errors.New("a published field has no redaction method")
		}
	case RDAPPublishIfDisclosed:
		if f == RDAPFieldHandle {
			return errors.New("a handle has no disclose flag; publish or withhold it")
		}
		if r.Method != "" {
			return errors.New("an undisclosed field is removed; give no method")
		}
	case RDAPWithhold:
		switch r.Method {
		case RDAPRemoval, RDAPEmptyValue:
		case RDAPReplacementValue:
			if f != RDAPFieldEmail {
				return errors.New("only an email can be replaced (by the contact form URI)")
			}
		default:
			return fmt.Errorf("method %q is not removal, emptyValue or replacementValue", r.Method)
		}
		if f == RDAPFieldHandle && r.Method != RDAPRemoval {
			return errors.New("a withheld handle is removed")
		}
		// An empty tel is not a valid jCard property, and the gTLD profile
		// requires withheld phones to be removed.
		if (f == RDAPFieldVoice || f == RDAPFieldFax) && r.Method != RDAPRemoval {
			return errors.New("a withheld phone or fax is removed")
		}
	default:
		return fmt.Errorf("rule %q is not publish, publish-if-disclosed or withhold", r.Rule)
	}
	return nil
}

func knownRole(r RDAPRole) bool {
	for _, k := range RDAPRoles {
		if k == r {
			return true
		}
	}
	return false
}

func knownField(f RDAPField) bool {
	for _, k := range RDAPFields {
		if k == f {
			return true
		}
	}
	return false
}

func rdapWithhold(m RDAPRedactionMethod) RDAPFieldRule {
	return RDAPFieldRule{Rule: RDAPWithhold, Method: m}
}

// BuiltInRDAPPublicationPolicies are gtld-rdp-2024 and cctld-minimal with
// their starting rules, as docs/rdap/PUBLICATION_REFERENCE.md lists them.
// The migration stores them if they are not stored yet; their rules can be
// changed, they cannot be deleted.
func BuiltInRDAPPublicationPolicies() []*RDAPPublicationPolicy {
	publish := RDAPFieldRule{Rule: RDAPPublish}
	gtld := &RDAPPublicationPolicy{
		Name:        RDAPPolicyGTLDRDP2024,
		Description: "ICANN Registration Data Policy baseline",
		Roles: map[RDAPRole]map[RDAPField]RDAPFieldRule{
			RDAPRoleRegistrant: {
				RDAPFieldHandle: rdapWithhold(RDAPRemoval),
				RDAPFieldName:   rdapWithhold(RDAPEmptyValue),
				RDAPFieldOrg:    {Rule: RDAPPublishIfDisclosed},
				RDAPFieldStreet: rdapWithhold(RDAPEmptyValue),
				RDAPFieldCity:   rdapWithhold(RDAPEmptyValue),
				RDAPFieldSP:     publish,
				RDAPFieldPC:     rdapWithhold(RDAPEmptyValue),
				RDAPFieldCC:     publish,
				RDAPFieldVoice:  rdapWithhold(RDAPRemoval),
				RDAPFieldFax:    rdapWithhold(RDAPRemoval),
				RDAPFieldEmail:  rdapWithhold(RDAPReplacementValue),
			},
			RDAPRoleTech: {
				RDAPFieldHandle: rdapWithhold(RDAPRemoval),
				RDAPFieldName:   rdapWithhold(RDAPEmptyValue),
				RDAPFieldOrg:    rdapWithhold(RDAPRemoval),
				RDAPFieldStreet: rdapWithhold(RDAPRemoval),
				RDAPFieldCity:   rdapWithhold(RDAPRemoval),
				RDAPFieldSP:     rdapWithhold(RDAPRemoval),
				RDAPFieldPC:     rdapWithhold(RDAPRemoval),
				RDAPFieldCC:     rdapWithhold(RDAPRemoval),
				RDAPFieldVoice:  rdapWithhold(RDAPRemoval),
				RDAPFieldFax:    rdapWithhold(RDAPRemoval),
				RDAPFieldEmail:  rdapWithhold(RDAPReplacementValue),
			},
		},
	}
	// The country is published in the jCard's address and a jCard always
	// has a name, so the name and the other address parts are kept empty.
	registrant := map[RDAPField]RDAPFieldRule{}
	for _, f := range RDAPFields {
		registrant[f] = rdapWithhold(RDAPRemoval)
	}
	for _, f := range []RDAPField{RDAPFieldName, RDAPFieldStreet, RDAPFieldCity, RDAPFieldSP, RDAPFieldPC} {
		registrant[f] = rdapWithhold(RDAPEmptyValue)
	}
	registrant[RDAPFieldCC] = publish
	cc := &RDAPPublicationPolicy{
		Name:        RDAPPolicyCCTLDMinimal,
		Description: "Registrant country only",
		Roles:       map[RDAPRole]map[RDAPField]RDAPFieldRule{RDAPRoleRegistrant: registrant},
	}
	return []*RDAPPublicationPolicy{gtld, cc}
}

// IsBuiltInRDAPPublicationPolicy reports whether name is a built-in policy.
func IsBuiltInRDAPPublicationPolicy(name string) bool {
	return name == RDAPPolicyGTLDRDP2024 || name == RDAPPolicyCCTLDMinimal
}

// RDAPPublicationSet is every policy and assignment, as an RDAP node holds
// them between reloads.
type RDAPPublicationSet struct {
	policies    map[string]*RDAPPublicationPolicy
	assignments map[DomainName]string
	LoadedAt    time.Time
}

// NewRDAPPublicationSet builds a set. A built-in policy not stored is
// included with its starting rules.
func NewRDAPPublicationSet(policies []*RDAPPublicationPolicy, assignments map[DomainName]string, loadedAt time.Time) *RDAPPublicationSet {
	s := &RDAPPublicationSet{policies: map[string]*RDAPPublicationPolicy{}, assignments: assignments, LoadedAt: loadedAt}
	for _, b := range BuiltInRDAPPublicationPolicies() {
		s.policies[b.Name] = b
	}
	for _, p := range policies {
		s.policies[p.Name] = p
	}
	if s.assignments == nil {
		s.assignments = map[DomainName]string{}
	}
	return s
}

// DefaultRDAPPublicationSet is the set before any is loaded: the built-ins,
// assigned to nothing.
func DefaultRDAPPublicationSet() *RDAPPublicationSet {
	return NewRDAPPublicationSet(nil, nil, time.Time{})
}

// BuiltInRDAPPublicationPolicyFor names the built-in a TLD has when it is
// assigned none: gtld-rdp-2024 for a generic TLD, cctld-minimal for any
// other.
//
// The TLD type is inferred from the name (two letters is a ccTLD), which
// files an IDN ccTLD such as xn--p1ai as generic. When the type is
// uncertain the less revealing policy is the safe one, so an IDN TLD has
// cctld-minimal unless a policy is assigned to it.
func BuiltInRDAPPublicationPolicyFor(name DomainName, t TLDType) string {
	if t == TLDTypeGTLD && !strings.HasPrefix(strings.ToLower(string(name)), "xn--") {
		return RDAPPolicyGTLDRDP2024
	}
	return RDAPPolicyCCTLDMinimal
}

// ForTLD is the policy that applies to the TLD: its assigned one, or the
// built-in for its type.
func (s *RDAPPublicationSet) ForTLD(tld DomainName, t TLDType) *RDAPPublicationPolicy {
	if name, ok := s.assignments[tld]; ok {
		if p, ok := s.policies[name]; ok {
			return p
		}
	}
	return s.policies[BuiltInRDAPPublicationPolicyFor(tld, t)]
}

// UnresolvedAssignments lists, as "tld=policy", the assignments that name a
// policy the set does not hold. A set with any must not be used: ForTLD
// would serve those TLDs the built-in in place of their policy.
func (s *RDAPPublicationSet) UnresolvedAssignments() []string {
	var out []string
	for tld, name := range s.assignments {
		if _, ok := s.policies[name]; !ok {
			out = append(out, string(tld)+"="+name)
		}
	}
	sort.Strings(out)
	return out
}

// Policies returns every policy, by name.
func (s *RDAPPublicationSet) Policies() []*RDAPPublicationPolicy {
	out := make([]*RDAPPublicationPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// RemoveEmptiedPhones turns every withheld phone or fax redacted by
// emptyValue into one redacted by removal: an empty tel is not valid jCard.
// Policies stored before that rule are upgraded with it. It reports whether
// anything changed.
func (p *RDAPPublicationPolicy) RemoveEmptiedPhones() bool {
	changed := false
	for _, fields := range p.Roles {
		for _, f := range []RDAPField{RDAPFieldVoice, RDAPFieldFax} {
			if r, ok := fields[f]; ok && r.Rule == RDAPWithhold && r.Method == RDAPEmptyValue {
				r.Method = RDAPRemoval
				fields[f] = r
				changed = true
			}
		}
	}
	return changed
}
