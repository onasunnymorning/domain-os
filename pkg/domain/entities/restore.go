package entities

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Restoring a deleted domain: RFC 3915 §3–4, the registry grace period
// (RGP) restore, as ICANN's RGP policy and .com (Verisign) apply it. The
// user guide is the admin UI's Domain Restore page
// (frontend/lib/constants/domainRestoreDoc.ts); the EPP result codes are in
// docs/epp/POLICY_REFERENCE.md.
//
// A deleted or expired domain is pendingDelete (Domain.MarkForDeletion,
// Domain.Expire), first in its redemption period (RedemptionPeriodEnd, 30
// days by default), in which it can be restored, then until its purge date
// (5 days more). It is not in the zone meanwhile. The restore:
//
//  1. Request. The sponsor sends <domain:update> with
//     <rgp:restore op="request"/> and no other change, while the domain is
//     in its redemption period. Domain.RequestRestore makes it
//     pendingRestore (an RGP status; its EPP status stays pendingDelete,
//     RFC 3915 §4) until PendingRestoreEnd: the phase's pending restore
//     period, 7 days by default.
//  2. Report. The sponsor sends <rgp:restore op="report"> with the restore
//     report (RestoreReport): the registration data before the delete and
//     now, when the domain was deleted and the restore requested, why, and
//     the two statements ICANN requires. Domain.CompleteRestore restores
//     the domain: pendingDelete and pendingRestore are cleared, the domain
//     leaves its end of life, and it is renewed for a year, and for further
//     years while its expiry would still be past. The registrar is charged
//     the restore fee and the renewal, two separate charges.
//  3. No report by PendingRestoreEnd: the restore lapses. The RestoreWorkflow
//     loop returns the domain to where it was (Domain.RevertRestore):
//     pendingDelete, in its redemption period if that has not ended, with
//     its original purge date. Nothing is charged. A lapsed request cannot
//     be reported; the registrar may request a restore again while the
//     redemption period lasts.
//
// A phase may waive the report (PhasePolicy.RestoreReportRequired false,
// as registries that do not require one do): the request then restores the
// domain at once.

// Restore refusals. Each says what to do instead.
var (
	// ErrRestoreNotPending: a report for a domain whose restore was not
	// requested, or whose request lapsed (RFC 3915: 2304).
	ErrRestoreNotPending = errors.New("no restore is pending for the domain")
	// ErrRestoreWithChanges: a restore in a <domain:update> that also
	// changes the domain (2306). A restore changes nothing else.
	ErrRestoreWithChanges = errors.New("a restore takes no other change; send the changes in a separate update")
	// ErrRestoreReportIncomplete: a report without one of its required
	// parts (2003).
	ErrRestoreReportIncomplete = errors.New("the restore report is incomplete")
	// ErrRestoreReportTimes: a report whose delete and restore times do not
	// fit (2004).
	ErrRestoreReportTimes = errors.New("the restore report's times are not possible")
)

// RestoreReportStatements is how many statements a restore report must
// make (RFC 3915 §4.2.5): that the registrar has not restored the domain
// to assume the rights to use or sell it, and that the report is factual.
const RestoreReportStatements = 2

// RestoreReport is a registrar's restore report (RFC 3915 §4.2.5,
// <rgp:report>). The free-text parts keep the registrar's content as sent,
// XML included.
type RestoreReport struct {
	// PreData is the registration data before the domain was deleted.
	PreData string `json:"preData"`
	// PostData is the registration data as restored.
	PostData string `json:"postData"`
	// DelTime is when the domain was deleted.
	DelTime time.Time `json:"delTime"`
	// ResTime is when the restore was requested.
	ResTime time.Time `json:"resTime"`
	// ResReason is why the domain is restored.
	ResReason string `json:"resReason"`
	// Statements are the registrar's two statements.
	Statements []string `json:"statements"`
	// Other is anything else the registrar adds; optional.
	Other string `json:"other,omitempty"`
}

// Validate checks the report is whole and its times possible at now: every
// required part has content, there are two statements, and the domain was
// deleted before the restore was requested, which was not after now.
func (r *RestoreReport) Validate(now time.Time) error {
	var missing []string
	for _, p := range []struct{ name, v string }{
		{"preData", r.PreData}, {"postData", r.PostData}, {"resReason", r.ResReason},
	} {
		if strings.TrimSpace(p.v) == "" {
			missing = append(missing, p.name)
		}
	}
	given := 0
	for _, s := range r.Statements {
		if strings.TrimSpace(s) != "" {
			given++
		}
	}
	if given < RestoreReportStatements {
		missing = append(missing, fmt.Sprintf("%d statements", RestoreReportStatements))
	}
	if len(missing) > 0 {
		return fmt.Errorf("%w: give %s", ErrRestoreReportIncomplete, strings.Join(missing, ", "))
	}
	switch {
	case !r.DelTime.Before(r.ResTime):
		return fmt.Errorf("%w: delTime must be before resTime", ErrRestoreReportTimes)
	case r.ResTime.After(now):
		return fmt.Errorf("%w: resTime is in the future", ErrRestoreReportTimes)
	}
	return nil
}

