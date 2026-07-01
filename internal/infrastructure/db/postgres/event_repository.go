package postgres

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"gorm.io/gorm"
)

const (
	defaultEventLimit = 50
	maxEventLimit     = 200
)

// PostgresEventRepository implements repositories.EventRepository using PostgreSQL.
type PostgresEventRepository struct {
	db *gorm.DB
}

// NewPostgresEventRepository creates a new PostgresEventRepository.
func NewPostgresEventRepository(db *gorm.DB) *PostgresEventRepository {
	return &PostgresEventRepository{db: db}
}

// EncodeCursor encodes an occurred_at timestamp and event ID into an opaque cursor string.
func EncodeCursor(occurredAt time.Time, id string) string {
	raw := occurredAt.Format(time.RFC3339Nano) + "|" + id
	return base64.StdEncoding.EncodeToString([]byte(raw))
}

// DecodeCursor decodes an opaque cursor string into an occurred_at timestamp and event ID.
func DecodeCursor(cursor string) (time.Time, string, error) {
	decoded, err := base64.StdEncoding.DecodeString(cursor)
	if err != nil {
		return time.Time{}, "", fmt.Errorf("DecodeCursor(base64): %w", err)
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 {
		return time.Time{}, "", fmt.Errorf("DecodeCursor: invalid cursor format, expected 'timestamp|id'")
	}
	t, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return time.Time{}, "", fmt.Errorf("DecodeCursor(time.Parse): %w", err)
	}
	return t, parts[1], nil
}

// SearchEvents searches for domain events matching the given filter with cursor-based pagination.
func (r *PostgresEventRepository) SearchEvents(ctx context.Context, filter entities.EventSearchFilter) (*entities.EventSearchResult, error) {
	// Normalize limit
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultEventLimit
	}
	if limit > maxEventLimit {
		limit = maxEventLimit
	}

	// Build base query with WHERE conditions
	query := r.db.WithContext(ctx).Model(&DomainEventRecord{})
	countQuery := r.db.WithContext(ctx).Model(&DomainEventRecord{})

	// Apply filter conditions to both queries
	if filter.Subject != "" {
		query = query.Where("subject = ?", filter.Subject)
		countQuery = countQuery.Where("subject = ?", filter.Subject)
	}
	if filter.Type != "" {
		if strings.HasSuffix(filter.Type, "*") {
			prefix := strings.TrimSuffix(filter.Type, "*") + "%"
			query = query.Where("type LIKE ?", prefix)
			countQuery = countQuery.Where("type LIKE ?", prefix)
		} else {
			query = query.Where("type = ?", filter.Type)
			countQuery = countQuery.Where("type = ?", filter.Type)
		}
	}
	if filter.Source != "" {
		query = query.Where("source = ?", filter.Source)
		countQuery = countQuery.Where("source = ?", filter.Source)
	}
	if filter.Actor != "" {
		query = query.Where("actor = ?", filter.Actor)
		countQuery = countQuery.Where("actor = ?", filter.Actor)
	}
	if filter.RoID != "" {
		query = query.Where("ro_id = ?", filter.RoID)
		countQuery = countQuery.Where("ro_id = ?", filter.RoID)
	}
	if filter.TraceID != "" {
		query = query.Where("trace_id = ?", filter.TraceID)
		countQuery = countQuery.Where("trace_id = ?", filter.TraceID)
	}
	if filter.CorrelationID != "" {
		query = query.Where("correlation_id = ?", filter.CorrelationID)
		countQuery = countQuery.Where("correlation_id = ?", filter.CorrelationID)
	}
	if filter.After != nil {
		query = query.Where("occurred_at >= ?", *filter.After)
		countQuery = countQuery.Where("occurred_at >= ?", *filter.After)
	}
	if filter.Before != nil {
		query = query.Where("occurred_at < ?", *filter.Before)
		countQuery = countQuery.Where("occurred_at < ?", *filter.Before)
	}

	// Total count (without cursor/limit)
	var totalCount int64
	if err := countQuery.Count(&totalCount).Error; err != nil {
		return nil, fmt.Errorf("SearchEvents(count): %w", err)
	}

	// Apply cursor-based pagination
	if filter.Cursor != "" {
		cursorTime, cursorID, err := DecodeCursor(filter.Cursor)
		if err != nil {
			return nil, fmt.Errorf("SearchEvents(cursor): %w", err)
		}
		query = query.Where("(occurred_at < ? OR (occurred_at = ? AND id < ?))", cursorTime, cursorTime, cursorID)
	}

	// Order and fetch limit+1 rows
	var records []DomainEventRecord
	if err := query.Order("occurred_at DESC, id DESC").Limit(limit + 1).Find(&records).Error; err != nil {
		return nil, fmt.Errorf("SearchEvents(find): %w", err)
	}

	// Determine next cursor
	var nextCursor string
	if len(records) > limit {
		// There are more results — use the last returned event as cursor
		lastRecord := records[limit-1]
		nextCursor = EncodeCursor(lastRecord.OccurredAt, lastRecord.ID)
		records = records[:limit] // Trim to requested limit
	}

	// Convert records to domain events
	events := make([]entities.DomainEvent, 0, len(records))
	for _, record := range records {
		event, err := record.ToDomainEvent()
		if err != nil {
			return nil, fmt.Errorf("SearchEvents(ToDomainEvent id=%s): %w", record.ID, err)
		}
		events = append(events, event)
	}

	return &entities.EventSearchResult{
		Events:     events,
		NextCursor: nextCursor,
		TotalCount: totalCount,
		Tier:       "hot",
	}, nil
}
