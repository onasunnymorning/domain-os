package entities

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Lifecycle policies (ADR 0023): the timing of a domain's lifecycle, as a
// named, platform-wide record that a TLD phase names instead of carrying a
// copy. The states and transitions are code (domain.go, transfer.go,
// restore.go); a policy sets how long each period lasts and two switches.
//
//   - A policy is a named set of settings. One is built in and always
//     exists: "gtld-standard", the registry's defaults, which NewPhase
//     assigns. It can be changed; it cannot be deleted.
//   - A setting left nil is the policy's default (the gtld-standard value
//     below); 0 is zero days. A zero-day grace period is a plain setting.
//   - A phase names one policy. A policy a phase names cannot be deleted.
//   - Changing a policy needs the platform permission registry:policy:admin;
//     naming one on a phase is a phase edit under the operator's tenant.

// Default lifecycle settings: the gtld-standard policy's values, and what a
// setting left nil resolves to.
const (
	RegistrationGP     = 5
	RenewalGP          = 5
	AutoRenewalGP      = 45
	TransferGP         = 5
	RedemptionGP       = 30
	PendingDeleteGP    = 5
	TransferLockPeriod = 60
	// PendingTransferPeriod is how many days the losing registrar has to
	// approve or reject a transfer before the registry approves it.
	PendingTransferPeriod = 5
	// PendingRestorePeriod is how many days a registrar has to send the
	// restore report after requesting a restore (RFC 3915 pendingRestore;
	// 7 days, as .com).
	PendingRestorePeriod  = 7
	MaxHorizon            = 10
	AllowAutoRenew        = true
	RestoreReportRequired = true
)

// Upper bounds on each duration (ADR 0023 decision 4). A value over its
// bound is a typo, not a policy: a redemption period over a year, a
// transfer lock over a year, a pending transfer or restore window over a
// month, a pending delete over a quarter.
const (
	MaxGraceDays           = 365
	MaxTransferLockDays    = 365
	MaxPendingTransferDays = 30
	MaxRedemptionDays      = 365
	MaxPendingDeleteDays   = 90
	MaxPendingRestoreDays  = 30
	MinHorizonYears        = 1
	MaxHorizonYears        = 10
)

// LifecyclePolicyStandard is the built-in policy's name.
const LifecyclePolicyStandard = "gtld-standard"

// Outbox events for lifecycle policy changes. Each carries the policy after
// the change as its data, and before and after as its states.
const (
	EventLifecyclePolicyCreated = "lifecycle_policy.created"
	EventLifecyclePolicyUpdated = "lifecycle_policy.updated"
	EventLifecyclePolicyDeleted = "lifecycle_policy.deleted"
)

// Lifecycle policy errors.
var (
	ErrInvalidLifecyclePolicy  = errors.New("invalid lifecycle policy")
	ErrLifecyclePolicyNotFound = errors.New("lifecycle policy not found")
	ErrLifecyclePolicyBuiltIn  = errors.New("the built-in lifecycle policy cannot be deleted")
	ErrLifecyclePolicyInUse    = errors.New("the lifecycle policy is named by a phase")
	// ErrLifecyclePolicyExists is a create-only save of a policy that exists.
	ErrLifecyclePolicyExists = errors.New("lifecycle policy already exists")
	// ErrLifecyclePolicyChanged is a conditional save whose expected version
	// is not the one stored: someone saved in between.
	ErrLifecyclePolicyChanged = errors.New("lifecycle policy changed since it was read")
)

