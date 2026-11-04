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
// A refresh fetches each set and has the repository store it when it
// supersedes the stored one, deciding under the set's lock and writing the
// set and its outbox event in one transaction (INV-01), so what sunrise and
// claims read is always a complete list and never an older one.
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

// Refresh fetches the three sets and stores each one that supersedes the
// stored one. The decision is the repository's, under the set's lock
// (entities.ErrTMCHListStale means "nothing newer": the fetch is recorded
// and the set reported unchanged); the read of the stored status here only
// spares the fetch of a set already known, never decides.
func (s *TMCHService) Refresh(ctx context.Context) (*queries.TMCHRefreshResult, error) {
	result := &queries.TMCHRefreshResult{StartedAt: s.now()}
	if !s.Configured() {
		result.CompletedAt = s.now()
		return result, entities.ErrTMCHNotConfigured
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
	// store runs a replace and turns its outcome into the set's record: a
	// stale answer is an unchanged set, whose stored status is then read.
	store := func(set string, fetched entities.TMCHSetStatus, replace func(events []entities.DomainEvent) error) {
		events, err := s.events(ctx, fetched)
		if err != nil {
			record(set, "", nil, err)
			return
		}
		switch err := replace(events); {
		case errors.Is(err, entities.ErrTMCHListStale):
			record(set, queries.TMCHUnchanged, s.stored(ctx, set), nil)
		case err != nil:
			record(set, "", s.stored(ctx, set), fmt.Errorf("store: %w", err))
		default:
			record(set, queries.TMCHRefreshed, &fetched, nil)
		}
	}

	// Certificates first: a new bundle is what a new SMDRL or DNL may assume.
	if bundle, err := s.fetcher.FetchCertificates(ctx); err != nil {
		record(entities.TMCHSetCertificates, "", s.stored(ctx, entities.TMCHSetCertificates), err)
	} else {
		store(entities.TMCHSetCertificates, bundle.Status(), func(ev []entities.DomainEvent) error { return s.repo.ReplaceCertificates(ctx, bundle, ev) })
	}
	if list, err := s.fetcher.FetchSMDRL(ctx); err != nil {
		record(entities.TMCHSetSMDRL, "", s.stored(ctx, entities.TMCHSetSMDRL), err)
	} else {
		store(entities.TMCHSetSMDRL, list.Status(), func(ev []entities.DomainEvent) error { return s.repo.ReplaceSMDRL(ctx, list, ev) })
	}
	if list, err := s.fetcher.FetchDNL(ctx); err != nil {
		record(entities.TMCHSetDNL, "", s.stored(ctx, entities.TMCHSetDNL), err)
	} else {
		store(entities.TMCHSetDNL, list.Status(), func(ev []entities.DomainEvent) error { return s.repo.ReplaceDNL(ctx, list, ev) })
	}

	result.CompletedAt = s.now()
	return result, errors.Join(failures...)
}

// stored is the status of a set as stored now, nil if none or unreadable.
func (s *TMCHService) stored(ctx context.Context, set string) *entities.TMCHSetStatus {
	status, err := s.repo.Status(ctx)
	if err != nil {
		return nil
	}
	st, err := status.Set(set)
	if err != nil {
		return nil
	}
	return st
}

// events builds the refresh event for a set, carrying the request's trace
// and actor when there is one (a manual refresh over the admin API). An
// event that cannot be built is an error, not a silent gap (INV-01).
func (s *TMCHService) events(ctx context.Context, status entities.TMCHSetStatus) ([]entities.DomainEvent, error) {
	ev, err := entities.NewTMCHRefreshEvent(s.source, status)
	if err != nil {
		return nil, err
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
	return []entities.DomainEvent{ev}, nil
}
