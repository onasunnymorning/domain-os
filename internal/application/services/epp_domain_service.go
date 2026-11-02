package services

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// Why a <domain:check> reports a name unavailable. They are the
// <domain:reason> text, which RFC 5730 bounds to 32 characters.
const (
	CheckReasonInvalid     = "Invalid domain name"
	CheckReasonNotServed   = "TLD not served by this registry"
	CheckReasonNotOpen     = "Not open for registration"
	CheckReasonInUse       = "In use"
	CheckReasonReserved    = "Reserved"
	CheckReasonPhasePolicy = "Not allowed by registry policy"
)

// phasePageSize is how many phases one repository call returns.
const phasePageSize = 100

// EPPDomainService answers registrars' domain commands over EPP
// (docs/epp/ARCHITECTURE.md §4). It reads through the same repositories as
// DomainService and registers through it, but answers in EPP's terms: an
// unavailable name is a result with a reason, and every refusal is a
// sentinel error the EPP adapter maps to a result code.
//
// The commands that touch sponsored objects take the session's registrar as
// a typed scope, and check sponsorship or accreditation before anything
// else (ADR 0006, rule 5).
type EPPDomainService struct {
	domains    repositories.DomainRepository
	nndns      repositories.NNDNRepository
	tlds       repositories.TLDRepository
	phases     repositories.PhaseRepository
	contacts   repositories.ContactRepository
	hosts      repositories.HostRepository
	registrars repositories.RegistrarRepository
	transfers  repositories.DomainTransferRepository
	register   EPPDomainRegistrar
	now        func() time.Time
}

// EPPDomainRegistrar writes the domain transforms: DomainService, which
// prices them, assigns ROIDs, stores them and publishes their events. Each
// write but registration takes the domain as the EPP service read and
// checked it, and is refused with entities.ErrDomainChanged if another
// command changed it since.
type EPPDomainRegistrar interface {
	RegisterDomain(ctx context.Context, cmd *commands.RegisterDomainCommand) (*entities.Domain, error)
	SaveDomainUpdate(ctx context.Context, before, after *entities.Domain, command interface{}) (*entities.Domain, error)
	RenewDomainAsRead(ctx context.Context, dom *entities.Domain, years int) (*entities.Domain, error)
	MarkDomainForDeletionAsRead(ctx context.Context, dom *entities.Domain) (*entities.Domain, error)
	// RequestTransferAsRead starts a transfer of dom to gaining
	// (entities.Domain.RequestTransfer), for years, and queues the poll
	// messages it causes.
	RequestTransferAsRead(ctx context.Context, dom *entities.Domain, gaining entities.ClIDType, authInfo string, years int) (*entities.Domain, *entities.DomainTransfer, error)
	// ResolveTransferAsRead ends tr, dom's pending transfer, with status as
	// actor did (empty for the registry). An approval re-links dom's
	// nameservers as entities.TransferResolution describes.
	ResolveTransferAsRead(ctx context.Context, dom *entities.Domain, tr *entities.DomainTransfer, status entities.TransferStatus, actor entities.ClIDType) (*entities.Domain, *entities.DomainTransfer, error)
	// RequestRestoreAsRead requests the restore of dom for actor, its
	// sponsor (entities.Domain.RequestRestore); a phase that requires no
	// report restores it at once.
	RequestRestoreAsRead(ctx context.Context, dom *entities.Domain, actor entities.ClIDType) (*entities.Domain, error)
	// CompleteRestoreAsRead restores dom, whose restore is pending, with
	// its sponsor's report (entities.Domain.CompleteRestore).
	CompleteRestoreAsRead(ctx context.Context, dom *entities.Domain, report entities.RestoreReport, actor entities.ClIDType) (*entities.Domain, error)
}

// EPPDomainServiceDeps are what EPPDomainService reads and writes through.
type EPPDomainServiceDeps struct {
	Domains    repositories.DomainRepository
	NNDNs      repositories.NNDNRepository
	TLDs       repositories.TLDRepository
	Phases     repositories.PhaseRepository
	Contacts   repositories.ContactRepository
	Hosts      repositories.HostRepository
	Registrars repositories.RegistrarRepository
	// Transfers reads the domains' transfers; Register writes them.
	Transfers repositories.DomainTransferRepository
	Register  EPPDomainRegistrar
}

// NewEPPDomainService returns the service.
func NewEPPDomainService(d EPPDomainServiceDeps) *EPPDomainService {
	return &EPPDomainService{
		domains: d.Domains, nndns: d.NNDNs, tlds: d.TLDs, phases: d.Phases,
		contacts: d.Contacts, hosts: d.Hosts, registrars: d.Registrars, transfers: d.Transfers, register: d.Register,
		now: time.Now,
	}
}