// LifecycleSettings are a policy's settings as stored. Durations are whole
// days; nil is the default, 0 is zero days. MaxHorizon is years.
type LifecycleSettings struct {
	// RegistrationGP: days after a create in which a delete earns a refund
	// and skips redemption (RFC 3915 addPeriod).
	RegistrationGP *int `json:"registrationGP"`
	// RenewalGP: days after a renew in which a delete earns its refund
	// (renewPeriod).
	RenewalGP *int `json:"renewalGP"`
	// AutoRenewalGP: the same after an auto-renew (autoRenewPeriod).
	AutoRenewalGP *int `json:"autorenewalGP"`
	// TransferGP: the same after a transfer (transferPeriod).
	TransferGP *int `json:"transferGP"`
	// TransferLockPeriod: days after a registration or a transfer in which
	// the domain cannot be transferred; 0 is no lock.
	TransferLockPeriod *int `json:"transferLockPeriod"`
	// PendingTransferPeriod: days the losing registrar has to answer a
	// transfer request before the registry approves it.
	PendingTransferPeriod *int `json:"pendingTransferPeriod"`
	// RedemptionGP: days a deleted domain is restorable.
	RedemptionGP *int `json:"redemptionGP"`
	// PendingDeleteGP: days after redemption until the purge.
	PendingDeleteGP *int `json:"pendingdeleteGP"`
	// PendingRestorePeriod: days a registrar has to send the restore report
	// after requesting a restore.
	PendingRestorePeriod *int `json:"pendingRestorePeriod"`
	// RestoreReportRequired: whether a restore waits for the registrar's
	// restore report; false restores a domain on its request alone.
	RestoreReportRequired *bool `json:"restoreReportRequired"`
	// AllowAutoRenew: whether the expiry loop may auto-renew in the TLD.
	AllowAutoRenew *bool `json:"allowAutorenew"`
	// MaxHorizon: the furthest expiry, in years from now, a registration,
	// renewal or transfer may reach.
	MaxHorizon *int `json:"maxHorizon"`
}

// ResolvedLifecycle is a policy with every setting resolved: what the
// lifecycle code reads.
type ResolvedLifecycle struct {
	RegistrationGP        int  `json:"registrationGP"`
	RenewalGP             int  `json:"renewalGP"`
	AutoRenewalGP         int  `json:"autorenewalGP"`
	TransferGP            int  `json:"transferGP"`
	TransferLockPeriod    int  `json:"transferLockPeriod"`
	PendingTransferPeriod int  `json:"pendingTransferPeriod"`
	RedemptionGP          int  `json:"redemptionGP"`
	PendingDeleteGP       int  `json:"pendingdeleteGP"`
	PendingRestorePeriod  int  `json:"pendingRestorePeriod"`
	RestoreReportRequired bool `json:"restoreReportRequired"`
	AllowAutoRenew        bool `json:"allowAutorenew"`
	MaxHorizon            int  `json:"maxHorizon"`
}

// Settings are r as explicit settings, every one set.
func (r ResolvedLifecycle) Settings() LifecycleSettings {
	i := func(v int) *int { return &v }
	b := func(v bool) *bool { return &v }
	return LifecycleSettings{
		RegistrationGP: i(r.RegistrationGP), RenewalGP: i(r.RenewalGP), AutoRenewalGP: i(r.AutoRenewalGP),
		TransferGP: i(r.TransferGP), TransferLockPeriod: i(r.TransferLockPeriod), PendingTransferPeriod: i(r.PendingTransferPeriod),
		RedemptionGP: i(r.RedemptionGP), PendingDeleteGP: i(r.PendingDeleteGP), PendingRestorePeriod: i(r.PendingRestorePeriod),
		RestoreReportRequired: b(r.RestoreReportRequired), AllowAutoRenew: b(r.AllowAutoRenew), MaxHorizon: i(r.MaxHorizon),
	}
}

// DefaultLifecycle is the gtld-standard policy's values: what every nil
// setting resolves to.
func DefaultLifecycle() ResolvedLifecycle {
	return ResolvedLifecycle{
		RegistrationGP: RegistrationGP, RenewalGP: RenewalGP, AutoRenewalGP: AutoRenewalGP, TransferGP: TransferGP,
		TransferLockPeriod: TransferLockPeriod, PendingTransferPeriod: PendingTransferPeriod,
		RedemptionGP: RedemptionGP, PendingDeleteGP: PendingDeleteGP, PendingRestorePeriod: PendingRestorePeriod,
		RestoreReportRequired: RestoreReportRequired, AllowAutoRenew: AllowAutoRenew, MaxHorizon: MaxHorizon,
	}
}

func orInt(v *int, def int) int {
	if v == nil {
		return def
	}
	return *v
}

func orBool(v *bool, def bool) bool {
	if v == nil {
		return def
	}
	return *v
}

