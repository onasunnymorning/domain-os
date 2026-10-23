package entities

import (
	"errors"
	"fmt"
	"time"
)

// Domain transfers: moving a domain from one registrar (the losing
// registrar, its sponsor) to another (the gaining registrar), as RFC 5731
// §3.2.4 defines the EPP <transfer> command, RFC 9154 the transfer code,
// and the ICANN Transfer Policy the process. The user guide is the admin
// UI's Domain Transfers page (frontend/lib/constants/domainTransfersDoc.ts);
// the EPP result codes are in docs/epp/POLICY_REFERENCE.md.
//
// The process:
//
//  1. Request. The registrant gets a transfer code from the losing
//     registrar and gives it to the gaining registrar, which sends
//     <transfer op="request"> with the code and, optionally, a period.
//     Domain.RequestTransfer checks, in order: the requester is not the
//     sponsor; the code matches (RFC 9154 §4.4); the domain has no pending
//     action, no transfer prohibition, and is out of its transfer lock
//     period; and the period keeps the expiry within the phase's maximum
//     horizon. The domain becomes pendingTransfer, and a DomainTransfer is
//     recorded, pending until its ActionDate (the phase's pending transfer
//     period, 5 days by default).
//  2. Answer. The losing registrar approves or rejects it, or the gaining
//     registrar cancels it. Unanswered by its ActionDate, the registry
//     approves it (serverApproved; Phase 4i).
//  3. Approval. Domain.CompleteTransfer moves the domain to the gaining
//     registrar: its expiry is extended by the period (one year if none was
//     given), its transfer code is cleared so it cannot be used again, its
//     grace periods from the losing registrar's transactions end, and it
//     enters the transfer grace period (RFC 3915 transferPeriod) and a new
//     transfer lock period. Grandfathered pricing granted until a transfer
//     is dropped. Host objects are re-linked in the same transaction
//     (TransferResolution). Contacts stay as they are, still the losing
//     registrar's: the gaining registrar is told to replace them (ADR 0006,
//     amended 2026-10-22).
//  4. Rejection or cancellation clears pendingTransfer and changes nothing
//     else.
//
// Every step queues poll messages (NewTransferPollMessages) for the
// registrars that did not take it, and the latest transfer of a domain can
// be queried (<transfer op="query">) until another replaces it.

// TransferStatus is a transfer's state, as RFC 5730 §2.9.3.4 names them
// (eppcom:trStatusType).
type TransferStatus string

const (
	// TransferStatusPending: requested, not yet answered.
	TransferStatusPending TransferStatus = "pending"
	// TransferStatusClientApproved: approved by the losing registrar.
	TransferStatusClientApproved TransferStatus = "clientApproved"
	// TransferStatusClientRejected: rejected by the losing registrar.
	TransferStatusClientRejected TransferStatus = "clientRejected"
	// TransferStatusClientCancelled: cancelled by the gaining registrar.
	TransferStatusClientCancelled TransferStatus = "clientCancelled"
	// TransferStatusServerApproved: approved by the registry, because the
	// losing registrar did not answer in time.
	TransferStatusServerApproved TransferStatus = "serverApproved"
	// TransferStatusServerCancelled: cancelled by the registry.
	TransferStatusServerCancelled TransferStatus = "serverCancelled"
)

// IsApproved reports whether the status completes the transfer.
func (s TransferStatus) IsApproved() bool {
	return s == TransferStatusClientApproved || s == TransferStatusServerApproved
}

// Refusals of transfer commands. The EPP adapter maps them to result codes.
var (
	// ErrTransferToSponsor: the requester already sponsors the domain
	// (RFC 5731 §3.2.4: 2106).
	ErrTransferToSponsor = errors.New("the domain is already sponsored by the requesting registrar")
	// ErrTransferPending: a transfer is already pending (2300).
	ErrTransferPending = errors.New("the domain is already pending transfer")
	// ErrTransferNotPending: an answer to, or a cancellation of, a transfer
	// that is not pending; or a query of a domain never transferred (2301).
	ErrTransferNotPending = errors.New("the domain has no pending transfer")
	// ErrTransferNotEligible: the domain is in its transfer lock period
	// (2106).
	ErrTransferNotEligible = errors.New("the domain is not eligible for transfer")
	// ErrTransferProhibited: the domain's statuses prohibit a transfer: a
	// transfer prohibition, or another pending action (2304).
	ErrTransferProhibited = errors.New("the domain's status prohibits a transfer")
	// ErrTransferExceedsMaxHorizon: the period would take the expiry beyond
	// the phase's maximum horizon (2306).
	ErrTransferExceedsMaxHorizon = errors.New("the transfer period would take the expiry beyond the maximum horizon")
	// ErrInvalidTransferStatus: a transfer resolved to a status that does not
	// end it.
	ErrInvalidTransferStatus = errors.New("invalid transfer status")
)

