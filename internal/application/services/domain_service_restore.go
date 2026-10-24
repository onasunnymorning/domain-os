package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// Restoring a deleted domain (RFC 3915). The process and its rules are
// described with entities.Domain.RequestRestore; this file prices and
// records each step, and writes it over the domain as read.

// Domain restore events. A completed restore is two charges, each its own
// event and line item: domain.restored carries the restore fee (and the
// restore report, as its command), domain.renewed the renewal the restore
// includes, for the years it added (ICANN's RGP terms: the restore fee is
// in addition to the renewal). The request and a reversion record the
// steps, with no years and no quote. When the fees are taken, at the
// request or the report, and whether a lapsed request is refunded, is the
// billing system's to decide; nothing here charges.
const (
	EventDomainRestoreRequested = "domain.restore_requested"
	EventDomainRestored         = "domain.restored"
	EventDomainRestoreRenewed   = "domain.renewed"
	EventDomainRestoreReverted  = "domain.restore_reverted"
)

// restoreNow is the time a restore step is taken, as the database keeps
// it.
func restoreNow() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

// RestoreDomain requests the restore of the domain named domainName for its
// sponsor (the admin API): RequestRestoreAsRead.
func (s *DomainService) RestoreDomain(ctx context.Context, domainName string) (*entities.Domain, error) {
	return s.retryRestore(ctx, domainName, func(dom *entities.Domain) (*entities.Domain, error) {
		return s.RequestRestoreAsRead(ctx, dom, dom.ClID)
	})
}

// ReportRestore sends the restore report of the domain named domainName
// for its sponsor (the admin API): CompleteRestoreAsRead.
func (s *DomainService) ReportRestore(ctx context.Context, domainName string, report entities.RestoreReport) (*entities.Domain, error) {
	return s.retryRestore(ctx, domainName, func(dom *entities.Domain) (*entities.Domain, error) {
		return s.CompleteRestoreAsRead(ctx, dom, report, dom.ClID)
	})
}

// retryRestore reads the domain, with its nameservers (a completed restore
// sets inactive from them), and takes step on it, again if another command
// changed it meanwhile, up to updateAttempts times.
func (s *DomainService) retryRestore(ctx context.Context, domainName string, step func(*entities.Domain) (*entities.Domain, error)) (*entities.Domain, error) {
	for attempt := 1; ; attempt++ {
		dom, err := s.GetDomainByName(ctx, domainName, true)
		if err != nil {
			return nil, err
		}
		out, err := step(dom)
		if !errors.Is(err, entities.ErrDomainChanged) || attempt == updateAttempts {
			return out, err
		}
	}
}

// RequestRestoreAsRead requests the restore of dom, a domain the caller has
// read, for actor, its sponsor (entities.Domain.RequestRestore, in the
// TLD's current GA phase). The restore must be quotable: its fee and the
// renewal. It is recorded as a domain.restore_requested event. A phase that
// requires no restore report restores the domain at once, as
// CompleteRestoreAsRead does. The domain is written only if it is still as
// read (else entities.ErrDomainChanged).
func (s *DomainService) RequestRestoreAsRead(ctx context.Context, dom *entities.Domain, actor entities.ClIDType) (_ *entities.Domain, err error) {
	ctx, done := s.begin(ctx)
	defer done(&err)
	phase, err := s.currentGAPhase(ctx, dom)
	if err != nil {
		return nil, err
	}
	before := dom.DeepCopy()
	now := restoreNow()
	if err := dom.RequestRestore(phase, now); err != nil {
		return nil, err
	}
	if !phase.Policy.RequiresRestoreReport() {
		return s.completeRestore(ctx, before, dom, nil, phase, actor, now, true)
	}
	// A restore the registry cannot price is refused now, not at the
	// report: the renewal priced for the years the restore would add today.
	years := yearsToRestore(dom, now)
	if _, _, err := s.restoreQuotes(ctx, dom, years, phase); err != nil {
		return nil, err
	}
	dom.UpRr = actor
	stored, err := s.domainRepository.UpdateDomainColumnsIfUnchanged(ctx, dom)
	if err != nil {
		return nil, err
	}
	s.publishRestoreRequested(ctx, before, stored, actor)
	return stored, nil
}

