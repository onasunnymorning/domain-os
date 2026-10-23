package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// ErrTransfersNotConfigured: a transfer write on a DomainService built
// without a transfer repository (SetTransferRepo).
var ErrTransfersNotConfigured = errors.New("domain transfers are not configured")

// SetTransferRepo sets the repository the transfer writes go through. Only
// the processes that transfer domains (EPP, and the worker's lifecycle
// activities, for the auto-approval loop) need it; like SetTombstoneRepo, a
// setter keeps the other call sites as they are.
func (s *DomainService) SetTransferRepo(repo repositories.DomainTransferRepository) {
	s.transferRepo = repo
}

// Domain transfer events. Only domain.transferred is billable: it carries
// the gaining registrar's transfer quote and the years added. The others
// record the request's steps, with no years and no quote.
const (
	EventDomainTransferRequested = "domain.transfer_requested"
	EventDomainTransferred       = "domain.transferred"
	EventDomainTransferRejected  = "domain.transfer_rejected"
	EventDomainTransferCancelled = "domain.transfer_cancelled"
)

// RequestTransferAsRead starts a transfer of dom, a domain the caller has
// read, to gaining for years: entities.Domain.RequestTransfer checks it in
// the TLD's current GA phase, and the transfer must be quotable for the
// gaining registrar. The domain is written pendingTransfer only if it is
// still as it was read (else entities.ErrDomainChanged), with the transfer
// and the losing registrar's poll message, and the request is recorded as
// a domain.transfer_requested event.
func (s *DomainService) RequestTransferAsRead(ctx context.Context, dom *entities.Domain, gaining entities.ClIDType, authInfo string, years int) (*entities.Domain, *entities.DomainTransfer, error) {
	if s.transferRepo == nil {
		return nil, nil, ErrTransfersNotConfigured
	}
	phase, err := s.currentGAPhase(ctx, dom)
	if err != nil {
		return nil, nil, err
	}
	before := dom.DeepCopy()
	now := transferNow()
	tr, err := dom.RequestTransfer(gaining, authInfo, years, phase, now)
	if err != nil {
		return nil, nil, err
	}
	// A transfer the registry cannot quote (a currency without an exchange
	// rate, say) is refused now, not on approval. A phase with no transfer
	// price quotes it at no charge, as it does a registration or a renewal:
	// requiring a configured price is one rule for every transaction, not
	// for transfers alone.
	if _, err := s.transferQuote(ctx, dom, gaining, tr.Years, phase); err != nil {
		return nil, nil, err
	}
	stored, err := s.transferRepo.RequestTransfer(ctx, dom, tr, entities.NewTransferPollMessages(tr, gaining, now))
	if err != nil {
		return nil, nil, err
	}
	event, err := entities.NewDomainLifeCycleEvent(gaining.String(), "", stored.Name.ParentDomain(), stored.Name.String(), 0, entities.TransactionTypeTransfer)
	if err != nil {
		return nil, nil, err
	}
	event.DomainRoID = stored.RoID.String()
	s.publishDomainEvent(ctx, EventDomainTransferRequested,
		fmt.Sprintf("Transfer of %s from %s to %s requested", stored.Name, tr.LosingClID, gaining), event, tr, stored, before)
	return stored, tr, nil
}

