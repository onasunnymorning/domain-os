package postgres

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type EventRepositorySuite struct {
	suite.Suite
	db   *gorm.DB
	repo *PostgresEventRepository
}

func TestEventRepositorySuite(t *testing.T) {
	suite.Run(t, new(EventRepositorySuite))
}

func (s *EventRepositorySuite) SetupSuite() {
	s.db = setupTestDB()
	s.repo = NewPostgresEventRepository(s.db)
}

// seedEvents inserts test DomainEventRecords and returns them in insertion order.
func (s *EventRepositorySuite) seedEvents() []DomainEventRecord {
	// Clean up any existing test events
	s.db.Where("source LIKE ?", "test-%").Delete(&DomainEventRecord{})

	baseTime := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	emptyData, _ := json.Marshal(map[string]string{})

	records := []DomainEventRecord{
		{
			ID:            uuid.NewString(),
			Source:        "test-api",
			Type:          "domain.registered",
			Subject:       "example.com",
			OccurredAt:    baseTime,
			Data:          emptyData,
			Actor:         "admin@test.com",
			RoID:          "1001_DOM-APEX",
			TraceID:       "trace-reg-123",
			CorrelationID: "corr-reg-456",
		},
		{
			ID:         uuid.NewString(),
			Source:     "test-api",
			Type:       "domain.renewed",
			Subject:    "example.com",
			OccurredAt: baseTime.Add(1 * time.Hour),
			Data:       emptyData,
			Actor:      "admin@test.com",
			RoID:       "1001_DOM-APEX",
		},
		{
			ID:         uuid.NewString(),
			Source:     "test-worker",
			Type:       "domain.expired",
			Subject:    "other.net",
			OccurredAt: baseTime.Add(2 * time.Hour),
			Data:       emptyData,
			Actor:      "system",
			RoID:       "2002_DOM-APEX",
		},
		{
			ID:         uuid.NewString(),
			Source:     "test-api",
			Type:       "contact.created",
			Subject:    "CONTACT-001",
			OccurredAt: baseTime.Add(3 * time.Hour),
			Data:       emptyData,
			Actor:      "admin@test.com",
			RoID:       "",
		},
		{
			ID:         uuid.NewString(),
			Source:     "test-api",
			Type:       "domain.transferred",
			Subject:    "transfer.org",
			OccurredAt: baseTime.Add(4 * time.Hour),
			Data:       emptyData,
			Actor:      "registrar@test.com",
			RoID:       "3003_DOM-APEX",
		},
		{
			ID:         uuid.NewString(),
			Source:     "test-epp",
			Type:       "domain.updated",
			Subject:    "example.com",
			OccurredAt: baseTime.Add(5 * time.Hour),
			Data:       emptyData,
			Actor:      "",
			RoID:       "1001_DOM-APEX",
		},
	}

	for i := range records {
		err := s.db.Create(&records[i]).Error
		s.Require().NoError(err)
	}

	return records
}

func (s *EventRepositorySuite) TestSearchNoFilters() {
	records := s.seedEvents()
	ctx := context.Background()

	result, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		Limit: 3,
	})
	s.Require().NoError(err)
	s.Equal(int64(len(records)), result.TotalCount)
	s.Len(result.Events, 3)
	s.NotEmpty(result.NextCursor)
	s.Equal("hot", result.Tier)

	// Should be ordered by occurred_at DESC
	for i := 1; i < len(result.Events); i++ {
		s.True(result.Events[i-1].Time.After(result.Events[i].Time) || result.Events[i-1].Time.Equal(result.Events[i].Time))
	}
}

func (s *EventRepositorySuite) TestSearchBySubject() {
	s.seedEvents()
	ctx := context.Background()

	result, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		Subject: "example.com",
	})
	s.Require().NoError(err)
	s.Equal(int64(3), result.TotalCount) // registered, renewed, updated
	s.Len(result.Events, 3)
	for _, e := range result.Events {
		s.Equal("example.com", e.Subject)
	}
}

func (s *EventRepositorySuite) TestSearchByType() {
	s.seedEvents()
	ctx := context.Background()

	result, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		Type: "domain.registered",
	})
	s.Require().NoError(err)
	s.Equal(int64(1), result.TotalCount)
	s.Len(result.Events, 1)
	s.Equal("domain.registered", result.Events[0].Type)
}

func (s *EventRepositorySuite) TestSearchByTypePrefix() {
	s.seedEvents()
	ctx := context.Background()

	result, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		Type: "domain.*",
	})
	s.Require().NoError(err)
	s.Equal(int64(5), result.TotalCount) // registered, renewed, expired, transferred, updated
	s.Len(result.Events, 5)
	for _, e := range result.Events {
		s.True(len(e.Type) > 7 && e.Type[:7] == "domain.")
	}
}