// CompleteRestoreAsRead restores dom, a domain the caller has read whose
// restore is pending, with the sponsor's report
// (entities.Domain.CompleteRestore, in the TLD's current GA phase), and
// records the two charges: domain.restored with the restore fee and the
// report, and domain.renewed with the renewal for the years added. Written
// only if dom is still as read (else entities.ErrDomainChanged).
func (s *DomainService) CompleteRestoreAsRead(ctx context.Context, dom *entities.Domain, report entities.RestoreReport, actor entities.ClIDType) (*entities.Domain, error) {
	phase, err := s.currentGAPhase(ctx, dom)
	if err != nil {
		return nil, err
	}
	return s.completeRestore(ctx, dom.DeepCopy(), dom, &report, phase, actor, restoreNow(), false)
}

// completeRestore completes dom's restore at now and writes it; before is
// dom as read. requested says the request is part of the same command (a
// phase that requires no report), so it is recorded too.
func (s *DomainService) completeRestore(ctx context.Context, before, dom *entities.Domain, report *entities.RestoreReport,
	phase *entities.Phase, actor entities.ClIDType, now time.Time, requested bool) (_ *entities.Domain, err error) {
	ctx, done := s.begin(ctx)
	defer done(&err)
	years, err := dom.CompleteRestore(report, phase, now)
	if err != nil {
		return nil, err
	}
	restoreQ, renewQ, err := s.restoreQuotes(ctx, dom, years, phase)
	if err != nil {
		return nil, err
	}
	dom.UpRr = actor
	stored, err := s.domainRepository.UpdateDomainColumnsIfUnchanged(ctx, dom)
	if err != nil {
		return nil, err
	}
	if requested {
		s.publishRestoreRequested(ctx, before, stored, actor)
	}
	restored, err := entities.NewDomainLifeCycleEvent(actor.String(), "", stored.Name.ParentDomain(), stored.Name.String(), 0, entities.TransactionTypeRestore)
	if err != nil {
		return nil, err
	}
	restored.DomainRoID, restored.Quote = stored.RoID.String(), *restoreQ
	var command interface{}
	if report != nil {
		command = report
	}
	s.publishDomainEvent(ctx, EventDomainRestored, fmt.Sprintf("Domain %s restored by %s", stored.Name, actor), restored, command, stored, before)
	renewed, err := entities.NewDomainLifeCycleEvent(actor.String(), "", stored.Name.ParentDomain(), stored.Name.String(), years, entities.TransactionTypeRenewal)
	if err != nil {
		return nil, err
	}
	renewed.DomainRoID, renewed.Quote = stored.RoID.String(), *renewQ
	s.publishDomainEvent(ctx, EventDomainRestoreRenewed,
		fmt.Sprintf("Domain %s renewed for %d year(s) by its restore", stored.Name, years), renewed, nil, stored, before)
	return stored, nil
}

// publishRestoreRequested records a restore request.
func (s *DomainService) publishRestoreRequested(ctx context.Context, before, stored *entities.Domain, actor entities.ClIDType) {
	event, err := entities.NewDomainLifeCycleEvent(actor.String(), "", stored.Name.ParentDomain(), stored.Name.String(), 0, entities.TransactionTypeRestore)
	if err != nil {
		return // only an empty sponsor or name, which a stored domain has not
	}
	event.DomainRoID = stored.RoID.String()
	s.publishDomainEvent(ctx, EventDomainRestoreRequested, fmt.Sprintf("Restore of %s requested by %s", stored.Name, actor), event, nil, stored, before)
}

// yearsToRestore is how many years a restore at now would renew dom for:
// one, and more while its expiry would still be past
// (entities.Domain.CompleteRestore).
func yearsToRestore(dom *entities.Domain, now time.Time) int {
	years, exp := 1, dom.ExpiryDate.AddDate(1, 0, 0)
	for !exp.After(now) {
		exp = exp.AddDate(1, 0, 0)
		years++
	}
	return years
}

// restoreQuotes prices a restore of dom for its sponsor in the phase's base
// currency: the restore fee, and the renewal for years.
func (s *DomainService) restoreQuotes(ctx context.Context, dom *entities.Domain, years int, phase *entities.Phase) (*entities.Quote, *entities.Quote, error) {
	quote := func(t entities.TransactionType, years int) (*entities.Quote, error) {
		return s.GetQuote(ctx, &queries.QuoteRequest{
			DomainName:      dom.Name.String(),
			ClID:            dom.ClID.String(),
			TransactionType: t,
			Currency:        phase.Policy.BaseCurrency,
			Years:           years,
			PhaseName:       phase.Name.String(),
		})
	}
	restore, err := quote(entities.TransactionTypeRestore, 1)
	if err != nil {
		return nil, nil, err
	}
	renew, err := quote(entities.TransactionTypeRenewal, years)
	if err != nil {
		return nil, nil, err
	}
	return restore, renew, nil
}

