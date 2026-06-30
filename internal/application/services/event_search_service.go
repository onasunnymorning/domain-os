package services

import (
	"context"
	"fmt"
	"time"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

const (
	// DefaultRetentionDays is the number of days events are kept in PostgreSQL
	// before being archived to S3 and pruned. Matches the event-prune workflow config.
	DefaultRetentionDays = 30

	// DefaultSearchLimit is the default number of results per page.
	DefaultSearchLimit = 50

	// MaxSearchLimit is the maximum number of results per page.
	MaxSearchLimit = 200
)

// EventSearchService orchestrates event search across hot (PostgreSQL) and warm
// (S3 archive) storage tiers. It routes queries to the appropriate tier based on
// the date range in the filter:
//   - Hot: Events within the retention window (last 30 days) — fast PG queries
//   - Warm: Events older than the retention window — S3 JSONL.gz download + filter
//   - Mixed: Queries spanning both tiers — merge results, deduplicate by ID
type EventSearchService struct {
	hotRepo       repositories.EventRepository
	archiveReader *storage.EventArchiveReader
	retentionDays int
}

// NewEventSearchService creates a new EventSearchService. The archiveReader may
// be nil if S3 is not configured (warm-tier queries will return empty results).
func NewEventSearchService(hotRepo repositories.EventRepository, archiveReader *storage.EventArchiveReader, retentionDays int) *EventSearchService {
	if retentionDays <= 0 {
		retentionDays = DefaultRetentionDays
	}
	return &EventSearchService{
		hotRepo:       hotRepo,
		archiveReader: archiveReader,
		retentionDays: retentionDays,
	}
}

// Search executes a filtered, paginated event search across the appropriate
// storage tier(s). Returns results sorted by occurred_at DESC.
func (s *EventSearchService) Search(ctx context.Context, filter entities.EventSearchFilter) (*entities.EventSearchResult, error) {
	// Normalize limits
	if filter.Limit <= 0 {
		filter.Limit = DefaultSearchLimit
	}
	if filter.Limit > MaxSearchLimit {
		filter.Limit = MaxSearchLimit
	}

	tier := s.determineTier(filter)

	switch tier {
	case "hot":
		return s.searchHot(ctx, filter)
	case "warm":
		return s.searchWarm(ctx, filter)
	case "mixed":
		return s.searchMixed(ctx, filter)
	default:
		return s.searchHot(ctx, filter)
	}
}

// determineTier decides which storage tier(s) to query based on the date range
// relative to the retention boundary.
func (s *EventSearchService) determineTier(filter entities.EventSearchFilter) string {
	boundary := time.Now().UTC().AddDate(0, 0, -s.retentionDays)

	hasAfter := filter.After != nil
	hasBefore := filter.Before != nil

	// No date filters → default to hot (most recent events are what operators want)
	if !hasAfter && !hasBefore {
		return "hot"
	}

	// Both dates specified
	if hasAfter && hasBefore {
		if filter.Before.Before(boundary) || filter.Before.Equal(boundary) {
			// Entire range is in the past → warm only
			return "warm"
		}
		if filter.After.After(boundary) || filter.After.Equal(boundary) {
			// Entire range is within retention → hot only
			return "hot"
		}
		// Range spans the boundary → mixed
		return "mixed"
	}

	// Only After specified
	if hasAfter {
		if filter.After.Before(boundary) {
			// Starting from before retention boundary → mixed
			return "mixed"
		}
		// Starting from within retention → hot
		return "hot"
	}

	// Only Before specified
	if filter.Before.Before(boundary) || filter.Before.Equal(boundary) {
		// Ending before retention boundary → warm only
		return "warm"
	}
	// Ending within retention → could be mixed if we assume unbounded start
	return "hot"
}

// searchHot queries only PostgreSQL (hot tier).
func (s *EventSearchService) searchHot(ctx context.Context, filter entities.EventSearchFilter) (*entities.EventSearchResult, error) {
	result, err := s.hotRepo.SearchEvents(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("EventSearchService.searchHot: %w", err)
	}
	result.Tier = "hot"
	return result, nil
}

// searchWarm queries only S3 (warm tier).
func (s *EventSearchService) searchWarm(ctx context.Context, filter entities.EventSearchFilter) (*entities.EventSearchResult, error) {
	if s.archiveReader == nil {
		return &entities.EventSearchResult{
			Events:     []entities.DomainEvent{},
			TotalCount: 0,
			Tier:       "warm",
		}, nil
	}

	result, err := s.archiveReader.SearchArchive(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("EventSearchService.searchWarm: %w", err)
	}
	result.Tier = "warm"
	return result, nil
}

// searchMixed queries both PG and S3, merges results, deduplicates by ID,
// and returns sorted by occurred_at DESC up to the limit.
func (s *EventSearchService) searchMixed(ctx context.Context, filter entities.EventSearchFilter) (*entities.EventSearchResult, error) {
	// Query hot tier
	hotResult, err := s.searchHot(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("EventSearchService.searchMixed(hot): %w", err)
	}

	// Query warm tier
	warmResult, err := s.searchWarm(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("EventSearchService.searchMixed(warm): %w", err)
	}

	// Merge and deduplicate
	merged := s.mergeAndDeduplicate(hotResult.Events, warmResult.Events, filter.Limit)

	return &entities.EventSearchResult{
		Events:     merged,
		TotalCount: hotResult.TotalCount + warmResult.TotalCount,
		Tier:       "mixed",
	}, nil
}

// mergeAndDeduplicate combines events from hot and warm tiers, removes duplicates
// by ID, sorts by occurred_at DESC, and caps at limit.
func (s *EventSearchService) mergeAndDeduplicate(hot, warm []entities.DomainEvent, limit int) []entities.DomainEvent {
	seen := make(map[string]bool, len(hot))
	result := make([]entities.DomainEvent, 0, len(hot)+len(warm))

	// Add hot events first (already sorted DESC)
	for _, evt := range hot {
		seen[evt.ID] = true
		result = append(result, evt)
	}

	// Add warm events that aren't duplicates
	for _, evt := range warm {
		if !seen[evt.ID] {
			result = append(result, evt)
		}
	}

	// Sort merged results by occurred_at DESC (hot events are recent, warm are older,
	// but there may be overlap near the retention boundary)
	sortEventsByTimeDesc(result)

	// Cap at limit
	if len(result) > limit {
		result = result[:limit]
	}

	return result
}

// sortEventsByTimeDesc sorts events by Time descending using insertion sort
// (efficient for nearly-sorted data from two pre-sorted sources).
func sortEventsByTimeDesc(events []entities.DomainEvent) {
	for i := 1; i < len(events); i++ {
		key := events[i]
		j := i - 1
		for j >= 0 && events[j].Time.Before(key.Time) {
			events[j+1] = events[j]
			j--
		}
		events[j+1] = key
	}
}
