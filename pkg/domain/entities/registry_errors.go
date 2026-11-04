package entities

import "errors"

// Refusals the registry's services raise and its adapters answer, declared
// here so that every layer matches one sentinel (INV-09): the services wrap
// them, the REST and EPP adapters map them to status codes, and no adapter
// has to import a service implementation to recognise a refusal (INV-18).
var (
	// ErrDomainExists: the name is registered.
	ErrDomainExists = errors.New("domain exists")
	// ErrDomainBlocked: the name is reserved or blocked (ADR 0011).
	ErrDomainBlocked = errors.New("domain is blocked")
	// ErrPhaseRequired: an availability check without the phase it is in.
	ErrPhaseRequired = errors.New("phase is required to check domain availability")
	// ErrAutoRenewNotEnabledRar: the registrar does not auto-renew.
	ErrAutoRenewNotEnabledRar = errors.New("auto renew is not enabled for this registrar")
	// ErrAutoRenewNotEnabledTLD: the TLD does not auto-renew.
	ErrAutoRenewNotEnabledTLD = errors.New("auto renew is not enabled for this TLD")
	// ErrRegistrarNotAccredited: the registrar is not accredited for the
	// TLD (ADR 0006).
	ErrRegistrarNotAccredited = errors.New("registrar is not accredited for this TLD")
	// ErrCouldNotDetermineAccreditation: the accreditation lookup failed.
	ErrCouldNotDetermineAccreditation = errors.New("could not determine accreditation")
	// ErrMissingFXRate: a quote in a currency with no rate from the phase's
	// base currency.
	ErrMissingFXRate = errors.New("missing FX rate")
	// ErrCannotSetDomainStatus and ErrCannotUnSetDomainStatus: the status
	// change is not one an operator may make.
	ErrCannotSetDomainStatus   = errors.New("cannot set domain status")
	ErrCannotUnSetDomainStatus = errors.New("cannot unset domain status")
	// ErrInvalidAccreditation: the registrar's status does not allow it to
	// be accredited.
	ErrInvalidAccreditation = errors.New("invalid accreditation")
	// ErrCannotDeleteTLDWithActivePhases: a TLD is deleted only once no
	// phase of it is active.
	ErrCannotDeleteTLDWithActivePhases = errors.New("cannot delete TLD with active phases")
	// ErrInvalidCreateTLDCommand: a TLD create that does not hold together.
	ErrInvalidCreateTLDCommand = errors.New("invalid create TLD command")
	// ErrReasonRequired: a launch application is made invalid or rejected
	// without a reason (ADR 0024 §3).
	ErrReasonRequired = errors.New("a reason is required")
	// ErrDecisionRequired: a validation outcome without the decision (valid
	// true or false).
	ErrDecisionRequired = errors.New("the decision is required: valid true or false")
)