// ResolveTransferAsRead ends tr, the pending transfer of dom, a domain the
// caller has read, with status as actor did (empty when the registry did).
// An approval completes the transfer (entities.Domain.CompleteTransfer) in
// the TLD's current GA phase, re-links dom's nameservers as
// nameserversToRelink plans from them (entities.TransferResolution), and is
// recorded as a billable domain.transferred event with the gaining
// registrar's transfer quote. A rejection or cancellation clears
// pendingTransfer. Every outcome queues poll messages for the registrars
// that did not act. Written only if dom and tr are still as read (else
// entities.ErrDomainChanged).
func (s *DomainService) ResolveTransferAsRead(ctx context.Context, dom *entities.Domain, tr *entities.DomainTransfer, status entities.TransferStatus, actor entities.ClIDType) (*entities.Domain, *entities.DomainTransfer, error) {
	if s.transferRepo == nil {
		return nil, nil, ErrTransfersNotConfigured
	}
	before := dom.DeepCopy()
	now := transferNow()
	if err := tr.Resolve(status, actor, now); err != nil {
		return nil, nil, err
	}
	res := &entities.TransferResolution{Domain: dom, Transfer: tr}
	var quote *entities.Quote
	if status.IsApproved() {
		phase, err := s.currentGAPhase(ctx, dom)
		if err != nil {
			return nil, nil, err
		}
		if quote, err = s.transferQuote(ctx, dom, tr.RequestingClID, tr.Years, phase); err != nil {
			return nil, nil, err
		}
		relink, err := s.nameserversToRelink(ctx, dom)
		if err != nil {
			return nil, nil, err
		}
		if err := dom.CompleteTransfer(tr, phase, now); err != nil {
			return nil, nil, err
		}
		for _, name := range relink {
			roid, err := s.roidService.GenerateRoid("host")
			if err != nil {
				return nil, nil, err
			}
			h, err := entities.NewHost(name, roid.String(), tr.RequestingClID.String())
			if err != nil {
				return nil, nil, err
			}
			res.GainingHosts = append(res.GainingHosts, h)
		}
	} else {
		dom.EndTransfer()
	}
	res.Messages = entities.NewTransferPollMessages(tr, actor, now)
	stored, err := s.transferRepo.ResolveTransfer(ctx, res)
	if err != nil {
		return nil, nil, err
	}

	eventType, clid, years := EventDomainTransferRejected, tr.LosingClID, 0
	switch {
	case status.IsApproved():
		eventType, clid, years = EventDomainTransferred, tr.RequestingClID, tr.Years
	case status == entities.TransferStatusClientCancelled || status == entities.TransferStatusServerCancelled:
		eventType, clid = EventDomainTransferCancelled, tr.RequestingClID
	}
	event, err := entities.NewDomainLifeCycleEvent(clid.String(), "", stored.Name.ParentDomain(), stored.Name.String(), years, entities.TransactionTypeTransfer)
	if err != nil {
		return nil, nil, err
	}
	event.DomainRoID = stored.RoID.String()
	if quote != nil {
		event.Quote = *quote
	}
	s.publishDomainEvent(ctx, eventType,
		fmt.Sprintf("Transfer of %s from %s to %s: %s", stored.Name, tr.LosingClID, tr.RequestingClID, tr.Status), event, tr, stored, before)
	return stored, tr, nil
}

// DueTransfer is a pending transfer due for an answer: its domain, and its
// place in the sweep's (ActionDate, ID) order, the cursor of the next page.
type DueTransfer struct {
	DomainName string    `json:"domainName"`
	ID         int64     `json:"id"`
	ActionDate time.Time `json:"actionDate"`
}

// ListDueTransfers lists, up to limit, the pending transfers of existing
// domains that were due for an answer by dueBy, in (ActionDate, ID) order
// after the cursor (afterDate, afterID; zero for the first page): the
// transfers the registry approves because the losing registrar did not
// answer in time (workflows.TransferApprovalLoop).
func (s *DomainService) ListDueTransfers(ctx context.Context, dueBy, afterDate time.Time, afterID int64, limit int) ([]DueTransfer, error) {
	if s.transferRepo == nil {
		return nil, ErrTransfersNotConfigured
	}
	due, err := s.transferRepo.ListDueTransfers(ctx, dueBy, afterDate, afterID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]DueTransfer, len(due))
	for i, t := range due {
		out[i] = DueTransfer{DomainName: t.DomainName.String(), ID: t.ID, ActionDate: t.ActionDate}
	}
	return out, nil
}

// BatchApproveDueTransfers approves, as the registry (serverApproved), the
// pending transfer of each domain named whose acDate is at or before dueBy:
// ResolveTransferAsRead, with poll messages to both registrars. Each domain
// is read and checked again, so the batch is idempotent under retries: a
// domain whose transfer was answered meanwhile, or is not yet due, or that
// no longer exists, is Skipped. A domain changed while being approved is
// read again, up to updateAttempts times. One domain's failure does not stop
// the batch.
func (s *DomainService) BatchApproveDueTransfers(ctx context.Context, names []string, dueBy time.Time) BatchResult {
	result := BatchResult{Succeeded: make([]string, 0, len(names)), Failed: make([]BatchFailure, 0)}
	for _, name := range names {
		approved, err := s.approveIfDue(ctx, name, dueBy)
		switch {
		case err != nil:
			result.Failed = append(result.Failed, BatchFailure{DomainName: name, Error: err.Error()})
		case approved:
			result.Succeeded = append(result.Succeeded, name)
		default:
			result.Skipped = append(result.Skipped, name)
		}
	}
	return result
}

