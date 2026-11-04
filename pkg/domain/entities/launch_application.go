package entities

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Launch applications (RFC 8334, ADR 0024 §3): in a launch phase whose
// registration model is application, a <domain:create> files an application
// for a name instead of registering it. Several registrars, and several
// applicants through one registrar, may apply for one name; the operator
// allocates one application per name through the admin API, which registers
// the domain in the application's phase and rejects the rest. Nothing is
// charged at filing: the quote computed then is kept with the application
// and billed when it is allocated.

// ApplicationStatus is an application's state, as RFC 8334 §2.4 names them
// (launch:statusValueType). validated and custom are not used: validation
// moves an application straight to pendingAllocation.
type ApplicationStatus string

const (
	// ApplicationStatusPendingValidation: filed in a phase whose policy
	// requires validation (PhasePolicy.RequiresValidation), not yet
	// validated by the operator.
	ApplicationStatusPendingValidation ApplicationStatus = "pendingValidation"
	// ApplicationStatusPendingAllocation: waiting for the operator to
	// allocate the name to one of its applications.
	ApplicationStatusPendingAllocation ApplicationStatus = "pendingAllocation"
	// ApplicationStatusAllocated: the name was registered to this
	// application; DomainRoID is the domain's.
	ApplicationStatusAllocated ApplicationStatus = "allocated"
	// ApplicationStatusRejected: the name went to another application, or
	// the operator rejected this one; Reason says which.
	ApplicationStatusRejected ApplicationStatus = "rejected"
	// ApplicationStatusInvalid: validation failed; Reason says why.
	ApplicationStatusInvalid ApplicationStatus = "invalid"
)

// IsFinal reports whether the status ends the application: allocated,
// rejected or invalid. A final application cannot be changed or withdrawn.
func (s ApplicationStatus) IsFinal() bool {
	return s == ApplicationStatusAllocated || s == ApplicationStatusRejected || s == ApplicationStatusInvalid
}

// IsValid reports whether s is one of the statuses used.
func (s ApplicationStatus) IsValid() bool {
	switch s {
	case ApplicationStatusPendingValidation, ApplicationStatusPendingAllocation,
		ApplicationStatusAllocated, ApplicationStatusRejected, ApplicationStatusInvalid:
		return true
	}
	return false
}

// Refusals of application commands. The EPP adapter maps them to result
// codes (docs/epp/POLICY_REFERENCE.md).
var (
	// ErrApplicationNotFound: no application has the ID given (2303).
	ErrApplicationNotFound = errors.New("application not found")
	// ErrApplicationExists: the registrar already has an open application
	// for the name in the phase (2302). It may file again once that one
	// is final or withdrawn.
	ErrApplicationExists = errors.New("the registrar already has an open application for this name in this phase")
	// ErrApplicationFinal: the application is allocated, rejected or
	// invalid, and can no longer be changed or withdrawn (2304).
	ErrApplicationFinal = errors.New("the application is final and cannot be changed")
	// ErrApplicationChanged: another command changed the application since
	// it was read; it is read and checked again.
	ErrApplicationChanged = errors.New("the application changed since it was read")
	// ErrApplicationIDRequired: a <launch:info>, <launch:update> or
	// <launch:delete> on a name names no application (2306). Registered
	// names take no launch extension on those commands.
	ErrApplicationIDRequired = errors.New("the launch extension names no application; give its applicationID")
	// ErrApplicationMismatch: the application named is not for the name,
	// or not in the phase, the command gives (2306).
	ErrApplicationMismatch = errors.New("the application named is not for this name in this phase")
	// ErrApplicationUpdateUnsupported: an update of an application adds or
	// removes statuses, or restores, neither of which an application has
	// (2306).
	ErrApplicationUpdateUnsupported = errors.New("an application takes no statuses or restore; change its contacts, nameservers, DS records or code")
	// ErrApplicationTransition: a status change the application's status
	// does not allow, such as allocating one that is not pendingAllocation.
	ErrApplicationTransition = errors.New("the application's status does not allow that")
	// ErrInvalidApplication: an application that does not hold together.
	ErrInvalidApplication = errors.New("invalid application")
)

// ApplicationQuote is the price quoted when an application was filed, as
// the registration will be billed at allocation: the amount in the smallest
// unit of the currency, and the price class (standard, or a premium
// class).
type ApplicationQuote struct {
	Currency string
	Amount   int64
	Class    string
}