// Resolve fills every nil setting with its default.
func (s LifecycleSettings) Resolve() ResolvedLifecycle {
	d := DefaultLifecycle()
	return ResolvedLifecycle{
		RegistrationGP:        orInt(s.RegistrationGP, d.RegistrationGP),
		RenewalGP:             orInt(s.RenewalGP, d.RenewalGP),
		AutoRenewalGP:         orInt(s.AutoRenewalGP, d.AutoRenewalGP),
		TransferGP:            orInt(s.TransferGP, d.TransferGP),
		TransferLockPeriod:    orInt(s.TransferLockPeriod, d.TransferLockPeriod),
		PendingTransferPeriod: orInt(s.PendingTransferPeriod, d.PendingTransferPeriod),
		RedemptionGP:          orInt(s.RedemptionGP, d.RedemptionGP),
		PendingDeleteGP:       orInt(s.PendingDeleteGP, d.PendingDeleteGP),
		PendingRestorePeriod:  orInt(s.PendingRestorePeriod, d.PendingRestorePeriod),
		RestoreReportRequired: orBool(s.RestoreReportRequired, d.RestoreReportRequired),
		AllowAutoRenew:        orBool(s.AllowAutoRenew, d.AllowAutoRenew),
		MaxHorizon:            orInt(s.MaxHorizon, d.MaxHorizon),
	}
}

// Validate checks every duration given is within its bounds and maxHorizon
// is MinHorizonYears to MaxHorizonYears. A nil setting is the default and
// always valid.
func (s LifecycleSettings) Validate() error {
	var bad []string
	for _, f := range []struct {
		name     string
		v        *int
		min, max int
	}{
		{"registrationGP", s.RegistrationGP, 0, MaxGraceDays},
		{"renewalGP", s.RenewalGP, 0, MaxGraceDays},
		{"autorenewalGP", s.AutoRenewalGP, 0, MaxGraceDays},
		{"transferGP", s.TransferGP, 0, MaxGraceDays},
		{"transferLockPeriod", s.TransferLockPeriod, 0, MaxTransferLockDays},
		{"pendingTransferPeriod", s.PendingTransferPeriod, 0, MaxPendingTransferDays},
		{"redemptionGP", s.RedemptionGP, 0, MaxRedemptionDays},
		{"pendingdeleteGP", s.PendingDeleteGP, 0, MaxPendingDeleteDays},
		{"pendingRestorePeriod", s.PendingRestorePeriod, 0, MaxPendingRestoreDays},
		{"maxHorizon", s.MaxHorizon, MinHorizonYears, MaxHorizonYears},
	} {
		if f.v != nil && (*f.v < f.min || *f.v > f.max) {
			bad = append(bad, fmt.Sprintf("%s must be %d to %d", f.name, f.min, f.max))
		}
	}
	// A report cannot be sent within a zero-day window: a policy that
	// requires one needs a window to send it in.
	if r := s.Resolve(); r.RestoreReportRequired && r.PendingRestorePeriod == 0 {
		bad = append(bad, "pendingRestorePeriod must be at least 1 while restoreReportRequired is true")
	}
	if len(bad) > 0 {
		return fmt.Errorf("%w: %s", ErrInvalidLifecyclePolicy, strings.Join(bad, "; "))
	}
	return nil
}