// DomainTransfer is one transfer request of a domain and what became of
// it. Its fields are those of <domain:trnData> (RFC 5731 §3.2.4).
type DomainTransfer struct {
	// ID is the registry's identifier of the request.
	ID int64
	// DomainRoID and DomainName identify the domain.
	DomainRoID RoidType
	DomainName DomainName
	// Status is pending until the transfer is answered.
	Status TransferStatus
	// RequestingClID is the gaining registrar (reID), and RequestedAt
	// when it asked (reDate).
	RequestingClID ClIDType
	RequestedAt    time.Time
	// LosingClID is the sponsor when the transfer was requested.
	LosingClID ClIDType
	// ActingClID is acID: while pending, the registrar that should act,
	// the losing registrar; afterwards, the registrar that acted, or the
	// losing registrar when the registry did.
	ActingClID ClIDType
	// ActionDate is acDate: while pending, when the registry will approve
	// the transfer if the losing registrar has not answered; afterwards,
	// when it was answered.
	ActionDate time.Time
	// Years is the period requested, added to the expiry on approval.
	Years int
	// ExpiryDate is exDate: the expiry the transfer gives the domain, while
	// pending and once approved; zero when it was rejected or cancelled,
	// which changes no expiry.
	ExpiryDate time.Time
}

// RequestTransfer starts a transfer of d to gaining, who gave authInfo as
// the domain's transfer code, for years (one if 0) under phase's policy.
// It checks the request as the type's comment lists, then makes d
// pendingTransfer and returns the transfer, pending. On a refusal d is
// unchanged.
func (d *Domain) RequestTransfer(gaining ClIDType, authInfo string, years int, phase *Phase, now time.Time) (*DomainTransfer, error) {
	if phase == nil {
		return nil, ErrPhaseNotProvided
	}
	if years == 0 {
		years = 1
	}
	switch {
	case gaining == d.ClID:
		return nil, ErrTransferToSponsor
	case !d.AuthInfo.Matches(authInfo):
		// Before every status check: a registrar without the code learns
		// nothing about the domain.
		return nil, ErrEPPAuthInfoMismatch
	case d.Status.PendingTransfer:
		return nil, ErrTransferPending
	case d.Status.HasPendings():
		return nil, fmt.Errorf("%w: %v", ErrTransferProhibited, d.Status.StringSlice())
	case d.Status.ClientTransferProhibited:
		return nil, fmt.Errorf("%w: %s", ErrTransferProhibited, DomainStatusClientTransferProhibited)
	case d.Status.ServerTransferProhibited:
		return nil, fmt.Errorf("%w: %s", ErrTransferProhibited, DomainStatusServerTransferProhibited)
	case now.Before(d.RGPStatus.TransferLockPeriodEnd):
		return nil, fmt.Errorf("%w: it is in its transfer lock period until %s", ErrTransferNotEligible, d.RGPStatus.TransferLockPeriodEnd.UTC().Format(time.RFC3339))
	}
	expiry := d.ExpiryDate.AddDate(years, 0, 0)
	if expiry.After(now.AddDate(phase.Policy.MaxHorizon, 0, 0)) {
		return nil, fmt.Errorf("%w of %d years", ErrTransferExceedsMaxHorizon, phase.Policy.MaxHorizon)
	}

	// A transfer is not an update: only the transfer prohibitions stop it,
	// so pendingTransfer is set here rather than through SetStatus, which
	// refuses every change under an update prohibition.
	d.Status.PendingTransfer = true
	d.UnSetOKStatusIfNeeded()
	return &DomainTransfer{
		DomainRoID:     d.RoID,
		DomainName:     d.Name,
		Status:         TransferStatusPending,
		RequestingClID: gaining,
		RequestedAt:    now,
		LosingClID:     d.ClID,
		ActingClID:     d.ClID,
		ActionDate:     now.AddDate(0, 0, phase.Policy.PendingTransferDays()),
		Years:          years,
		ExpiryDate:     expiry,
	}, nil
}

