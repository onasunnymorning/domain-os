package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// ErrTransfersNotConfigured: a transfer write on a DomainService built
// without a transfer repository (SetTransferRepo).
var ErrTransfersNotConfigured = errors.New("domain transfers are not configured")

// SetTransferRepo sets the repository the transfer writes go through. Only
// the processes that transfer domains (EPP, and the auto-approval workflow
// to come) need it; like SetTombstoneRepo, a setter keeps the other call
// sites as they are.
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
// the TLD's current GA phase, and the transfer must have a price for the
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
	now := time.Now().UTC()
	tr, err := dom.RequestTransfer(gaining, authInfo, years, phase, now)
	if err != nil {
		return nil, nil, err
	}
	// A transfer the registry cannot price is refused now, not on approval.
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
// the TLD's current GA phase, re-links the nameservers named in relink to
// the gaining registrar's hosts (entities.TransferResolution), and is
// recorded as a billable domain.transferred event with the gaining
// registrar's transfer quote. A rejection or cancellation clears
// pendingTransfer. Every outcome queues poll messages for the registrars
// that did not act. Written only if dom and tr are still as read (else
// entities.ErrDomainChanged).
func (s *DomainService) ResolveTransferAsRead(ctx context.Context, dom *entities.Domain, tr *entities.DomainTransfer, status entities.TransferStatus, actor entities.ClIDType, relink []string) (*entities.Domain, *entities.DomainTransfer, error) {
	if s.transferRepo == nil {
		return nil, nil, ErrTransfersNotConfigured
	}
	before := dom.DeepCopy()
	now := time.Now().UTC()
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