// LaunchApplication is one registrar's application for a name in an
// application phase: what a create holds, the status it has reached, the
// quote kept for allocation, and once allocated the domain's ROID.
type LaunchApplication struct {
	// ID is the server-assigned applicationID: roidType-valid, minted as
	// ROIDs are (NewRoidType with RoidTypeApplication).
	ID        RoidType
	Name      DomainName
	TLDName   DomainName
	PhaseName ClIDType
	// PhaseMode is the phase's launch mode when the application was filed
	// (a phase that has started cannot change it): the value of the
	// <launch:phase> in every answer about the application.
	PhaseMode LaunchMode
	// ClID is the registrar that filed it, the only one that may see,
	// change or withdraw it (ADR 0006).
	ClID   ClIDType
	Status ApplicationStatus

	RegistrantID ClIDType
	AdminID      ClIDType
	TechID       ClIDType
	BillingID    ClIDType
	// HostNames are the nameservers, host objects of the registrar, in
	// lower case, each once.
	HostNames []string
	DSRecords []DSRecord
	Years     int
	// AuthInfo is the transfer code the domain gets when allocated, kept
	// hashed as a domain's is (RFC 9154).
	AuthInfo AuthInfoHash
	Quote    ApplicationQuote
	// Reason says why the application is rejected or invalid.
	Reason string
	// DomainRoID is the domain registered to the application, once
	// allocated.
	DomainRoID RoidType

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewLaunchApplication files an application for name by clid in phase, for
// years, with authInfo as the eventual domain's transfer code (hashed; empty
// for none). phase must be an application phase that admits applications
// (LaunchSettings.AdmitsApplication). It starts pendingValidation when the
// phase's policy requires validation, else pendingAllocation. Contacts,
// nameservers and DS records are set afterwards.
func NewLaunchApplication(id RoidType, name, clid, authInfo string, phase *Phase, years int, now time.Time) (*LaunchApplication, error) {
	if phase == nil {
		return nil, ErrPhaseNotProvided
	}
	if err := id.Validate(); err != nil {
		return nil, fmt.Errorf("%w: application ID: %w", ErrInvalidApplication, err)
	}
	dn, err := NewDomainName(name)
	if err != nil {
		return nil, err
	}
	if dn.ParentDomain() == "" {
		return nil, ErrTLDAsDomain
	}
	c, err := NewClIDType(clid)
	if err != nil {
		return nil, err
	}
	if err := phase.Launch.AdmitsApplication(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, phase.Name)
	}
	if years <= 0 {
		return nil, ErrZeroRenewalPeriod
	}
	if err := ValidateDomainAuthInfo(authInfo); err != nil {
		return nil, err
	}
	h, err := HashAuthInfo(authInfo)
	if err != nil {
		return nil, err
	}
	status := ApplicationStatusPendingAllocation
	if phase.Policy.RequiresValidation != nil && *phase.Policy.RequiresValidation {
		status = ApplicationStatusPendingValidation
	}
	now = now.UTC()
	return &LaunchApplication{
		ID: id, Name: *dn, TLDName: DomainName(dn.ParentDomain()), PhaseName: phase.Name, PhaseMode: phase.Launch.Mode, ClID: c,
		Status: status, Years: years, AuthInfo: h, CreatedAt: now, UpdatedAt: now,
	}, nil
}

// SetHostNames replaces the nameservers: each name lower-cased, each once
// (ErrDuplicateHost), at most MaxHostsPerDomain
// (ErrMaxHostsPerDomainExceeded). Whether each is one of the registrar's
// host objects is the caller's to check.
func (a *LaunchApplication) SetHostNames(names []string) error {
	out := make([]string, 0, len(names))
	for _, n := range names {
		n = strings.ToLower(n)
		if slices.Contains(out, n) {
			return fmt.Errorf("%w: %s is named more than once", ErrDuplicateHost, n)
		}
		out = append(out, n)
	}
	if len(out) > MaxHostsPerDomain {
		return ErrMaxHostsPerDomainExceeded
	}
	if len(out) == 0 {
		out = nil
	}
	a.HostNames = out
	return nil
}

// SetDSRecords replaces the DS records, each valid and given once, as
// Domain.SetDSRecords does.
func (a *LaunchApplication) SetDSRecords(records []DSRecord) error {
	d := Domain{}
	if err := d.SetDSRecords(records); err != nil {
		return err
	}
	a.DSRecords = d.DSRecords
	return nil
}