// approveIfDue approves name's transfer if it is pending and due, reporting
// whether it did.
func (s *DomainService) approveIfDue(ctx context.Context, name string, dueBy time.Time) (bool, error) {
	for attempt := 1; ; attempt++ {
		dom, err := s.domainRepository.GetDomainByName(ctx, name, true)
		if errors.Is(err, entities.ErrDomainNotFound) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		tr, err := s.transferRepo.LatestTransfer(ctx, dom.RoID)
		if errors.Is(err, entities.ErrTransferNotPending) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if tr.Status != entities.TransferStatusPending || !dom.Status.PendingTransfer || tr.ActionDate.After(dueBy) {
			return false, nil
		}
		_, _, err = s.ResolveTransferAsRead(ctx, dom, tr, entities.TransferStatusServerApproved, "")
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, entities.ErrDomainChanged) || attempt == updateAttempts {
			return false, err
		}
	}
}

// transferNow is the time a transfer step happens at, to the microsecond
// the database keeps: what a command answers is then what a later query
// reads back.
func transferNow() time.Time { return time.Now().UTC().Truncate(time.Microsecond) }

// currentGAPhase is the current GA phase of dom's TLD, whose policy every
// transaction on an existing domain follows.
func (s *DomainService) currentGAPhase(ctx context.Context, dom *entities.Domain) (*entities.Phase, error) {
	tld, err := s.tldRepo.GetByName(ctx, dom.Name.ParentDomain(), true)
	if err != nil {
		return nil, err
	}
	return tld.GetCurrentGAPhase()
}

// transferQuote prices a transfer of dom to clid for years, in the phase's
// base currency.
func (s *DomainService) transferQuote(ctx context.Context, dom *entities.Domain, clid entities.ClIDType, years int, phase *entities.Phase) (*entities.Quote, error) {
	return s.GetQuote(ctx, &queries.QuoteRequest{
		DomainName:      dom.Name.String(),
		ClID:            clid.String(),
		TransactionType: entities.TransactionTypeTransfer,
		Currency:        phase.Policy.BaseCurrency,
		Years:           years,
		PhaseName:       phase.Name.String(),
	})
}

// nameserversToRelink names the nameservers of dom that the gaining
// registrar is to have its own host objects for (entities.TransferResolution):
// the hosts under dom (named dom or below it), which go with it, and the
// external hosts. A host
// under another domain of this registry belongs to that domain's sponsor
// and keeps its link: the gaining registrar cannot hold one of its own
// (RST epp-25 to epp-27), and a copy would publish a second set of glue.
func (s *DomainService) nameserversToRelink(ctx context.Context, dom *entities.Domain) ([]string, error) {
	served := map[string]bool{}
	var out []string
	for _, h := range dom.Hosts {
		name := h.Name.String()
		if name == dom.Name.String() || strings.HasSuffix(name, "."+dom.Name.String()) {
			out = append(out, name)
			continue
		}
		internal, err := s.underServedTLD(ctx, name, served)
		if err != nil {
			return nil, err
		}
		if !internal {
			out = append(out, name)
		}
	}
	return out, nil
}

// underServedTLD reports whether host name is under a TLD this registry
// serves (RFC 5732 §1.1: an internal host). served caches the answers.
func (s *DomainService) underServedTLD(ctx context.Context, name string, served map[string]bool) (bool, error) {
	labels := strings.Split(name, ".")
	for i := 1; i < len(labels); i++ {
		tld := strings.Join(labels[i:], ".")
		ok, seen := served[tld]
		if !seen {
			_, err := s.tldRepo.GetByName(ctx, tld, false)
			switch {
			case err == nil:
				ok = true
			case errors.Is(err, entities.ErrTLDNotFound):
			default:
				return false, fmt.Errorf("look up TLD %s: %w", tld, err)
			}
			served[tld] = ok
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}