// LifecyclePolicy is a named set of lifecycle settings.
type LifecyclePolicy struct {
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Settings    LifecycleSettings `json:"settings"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	UpdatedBy   string            `json:"updatedBy,omitempty"`
	// UsedBy is the phases that name the policy, filled in by the service
	// on a read so a change's reach is visible before it is made; it is
	// not stored.
	UsedBy []LifecyclePolicyUse `json:"usedBy,omitempty"`
}

// MarshalJSON adds the resolved settings beside the stored ones, so a
// reader need not know the defaults.
func (p LifecyclePolicy) MarshalJSON() ([]byte, error) {
	type plain LifecyclePolicy
	return json.Marshal(struct {
		plain
		Resolved ResolvedLifecycle `json:"resolved"`
	}{plain(p), p.Resolved()})
}

// UnmarshalJSON reads what MarshalJSON writes; "resolved" is ignored.
func (p *LifecyclePolicy) UnmarshalJSON(b []byte) error {
	type plain LifecyclePolicy
	var v plain
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*p = LifecyclePolicy(v)
	return nil
}

// NewLifecyclePolicy returns a validated policy. The name is 1 to 32
// lower-case letters, digits and hyphens, starting with a letter, as an EPP
// policy's.
func NewLifecyclePolicy(name, description string, settings LifecycleSettings) (*LifecyclePolicy, error) {
	if !policyNamePattern.MatchString(name) {
		return nil, fmt.Errorf("%w: the name must be 1 to 32 lower-case letters, digits and hyphens, starting with a letter", ErrInvalidLifecyclePolicy)
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	return &LifecyclePolicy{Name: name, Description: strings.TrimSpace(description), Settings: settings}, nil
}

// StandardLifecyclePolicy is the built-in policy with its starting
// settings, every one set to its default.
func StandardLifecyclePolicy() *LifecyclePolicy {
	return &LifecyclePolicy{
		Name:        LifecyclePolicyStandard,
		Description: "The registry's standard gTLD lifecycle: 5-day add, renew and transfer grace, 45-day auto-renew grace, 60-day transfer lock, 30-day redemption, 5-day pending delete, 7-day restore report.",
		Settings:    DefaultLifecycle().Settings(),
	}
}

// BuiltIn reports whether p is the built-in policy.
func (p *LifecyclePolicy) BuiltIn() bool { return p.Name == LifecyclePolicyStandard }

// Resolved is p with every setting resolved.
func (p *LifecyclePolicy) Resolved() ResolvedLifecycle {
	if p == nil {
		return DefaultLifecycle()
	}
	return p.Settings.Resolve()
}

// The resolved accessors the lifecycle code reads. Each works on a nil
// policy, answering the default, so an entity test needs no policy.

// RegistrationGraceDays is the add grace period in days.
func (p *LifecyclePolicy) RegistrationGraceDays() int { return p.Resolved().RegistrationGP }

// RenewalGraceDays is the renew grace period in days.
func (p *LifecyclePolicy) RenewalGraceDays() int { return p.Resolved().RenewalGP }

// AutoRenewalGraceDays is the auto-renew grace period in days.
func (p *LifecyclePolicy) AutoRenewalGraceDays() int { return p.Resolved().AutoRenewalGP }

// TransferGraceDays is the transfer grace period in days.
func (p *LifecyclePolicy) TransferGraceDays() int { return p.Resolved().TransferGP }

// TransferLockDays is the transfer lock period in days; 0 is no lock.
func (p *LifecyclePolicy) TransferLockDays() int { return p.Resolved().TransferLockPeriod }

// PendingTransferDays is the losing registrar's window in days.
func (p *LifecyclePolicy) PendingTransferDays() int { return p.Resolved().PendingTransferPeriod }

// RedemptionDays is the redemption period in days.
func (p *LifecyclePolicy) RedemptionDays() int { return p.Resolved().RedemptionGP }

// PendingDeleteDays is the pending delete period in days.
func (p *LifecyclePolicy) PendingDeleteDays() int { return p.Resolved().PendingDeleteGP }

// PendingRestoreDays is the restore report window in days.
func (p *LifecyclePolicy) PendingRestoreDays() int { return p.Resolved().PendingRestorePeriod }

// RequiresRestoreReport reports whether a restore waits for the report.
func (p *LifecyclePolicy) RequiresRestoreReport() bool { return p.Resolved().RestoreReportRequired }

// AutoRenewAllowed reports whether the expiry loop may auto-renew.
func (p *LifecyclePolicy) AutoRenewAllowed() bool { return p.Resolved().AllowAutoRenew }

// MaxHorizonYears is the furthest expiry, in years from now.
func (p *LifecyclePolicy) MaxHorizonYears() int { return p.Resolved().MaxHorizon }

// LifecyclePolicyUse is a phase that names a policy: what a change to the
// policy reaches.
type LifecyclePolicyUse struct {
	TLD   string `json:"tld"`
	Phase string `json:"phase"`
}