// Check reports, for each name in order, whether it is available for
// registration in its TLD's current general-availability phase. A name is
// unavailable if it is not a valid domain name, its TLD is not served or
// not open, it is registered, it is reserved (on the NNDN list), or the
// phase's policy does not allow its label. Duplicates are answered each
// time. An error means a lookup failed; no partial answer is returned.
//
// A command costs two lookups however many names it carries, one against
// the domains and one against the reserved names, each over the names'
// unique index: <check> is the most frequent EPP command by far, and a
// drop-catching client sends it with many names at once.
//
// Check is not scoped to a registrar: availability is the same for all
// (ADR 0006 scopes reads and writes of sponsored objects, not this).
func (s *EPPDomainService) Check(ctx context.Context, names []string) ([]*queries.DomainCheckResult, error) {
	results := make([]*queries.DomainCheckResult, len(names))
	parsed := make([]*entities.DomainName, len(names))
	tlds := map[string]*tldForCheck{}
	for i, raw := range names {
		results[i] = &queries.DomainCheckResult{TimeStamp: s.now().UTC(), DomainName: raw}
		dn, err := entities.NewDomainName(raw)
		if err != nil || !strings.Contains(dn.String(), ".") {
			results[i].Reason = CheckReasonInvalid
			continue
		}
		parsed[i] = dn
		tlds[dn.ParentDomain()] = nil
	}
	for name := range tlds {
		t, err := s.tldForCheck(ctx, name)
		if err != nil {
			return nil, err
		}
		tlds[name] = t
	}

	// The names the TLD and its policy do not already answer, each once.
	var lookup []string
	seen := map[string]bool{}
	for i, dn := range parsed {
		if dn == nil {
			continue
		}
		t := tlds[dn.ParentDomain()]
		switch {
		case t.reason != "":
			results[i].Reason = t.reason
			continue
		case !t.phase.Policy.LabelIsAllowed(dn.Label()):
			results[i].PhaseName = t.phase.Name.String()
			results[i].Reason = CheckReasonPhasePolicy
			continue
		}
		results[i].PhaseName = t.phase.Name.String()
		if n := dn.String(); !seen[n] {
			seen[n] = true
			lookup = append(lookup, n)
		}
	}
	registered, reserved, err := s.registeredOrReserved(ctx, lookup)
	if err != nil {
		return nil, err
	}
	for i, dn := range parsed {
		if dn == nil || results[i].Reason != "" {
			continue
		}
		switch n := dn.String(); {
		case registered[n]:
			results[i].Reason = CheckReasonInUse
		case reserved[n]:
			results[i].Reason = CheckReasonReserved
		default:
			results[i].Available = true
		}
	}
	return results, nil
}

// tldForCheck is what Check needs to know about a TLD: its current GA
// phase, or why no name in it is available.
type tldForCheck struct {
	phase  *entities.Phase
	reason string
}

func (s *EPPDomainService) tldForCheck(ctx context.Context, name string) (*tldForCheck, error) {
	// Phases are listed oldest first, a page at a time; a TLD that has
	// accumulated many has its current GA phase on a later page.
	var phases []*entities.Phase
	for cursor := ""; ; {
		page, err := s.phases.ListPhasesByTLD(ctx, name, phasePageSize, cursor)
		if err != nil {
			return nil, fmt.Errorf("list phases of %s: %w", name, err)
		}
		phases = append(phases, page...)
		if len(page) < phasePageSize {
			break
		}
		cursor = strconv.FormatInt(page[len(page)-1].ID, 10)
	}
	if len(phases) == 0 {
		// No phases: either the TLD is not served, or it is but has never
		// opened. Only this case pays for the TLD lookup.
		if _, err := s.tlds.GetByName(ctx, name, false); err != nil {
			if errors.Is(err, entities.ErrTLDNotFound) {
				return &tldForCheck{reason: CheckReasonNotServed}, nil
			}
			return nil, fmt.Errorf("look up TLD %s: %w", name, err)
		}
		return &tldForCheck{reason: CheckReasonNotOpen}, nil
	}
	t := entities.TLD{Phases: make([]entities.Phase, 0, len(phases))}
	for _, p := range phases {
		t.Phases = append(t.Phases, *p)
	}
	phase, err := t.GetCurrentGAPhase()
	if err != nil {
		return &tldForCheck{reason: CheckReasonNotOpen}, nil //nolint:nilerr // no current GA phase is an answer, not a failure
	}
	return &tldForCheck{phase: phase}, nil
}

// registeredOrReserved looks names up at once, and returns which of them
// are registered and which are reserved. No lookup is made for no names.
func (s *EPPDomainService) registeredOrReserved(ctx context.Context, names []string) (registered, reserved map[string]bool, err error) {
	if len(names) == 0 {
		return nil, nil, nil
	}
	found, err := s.domains.ExistingNames(ctx, names)
	if err != nil {
		return nil, nil, fmt.Errorf("look up %d domains: %w", len(names), err)
	}
	registered = asSet(found)
	found, err = s.nndns.ExistingNames(ctx, names)
	if err != nil {
		return nil, nil, fmt.Errorf("look up %d reserved names: %w", len(names), err)
	}
	return registered, asSet(found), nil
}

// asSet is names as a set.
func asSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	return set
}