// Resolve ends a pending transfer with status, as actor did at now; actor
// is empty when the registry acted. It is ErrTransferNotPending if the
// transfer has already ended.
func (t *DomainTransfer) Resolve(status TransferStatus, actor ClIDType, now time.Time) error {
	if t.Status != TransferStatusPending {
		return ErrTransferNotPending
	}
	switch status {
	case TransferStatusClientApproved, TransferStatusClientRejected, TransferStatusClientCancelled:
		if actor == "" {
			return fmt.Errorf("%w: a client status needs the client", ErrInvalidTransferStatus)
		}
	case TransferStatusServerApproved, TransferStatusServerCancelled:
		actor = t.LosingClID
	default:
		return fmt.Errorf("%w: %s", ErrInvalidTransferStatus, status)
	}
	t.Status = status
	t.ActingClID = actor
	t.ActionDate = now
	if !status.IsApproved() {
		t.ExpiryDate = time.Time{}
	}
	return nil
}

// CompleteTransfer applies an approved transfer t to d at now, under
// phase's policy (step 3 of the process). t.ExpiryDate is set to the
// expiry d gets. Host objects are the repository's to re-link.
func (d *Domain) CompleteTransfer(t *DomainTransfer, phase *Phase, now time.Time) error {
	if phase == nil {
		return ErrPhaseNotProvided
	}
	if !t.Status.IsApproved() {
		return fmt.Errorf("%w: %s does not complete a transfer", ErrInvalidTransferStatus, t.Status)
	}
	d.ClID = t.RequestingClID
	d.ExpiryDate = d.ExpiryDate.AddDate(t.Years, 0, 0)
	d.RenewedYears += t.Years
	t.ExpiryDate = d.ExpiryDate
	d.TransferredAt = now
	// RFC 9154 §4.3: a code is used once.
	d.AuthInfo = ""
	// The grace periods of the losing registrar's transactions end: a
	// delete by the gaining registrar earns it no refund of them.
	d.RGPStatus.AddPeriodEnd = time.Time{}
	d.RGPStatus.RenewPeriodEnd = time.Time{}
	d.RGPStatus.AutoRenewPeriodEnd = time.Time{}
	d.RGPStatus.TransferPeriodEnd = now.AddDate(0, 0, phase.Policy.TransferGraceDays())
	d.RGPStatus.TransferLockPeriodEnd = now.AddDate(0, 0, phase.Policy.TransferLockDays())
	// Grandfathered pricing granted until a transfer ends here.
	if d.GrandFathering.GFExpiryCondition == GFConditionTransfer {
		d.GrandFathering = DomainGrandFathering{}
	}
	d.Status.PendingTransfer = false
	d.SetOKStatusIfNeeded()
	return nil
}

// EndTransfer clears pendingTransfer after a transfer was rejected or
// cancelled (step 4): nothing else about d changes.
func (d *Domain) EndTransfer() {
	d.Status.PendingTransfer = false
	d.SetOKStatusIfNeeded()
}

// TransferResolution is the end of a pending transfer, for
// repositories.DomainTransferRepository to write in one transaction:
//
//   - Domain, as the transfer leaves it, read with version
//     Domain.UpdatedAt while sponsored by Transfer.LosingClID; written only
//     if the stored domain is still so (else ErrDomainChanged).
//   - Transfer, resolved; written only if the stored transfer is still
//     pending (else ErrDomainChanged).
//   - Messages, queued.
//
// For an approval the host objects go with the domain too (RFC 5732 §1.1:
// a host's sponsor is its superordinate domain's):
//
//   - the losing registrar's hosts under the domain (named as the domain, or
//     below it) are moved to the gaining registrar, unless it already has a
//     host of that name;
//   - each of the domain's nameservers named in GainingHosts is re-linked
//     to the gaining registrar's host of that name, which is created from
//     the GainingHosts entry, as given, if the gaining registrar has none;
//   - the domain's other nameservers keep their links: hosts under other
//     domains of this registry, which belong to those domains' sponsors.
//
// GainingHosts is planned from Domain.Hosts as read: if the domain's
// nameservers are no longer those (a host renamed meanwhile, which does not
// change the domain's version), the write is ErrDomainChanged, to be planned
// again. The losing registrar's hosts left behind are left in place: orphan
// cleanup is a later phase.
type TransferResolution struct {
	Domain   *Domain
	Transfer *DomainTransfer
	Messages []*PollMessage
	// GainingHosts are the hosts the gaining registrar's copy of each
	// re-linked nameserver is created from, with ROIDs minted, when it has
	// none: the external hosts, and the hosts under the domain.
	GainingHosts []*Host
}
