package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/appcontext"
	"github.com/onasunnymorning/domain-os/internal/application/interfaces"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

// TMCHService implements interfaces.TMCHService (ADR 0024 §4, roadmap 9a).
// A refresh fetches each set and stores it only when the Clearinghouse has
// generated a newer one than the one stored; the repository writes the set
// and its outbox event in one transaction (INV-01), so what sunrise and
// claims read is always a complete list.
type TMCHService struct {
	repo    repositories.TMCHRepository
	fetcher interfaces.TMCHFetcher
	// source names the process in the events (DomainEvent.Source).
	source string
	now    func() time.Time
}

var _ interfaces.TMCHService = (*TMCHService)(nil)

// NewTMCHService builds the service; fetcher may be nil when the
// installation has no TMCH feed, in which case Refresh refuses and Status
// still answers.
func NewTMCHService(repo repositories.TMCHRepository, fetcher interfaces.TMCHFetcher, source string) *TMCHService {
	return &TMCHService{repo: repo, fetcher: fetcher, source: source, now: func() time.Time { return time.Now().UTC() }}
}

// Configured reports whether there is a feed to refresh from.
func (s *TMCHService) Configured() bool {
	return s.fetcher != nil && s.fetcher.Configured()
}

// Status reports each set's freshness.
func (s *TMCHService) Status(ctx context.Context) (*entities.TMCHStatus, error) {
	return s.repo.Status(ctx)
}

// Refresh fetches the three sets and stores each newer generation.
func (s *TMCHService) Refresh(ctx context.Context) (*queries.TMCHRefreshResult, error) {
	result := &queries.TMCHRefreshResult{StartedAt: s.now()}
	if !s.Configured() {
		result.CompletedAt = s.now()
		return result, entities.ErrTMCHNotConfigured
	}
	stored, err := s.repo.Status(ctx)
	if err != nil {
		result.CompletedAt = s.now()
		return result, fmt.Errorf("read TMCH status: %w", err)
	}

	var failures []error
	record := func(set string, outcome string, status *entities.TMCHSetStatus, err error) {
		r := queries.TMCHSetRefresh{Set: set, Outcome: outcome, Status: status}
		if err != nil {
			r.Outcome = queries.TMCHFailed
			r.Error = err.Error()
			failures = append(failures, fmt.Errorf("%s: %w", set, err))
		}
		result.Sets = append(result.Sets, r)
	}

	// Certificates first: a new bundle is what a new SMDRL or DNL may assume.
	if bundle, err := s.fetcher.FetchCertificates(ctx); err != nil {
		record(entities.TMCHSetCertificates, "", stored.Certificates, err)
	} else {
		status := bundle.Status()
		if !newer(stored.Certificates, status) {
			record(entities.TMCHSetCertificates, queries.TMCHUnchanged, stored.Certificates, nil)
		} else if err := s.repo.ReplaceCertificates(ctx, bundle, s.events(ctx, status)); err != nil {
			record(entities.TMCHSetCertificates, "", stored.Certificates, fmt.Errorf("store: %w", err))
		} else {
			record(entities.TMCHSetCertificates, queries.TMCHRefreshed, &status, nil)
		}
	}

	if list, err := s.fetcher.FetchSMDRL(ctx); err != nil {
		record(entities.TMCHSetSMDRL, "", stored.SMDRL, err)
	} else {
		status := list.Status()
		if !newer(stored.SMDRL, status) {
			record(entities.TMCHSetSMDRL, queries.TMCHUnchanged, stored.SMDRL, nil)
		} else if err := s.repo.ReplaceSMDRL(ctx, list, s.events(ctx, status)); err != nil {
			record(entities.TMCHSetSMDRL, "", stored.SMDRL, fmt.Errorf("store: %w", err))
		} else {
			record(entities.TMCHSetSMDRL, queries.TMCHRefreshed, &status, nil)
		}
	}

	if list, err := s.fetcher.FetchDNL(ctx); err != nil {
		record(entities.TMCHSetDNL, "", stored.DNL, err)
	} else {
		status := list.Status()
		if !newer(stored.DNL, status) {
			record(entities.TMCHSetDNL, queries.TMCHUnchanged, stored.DNL, nil)
		} else if err := s.repo.ReplaceDNL(ctx, list, s.events(ctx, status)); err != nil {
			record(entities.TMCHSetDNL, "", stored.DNL, fmt.Errorf("store: %w", err))
		} else {
			record(entities.TMCHSetDNL, queries.TMCHRefreshed, &status, nil)
		}
	}

	result.CompletedAt = s.now()
	return result, errors.Join(failures...)
}

// newer reports whether a fetched set should replace the stored one: there
// is none, or the Clearinghouse generated the fetched one later. The same
// generation fetched again is not a change, and an older one (a mirror
// behind the one last read) never replaces a newer one.
func newer(stored *entities.TMCHSetStatus, fetched entities.TMCHSetStatus) bool {
	return stored == nil || fetched.GeneratedAt.After(stored.GeneratedAt)
}

// events builds the refresh event for a set, carrying the request's trace
// and actor when there is one (a manual refresh over the admin API).
func (s *TMCHService) events(ctx context.Context, status entities.TMCHSetStatus) []entities.DomainEvent {
	ev, err := entities.NewTMCHRefreshEvent(s.source, status)
	if err != nil {
		return nil
	}
	if traceID, ok := appcontext.TraceID(ctx); ok {
		ev.TraceID = traceID
	}
	if correlationID, ok := appcontext.CorrelationID(ctx); ok {
		ev.CorrelationID = correlationID
	}
	if actor, ok := appcontext.UserID(ctx); ok {
		ev.Actor = actor
	}
	return []entities.DomainEvent{ev}
}