func (s *EventRepositorySuite) TestSearchByActor() {
	s.seedEvents()
	ctx := context.Background()

	result, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		Actor: "admin@test.com",
	})
	s.Require().NoError(err)
	s.Equal(int64(3), result.TotalCount) // registered, renewed, contact.created
	for _, e := range result.Events {
		s.Equal("admin@test.com", e.Actor)
	}
}

func (s *EventRepositorySuite) TestSearchByRoID() {
	s.seedEvents()
	ctx := context.Background()

	result, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		RoID: "1001_DOM-APEX",
	})
	s.Require().NoError(err)
	s.Equal(int64(3), result.TotalCount) // registered, renewed, updated (all example.com)
	for _, e := range result.Events {
		s.Equal("1001_DOM-APEX", e.RoID)
	}
}

func (s *EventRepositorySuite) TestSearchByDateRange() {
	records := s.seedEvents()
	ctx := context.Background()

	// After the second event's time (inclusive), before the fifth event's time (exclusive)
	after := records[1].OccurredAt
	before := records[4].OccurredAt

	result, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		After:  &after,
		Before: &before,
	})
	s.Require().NoError(err)
	s.Equal(int64(3), result.TotalCount) // records[1], records[2], records[3]
	for _, e := range result.Events {
		s.True(!e.Time.Before(after), "event time should be >= after")
		s.True(e.Time.Before(before), "event time should be < before")
	}
}

func (s *EventRepositorySuite) TestSearchByTraceID() {
	s.seedEvents()
	ctx := context.Background()

	result, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		TraceID: "trace-reg-123",
	})
	s.Require().NoError(err)
	s.Equal(int64(1), result.TotalCount)
	s.Len(result.Events, 1)
	s.Equal("trace-reg-123", result.Events[0].TraceID)
}

func (s *EventRepositorySuite) TestSearchByCorrelationID() {
	s.seedEvents()
	ctx := context.Background()

	result, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		CorrelationID: "corr-reg-456",
	})
	s.Require().NoError(err)
	s.Equal(int64(1), result.TotalCount)
	s.Len(result.Events, 1)
	s.Equal("corr-reg-456", result.Events[0].CorrelationID)
}

func (s *EventRepositorySuite) TestSearchCursorPagination() {
	s.seedEvents()
	ctx := context.Background()

	// Page 1: fetch 3
	page1, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		Limit: 3,
	})
	s.Require().NoError(err)
	s.Len(page1.Events, 3)
	s.NotEmpty(page1.NextCursor)

	// Page 2: use cursor
	page2, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		Limit:  3,
		Cursor: page1.NextCursor,
	})
	s.Require().NoError(err)
	s.Len(page2.Events, 3)
	s.Empty(page2.NextCursor) // 6 total, 3+3 = done

	// Verify no overlap
	page1IDs := make(map[string]bool)
	for _, e := range page1.Events {
		page1IDs[e.ID] = true
	}
	for _, e := range page2.Events {
		s.False(page1IDs[e.ID], "page 2 event should not appear in page 1")
	}

	// Verify page2 events are older than page1 events
	oldestPage1 := page1.Events[len(page1.Events)-1].Time
	for _, e := range page2.Events {
		s.True(!e.Time.After(oldestPage1), "page 2 events should be older than or equal to page 1 events")
	}
}

func (s *EventRepositorySuite) TestSearchTotalCount() {
	s.seedEvents()
	ctx := context.Background()

	// Total count should be independent of limit and cursor
	page1, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		Limit: 2,
	})
	s.Require().NoError(err)
	s.Equal(int64(6), page1.TotalCount)

	page2, err := s.repo.SearchEvents(ctx, entities.EventSearchFilter{
		Limit:  2,
		Cursor: page1.NextCursor,
	})
	s.Require().NoError(err)
	s.Equal(int64(6), page2.TotalCount) // Same total regardless of cursor
}

func (s *EventRepositorySuite) TestEncodeDecode() {
	ts := time.Date(2026, 6, 15, 10, 30, 0, 123456789, time.UTC)
	id := uuid.NewString()

	cursor := EncodeCursor(ts, id)
	s.NotEmpty(cursor)

	decodedTime, decodedID, err := DecodeCursor(cursor)
	s.Require().NoError(err)
	s.True(ts.Equal(decodedTime), "round-trip time should match")
	s.Equal(id, decodedID)

	// Invalid cursors
	_, _, err = DecodeCursor("not-base64!!!")
	s.Error(err)

	_, _, err = DecodeCursor("bm9waXBl") // base64("nopipe") — no pipe separator
	s.Error(err)

	_, _, err = DecodeCursor("bm90LWEtdGltZXN0YW1wfHh4eA==") // base64("not-a-timestamp|xxx")
	s.Error(err)
}