// RestorePending reports whether a restore was requested and its report is
// still awaited at now.
func (d *Domain) RestorePending(now time.Time) bool {
	return d.Status.PendingRestore && now.Before(d.RGPStatus.PendingRestoreEnd)
}

// RestoreLapsed reports whether a restore was requested and its report not
// sent by its deadline: the restore is reverted (RevertRestore).
func (d *Domain) RestoreLapsed(now time.Time) bool {
	return d.Status.PendingRestore && !now.Before(d.RGPStatus.PendingRestoreEnd)
}

// RequestRestore requests the restore of d at now, in phase (step 1). d
// must be pendingDelete, in its redemption period, with no restore pending
// (ErrDomainRestoreNotAllowed otherwise: 2304). It becomes pendingRestore
// until the phase's pending restore period ends.
func (d *Domain) RequestRestore(phase *Phase, now time.Time) error {
	switch {
	case phase == nil:
		return errors.Join(ErrDomainRestoreNotAllowed, ErrPhaseNotProvided)
	case d.Status.PendingRestore:
		return fmt.Errorf("%w: a restore was already requested; send its report by %s",
			ErrDomainRestoreNotAllowed, d.RGPStatus.PendingRestoreEnd.UTC().Format(time.RFC3339))
	case !d.Status.PendingDelete:
		return fmt.Errorf("%w: it is not deleted (pendingDelete)", ErrDomainRestoreNotAllowed)
	case !now.Before(d.RGPStatus.RedemptionPeriodEnd):
		return fmt.Errorf("%w: its redemption period ended %s",
			ErrDomainRestoreNotAllowed, d.RGPStatus.RedemptionPeriodEnd.UTC().Format(time.RFC3339))
	}
	d.Status.PendingRestore = true
	d.RGPStatus.PendingRestoreEnd = now.AddDate(0, 0, phase.Policy.PendingRestoreDays())
	return d.Status.Validate()
}

// CompleteRestore restores d at now, in phase (step 2), and returns the
// years the domain was renewed for. report is the registrar's restore
// report, checked by RestoreReport.Validate; nil when the phase does not
// require one. d must have a restore pending (ErrRestoreNotPending
// otherwise).
//
// pendingDelete and pendingRestore are cleared, and the domain's end of
// life with them (redemption period, purge date). Its expiry is extended by
// a year, and by further years while it would still be past, so it is
// always in the future: an expired domain may have been in redemption past
// a whole term. No renew grace period starts: a restored domain deleted
// again gets no refund.
func (d *Domain) CompleteRestore(report *RestoreReport, phase *Phase, now time.Time) (int, error) {
	switch {
	case phase == nil:
		return 0, errors.Join(ErrInvalidRenewal, ErrPhaseNotProvided)
	case !d.Status.PendingRestore:
		return 0, fmt.Errorf("%w: request a restore first", ErrRestoreNotPending)
	case d.RestoreLapsed(now):
		return 0, fmt.Errorf("%w: the report was due by %s; request a restore again",
			ErrRestoreNotPending, d.RGPStatus.PendingRestoreEnd.UTC().Format(time.RFC3339))
	}
	if report != nil {
		if err := report.Validate(now); err != nil {
			return 0, err
		}
	}
	d.Status.PendingRestore = false
	d.Status.PendingDelete = false
	d.RGPStatus.PendingRestoreEnd = time.Time{}
	d.RGPStatus.RedemptionPeriodEnd = time.Time{}
	d.RGPStatus.PurgeDate = time.Time{}
	years := 1
	d.ExpiryDate = d.ExpiryDate.AddDate(1, 0, 0)
	for !d.ExpiryDate.After(now) {
		d.ExpiryDate = d.ExpiryDate.AddDate(1, 0, 0)
		years++
	}
	d.RenewedYears += years
	d.SetUnsetInactiveStatus()
	d.SetOKStatusIfNeeded()
	return years, d.Status.Validate()
}

// RevertRestore ends a restore whose report was not sent in time (step 3):
// pendingRestore is cleared, and d is pendingDelete as before the request,
// with its redemption period end and purge date unchanged. d's restore must
// have lapsed at now (ErrRestoreNotPending otherwise).
func (d *Domain) RevertRestore(now time.Time) error {
	if !d.RestoreLapsed(now) {
		return fmt.Errorf("%w: no lapsed restore to revert", ErrRestoreNotPending)
	}
	d.Status.PendingRestore = false
	d.RGPStatus.PendingRestoreEnd = time.Time{}
	return d.Status.Validate()
}
