package services

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// The EPP <domain:transfer> command (RFC 5731 §3.2.4). The process, its
// rules and their order are described with entities.DomainTransfer; this
// file adds who may do what:
//
//   - request: any accredited registrar but the sponsor, with the domain's
//     transfer code;
//   - approve and reject: the sponsor (the losing registrar);
//   - cancel: the registrar that requested the transfer;
//   - query: the sponsor and the two registrars of the latest transfer, or
//     any registrar with the domain's transfer code.
//
// Anyone else is entities.ErrEPPNotSponsor (2201, ADR 0006), and learns
// nothing about the domain's transfers.

// TransferRequest asks for the domain to be transferred to the session's
// registrar, and returns the transfer, pending (1001). It checks, in order:
// the domain exists; the registrar is accredited for its TLD; the period
// is whole years; then the rules of entities.Domain.RequestTransfer.
func (s *EPPDomainService) TransferRequest(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPTransferDomainCommand) (*entities.DomainTransfer, error) {
	for attempt := 1; ; attempt++ {
		tr, err := s.transferRequest(ctx, scope, cmd)
		if !errors.Is(err, entities.ErrDomainChanged) || attempt == updateAttempts {
			return tr, err
		}
	}
}

func (s *EPPDomainService) transferRequest(ctx context.Context, scope entities.RegistrarClID, cmd commands.EPPTransferDomainCommand) (*entities.DomainTransfer, error) {
	dom, err := s.domain(ctx, scope, cmd.Name)
	if err != nil {
		return nil, err
	}
	accredited, err := s.registrars.IsRegistrarAccreditedForTLD(ctx, dom.Name.ParentDomain(), scope.String())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCouldNotDetermineAccreditation, err)
	}
	if !accredited {
		return nil, ErrRegistrarNotAccredited
	}
	years := 1
	if cmd.Period != 0 {
		years = cmd.Period
		if cmd.PeriodUnit == "m" {
			if cmd.Period%12 != 0 {
				return nil, entities.ErrEPPPeriodNotWholeYears
			}
			years = cmd.Period / 12
		}
	}
	code := ""
	if cmd.AuthInfo != nil {
		code = *cmd.AuthInfo
	}
	_, tr, err := s.register.RequestTransferAsRead(ctx, dom, entities.ClIDType(scope), code, years)
	return tr, err
}

// TransferApprove approves the domain's pending transfer: the session's
// registrar must sponsor the domain. The domain moves to the gaining
// registrar.
func (s *EPPDomainService) TransferApprove(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.DomainTransfer, error) {
	return s.answerTransfer(ctx, scope, name, entities.TransferStatusClientApproved)
}

// TransferReject rejects the domain's pending transfer: the session's
// registrar must sponsor the domain.
func (s *EPPDomainService) TransferReject(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.DomainTransfer, error) {
	return s.answerTransfer(ctx, scope, name, entities.TransferStatusClientRejected)
}

// TransferCancel cancels the domain's pending transfer: the session's
// registrar must be the one that requested it.
func (s *EPPDomainService) TransferCancel(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.DomainTransfer, error) {
	return s.answerTransfer(ctx, scope, name, entities.TransferStatusClientCancelled)
}

func (s *EPPDomainService) answerTransfer(ctx context.Context, scope entities.RegistrarClID, name string, status entities.TransferStatus) (*entities.DomainTransfer, error) {
	for attempt := 1; ; attempt++ {
		tr, err := s.answer(ctx, scope, name, status)
		if !errors.Is(err, entities.ErrDomainChanged) || attempt == updateAttempts {
			return tr, err
		}
	}
}

func (s *EPPDomainService) answer(ctx context.Context, scope entities.RegistrarClID, name string, status entities.TransferStatus) (*entities.DomainTransfer, error) {
	dom, err := s.domain(ctx, scope, name)
	if err != nil {
		return nil, err
	}
	tr, err := s.transfers.LatestTransfer(ctx, dom.RoID)
	if err != nil && !errors.Is(err, entities.ErrTransferNotPending) {
		return nil, err
	}
	me := entities.ClIDType(scope)
	// Who may act is checked before whether there is anything to act on, so
	// that a registrar with no part in the domain learns nothing.
	if status == entities.TransferStatusClientCancelled {
		if tr == nil || tr.RequestingClID != me {
			return nil, entities.ErrEPPNotSponsor
		}
	} else if dom.ClID != me {
		return nil, entities.ErrEPPNotSponsor
	}
	if tr == nil || tr.Status != entities.TransferStatusPending || !dom.Status.PendingTransfer {
		return nil, entities.ErrTransferNotPending
	}
	var relink []string
	if status.IsApproved() {
		if relink, err = s.nameserversToRelink(ctx, dom); err != nil {
			return nil, err
		}
	}
	_, tr, err = s.register.ResolveTransferAsRead(ctx, dom, tr, status, me, relink)
	return tr, err
}

// TransferQuery returns the domain's latest transfer, pending or not
// (RFC 5731 §3.1.3). The sponsor and the latest transfer's two registrars
// may ask; anyone else must give the domain's transfer code (2201 without,
// 2202 with a wrong one). A domain never transferred is
// entities.ErrTransferNotPending (2301).
func (s *EPPDomainService) TransferQuery(ctx context.Context, scope entities.RegistrarClID, name string, authInfo *string) (*entities.DomainTransfer, error) {
	dom, err := s.domain(ctx, scope, name)
	if err != nil {
		return nil, err
	}
	tr, err := s.transfers.LatestTransfer(ctx, dom.RoID)
	if err != nil && !errors.Is(err, entities.ErrTransferNotPending) {
		return nil, err
	}
	me := entities.ClIDType(scope)
	party := dom.ClID == me || (tr != nil && (tr.RequestingClID == me || tr.LosingClID == me))
	if !party {
		switch {
		case authInfo == nil:
			return nil, entities.ErrEPPNotSponsor
		case !dom.AuthInfo.Matches(*authInfo):
			return nil, entities.ErrEPPAuthInfoMismatch
		}
	}
	if tr == nil {
		return nil, entities.ErrTransferNotPending
	}
	return tr, nil
}

// domain returns the domain named name, with its nameservers, whoever
// sponsors it: the transfer commands decide who may see it.
func (s *EPPDomainService) domain(ctx context.Context, scope entities.RegistrarClID, name string) (*entities.Domain, error) {
	if err := scope.Validate(); err != nil {
		return nil, err
	}
	dn, err := entities.NewDomainName(name)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidDomain, err)
	}
	return s.domains.GetDomainByName(ctx, dn.String(), true)
}

// nameserversToRelink names the nameservers of dom that the gaining
// registrar is to have its own host objects for (entities.TransferResolution):
// the hosts under dom (named dom or below it), which go with it, and the
// external hosts. A host
// under another domain of this registry belongs to that domain's sponsor
// and keeps its link: the gaining registrar cannot hold one of its own
// (RST epp-25 to epp-27), and a copy would publish a second set of glue.
func (s *EPPDomainService) nameserversToRelink(ctx context.Context, dom *entities.Domain) ([]string, error) {
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
func (s *EPPDomainService) underServedTLD(ctx context.Context, name string, served map[string]bool) (bool, error) {
	labels := strings.Split(name, ".")
	for i := 1; i < len(labels); i++ {
		tld := strings.Join(labels[i:], ".")
		ok, seen := served[tld]
		if !seen {
			_, err := s.tlds.GetByName(ctx, tld, false)
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
