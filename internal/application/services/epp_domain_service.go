package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// Why a <domain:check> reports a name unavailable. They are the
// <domain:reason> text, which RFC 5730 bounds to 32 characters.
const (
	CheckReasonInvalid      = "Invalid domain name"
	CheckReasonNotServed    = "TLD not served by this registry"
	CheckReasonNotOpen      = "Not open for registration"
	CheckReasonInUse        = "In use"
	CheckReasonReserved     = "Reserved"
	CheckReasonPhasePolicy  = "Not allowed by registry policy"
	maxPhasesPerTLDForCheck = 100
)

// EPPDomainService answers registrars' domain commands over EPP
// (docs/epp/ARCHITECTURE.md §4). It reads through the same repositories as
// DomainService, but answers in EPP's terms: an unavailable name is a result
// with a reason, not an error.
type EPPDomainService struct {
	domains repositories.DomainRepository
	nndns   repositories.NNDNRepository
	tlds    repositories.TLDRepository
	phases  repositories.PhaseRepository
	// parallelism bounds the lookups one <check> runs at once.
	parallelism int
	now         func() time.Time
}

// NewEPPDomainService returns the service. parallelism bounds the
// concurrent lookups of one <check>.
func NewEPPDomainService(domains repositories.DomainRepository, nndns repositories.NNDNRepository, tlds repositories.TLDRepository, phases repositories.PhaseRepository, parallelism int) *EPPDomainService {
	if parallelism < 1 {
		parallelism = 1
	}
	return &EPPDomainService{domains: domains, nndns: nndns, tlds: tlds, phases: phases, parallelism: parallelism, now: time.Now}
}

// Check reports, for each name in order, whether it is available for
// registration in its TLD's current general-availability phase. A name is
// unavailable if it is not a valid domain name, its TLD is not served or
// not open, it is registered, it is reserved (on the NNDN list), or the
// phase's policy does not allow its label. Duplicates are answered each
// time. An error means a lookup failed; no partial answer is returned.
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

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
		slots    = make(chan struct{}, s.parallelism)
	)
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
		wg.Add(1)
		slots <- struct{}{}
		go func(r *queries.DomainCheckResult, name string) {
			defer func() { <-slots; wg.Done() }()
			reason, err := s.registeredOrReserved(ctx, name)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				if firstErr == nil {
					firstErr = err
					cancel()
				}
				return
			}
			r.Reason = reason
			r.Available = reason == ""
		}(results[i], dn.String())
	}
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
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
	phases, err := s.phases.ListPhasesByTLD(ctx, name, maxPhasesPerTLDForCheck, "")
	if err != nil {
		return nil, fmt.Errorf("list phases of %s: %w", name, err)
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

// registeredOrReserved returns the reason name is unavailable, or "" if it
// is neither registered nor reserved.
func (s *EPPDomainService) registeredOrReserved(ctx context.Context, name string) (string, error) {
	if _, err := s.domains.GetDomainByName(ctx, name, false); err == nil {
		return CheckReasonInUse, nil
	} else if !errors.Is(err, entities.ErrDomainNotFound) {
		return "", fmt.Errorf("look up domain %s: %w", name, err)
	}
	if _, err := s.nndns.GetNNDN(ctx, name); err == nil {
		return CheckReasonReserved, nil
	} else if !errors.Is(err, entities.ErrNNDNNotFound) {
		return "", fmt.Errorf("look up NNDN %s: %w", name, err)
	}
	return "", nil
}