// CanChange reports whether the registrar may still change or withdraw the
// application: ErrApplicationFinal once it is final.
func (a *LaunchApplication) CanChange() error {
	if a.Status.IsFinal() {
		return fmt.Errorf("%w: %s is %s", ErrApplicationFinal, a.ID, a.Status)
	}
	return nil
}

// Validated records that the operator validated the application: from
// pendingValidation to pendingAllocation.
func (a *LaunchApplication) Validated(now time.Time) error {
	if a.Status != ApplicationStatusPendingValidation {
		return fmt.Errorf("%w: %s is %s, not pendingValidation", ErrApplicationTransition, a.ID, a.Status)
	}
	a.Status = ApplicationStatusPendingAllocation
	a.UpdatedAt = now.UTC()
	return nil
}

// Invalidate records that validation failed, with the reason: from
// pendingValidation to invalid.
func (a *LaunchApplication) Invalidate(reason string, now time.Time) error {
	if a.Status != ApplicationStatusPendingValidation {
		return fmt.Errorf("%w: %s is %s, not pendingValidation", ErrApplicationTransition, a.ID, a.Status)
	}
	a.Status, a.Reason = ApplicationStatusInvalid, reason
	a.UpdatedAt = now.UTC()
	return nil
}

// Allocate records that the name was registered to the application as the
// domain with ROID domainRoID: from pendingAllocation to allocated.
func (a *LaunchApplication) Allocate(domainRoID RoidType, now time.Time) error {
	if a.Status != ApplicationStatusPendingAllocation {
		return fmt.Errorf("%w: %s is %s, not pendingAllocation", ErrApplicationTransition, a.ID, a.Status)
	}
	if err := domainRoID.Validate(); err != nil {
		return fmt.Errorf("%w: domain ROID: %w", ErrInvalidApplication, err)
	}
	a.Status, a.DomainRoID = ApplicationStatusAllocated, domainRoID
	a.UpdatedAt = now.UTC()
	return nil
}

// Reject ends a non-final application with the reason: the name went to
// another application, or the operator rejected this one.
func (a *LaunchApplication) Reject(reason string, now time.Time) error {
	if a.Status.IsFinal() {
		return fmt.Errorf("%w: %s is %s", ErrApplicationTransition, a.ID, a.Status)
	}
	a.Status, a.Reason = ApplicationStatusRejected, reason
	a.UpdatedAt = now.UTC()
	return nil
}

// Validate checks the application holds together.
func (a *LaunchApplication) Validate() error {
	if err := a.ID.Validate(); err != nil {
		return fmt.Errorf("%w: application ID: %w", ErrInvalidApplication, err)
	}
	if err := a.Name.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidApplication, err)
	}
	if a.Name.ParentDomain() != a.TLDName.String() {
		return fmt.Errorf("%w: %s is not under %s", ErrInvalidApplication, a.Name, a.TLDName)
	}
	if err := a.PhaseName.Validate(); err != nil {
		return fmt.Errorf("%w: phase: %w", ErrInvalidApplication, err)
	}
	if !a.PhaseMode.IsValid() {
		return fmt.Errorf("%w: phase mode %q", ErrInvalidApplication, a.PhaseMode)
	}
	if err := a.ClID.Validate(); err != nil {
		return fmt.Errorf("%w: registrar: %w", ErrInvalidApplication, err)
	}
	if !a.Status.IsValid() {
		return fmt.Errorf("%w: status %q", ErrInvalidApplication, a.Status)
	}
	if a.Years <= 0 {
		return fmt.Errorf("%w: %w", ErrInvalidApplication, ErrZeroRenewalPeriod)
	}
	if err := a.AuthInfo.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidApplication, err)
	}
	if len(a.HostNames) > MaxHostsPerDomain {
		return fmt.Errorf("%w: %w", ErrInvalidApplication, ErrMaxHostsPerDomainExceeded)
	}
	if a.Status == ApplicationStatusAllocated && a.DomainRoID == "" {
		return fmt.Errorf("%w: allocated without a domain", ErrInvalidApplication)
	}
	return nil
}

// DeepCopy returns a copy that shares nothing with a.
func (a *LaunchApplication) DeepCopy() *LaunchApplication {
	c := *a
	c.HostNames = slices.Clone(a.HostNames)
	c.DSRecords = slices.Clone(a.DSRecords)
	return &c
}