// LapsedRestore is a domain whose restore lapsed: its report was not sent
// by RestoreEnd, its PendingRestoreEnd.
type LapsedRestore struct {
	DomainName string    `json:"domainName"`
	RoID       int64     `json:"roid"`
	RestoreEnd time.Time `json:"restoreEnd"`
}

// ListLapsedRestores lists up to limit domains whose restore lapsed by
// dueBy, in (RestoreEnd, ROID) order after the cursor (afterEnd,
// afterRoID): those RestoreWorkflow reverts.
func (s *DomainService) ListLapsedRestores(ctx context.Context, dueBy, afterEnd time.Time, afterRoID int64, limit int) ([]LapsedRestore, error) {
	doms, err := s.domainRepository.ListLapsedRestores(ctx, dueBy, afterEnd, afterRoID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]LapsedRestore, len(doms))
	for i, d := range doms {
		id, err := d.RoID.Int64()
		if err != nil {
			return nil, err
		}
		out[i] = LapsedRestore{DomainName: d.Name.String(), RoID: id, RestoreEnd: d.RGPStatus.PendingRestoreEnd}
	}
	return out, nil
}

// BatchRevertLapsedRestores reverts the restores of the named domains that
// lapsed by dueBy (entities.Domain.RevertRestore), each written over the
// domain as read with a poll message for its sponsor in the same
// transaction (entities.NewRestoreLapsedPollMessage), and recorded as a
// domain.restore_reverted event. It reads
// and checks every domain again, so a retry never reverts twice: a domain
// gone, restored meanwhile, already reverted, or whose restore has not
// lapsed is Skipped. A domain changed while it is reverted is read again,
// up to updateAttempts times. One domain's failure does not stop the
// others.
func (s *DomainService) BatchRevertLapsedRestores(ctx context.Context, names []string, dueBy time.Time) BatchResult {
	result := BatchResult{Succeeded: make([]string, 0, len(names)), Failed: make([]BatchFailure, 0)}
	for _, name := range names {
		reverted, err := s.revertIfLapsed(ctx, name, dueBy)
		switch {
		case err != nil:
			result.Failed = append(result.Failed, BatchFailure{DomainName: name, Error: err.Error()})
		case reverted:
			result.Succeeded = append(result.Succeeded, name)
		default:
			result.Skipped = append(result.Skipped, name)
		}
	}
	return result
}

// revertIfLapsed reverts the domain's restore if it lapsed by dueBy, and
// reports whether it did.
func (s *DomainService) revertIfLapsed(ctx context.Context, name string, dueBy time.Time) (_ bool, err error) {
	ctx, done := s.begin(ctx)
	defer done(&err)
	for attempt := 1; ; attempt++ {
		dom, err := s.domainRepository.GetDomainByName(ctx, name, false)
		if errors.Is(err, entities.ErrDomainNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !dom.RestoreLapsed(dueBy) {
			return false, nil
		}
		before := dom.DeepCopy()
		if err := dom.RevertRestore(dueBy); err != nil {
			return false, err
		}
		msg := entities.NewRestoreLapsedPollMessage(dom, before.RGPStatus.PendingRestoreEnd, restoreNow())
		stored, err := s.domainRepository.UpdateDomainColumnsIfUnchanged(ctx, dom, msg)
		if errors.Is(err, entities.ErrDomainChanged) && attempt < updateAttempts {
			continue
		}
		if err != nil {
			return false, err
		}
		event, err := entities.NewDomainLifeCycleEvent(stored.ClID.String(), "", stored.Name.ParentDomain(), stored.Name.String(), 0, entities.TransactionTypeRestore)
		if err != nil {
			return false, err
		}
		event.DomainRoID = stored.RoID.String()
		s.publishDomainEvent(ctx, EventDomainRestoreReverted,
			fmt.Sprintf("Restore of %s reverted: no report by %s", stored.Name, before.RGPStatus.PendingRestoreEnd.Format(time.RFC3339)),
			event, nil, stored, before)
		return true, nil
	}
}
