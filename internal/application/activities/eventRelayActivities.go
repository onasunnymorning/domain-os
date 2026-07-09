package activities

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/storage"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.temporal.io/sdk/activity"
	"gorm.io/gorm"
)

// EventRelayActivities holds dependencies for the event relay workflow activities
// that handle outbox-pattern publishing: fetching unpublished events, archiving
// them to S3, marking them published, and pruning old events.
type EventRelayActivities struct {
	db *gorm.DB
	s3 *storage.S3Client
}

// NewEventRelayActivities creates a new EventRelayActivities with initialized DB
// and S3 dependencies. Follows the same DB init pattern as LifecycleActivities
// and the S3 init pattern from SnapshotActivities.
func NewEventRelayActivities() (*EventRelayActivities, error) {
	// Initialize S3 client, pointed at the event logs bucket (not escrow —
	// event archives have a different retention/access profile).
	s3c, err := storage.NewEventLogsS3Client()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize S3 for event relay activities: %w", err)
	}

	// Initialize gorm DB
	var db *gorm.DB
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		db, err = postgres.NewConnectionFromURL(dbURL, false)
	} else {
		dbCfg := postgres.Config{
			User:    os.Getenv("DB_USER"),
			Pass:    os.Getenv("DB_PASS"),
			Host:    os.Getenv("DB_HOST"),
			Port:    os.Getenv("DB_PORT"),
			DBName:  os.Getenv("DB_NAME"),
			SSLmode: os.Getenv("DB_SSLMODE"),
		}
		db, err = postgres.NewConnection(dbCfg)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DB for event relay activities: %w", err)
	}

	return &EventRelayActivities{
		db: db,
		s3: s3c,
	}, nil
}

// FetchUnpublishedEvents queries the domain_events table for unpublished events
// ordered by occurred_at ascending, limited to batchSize. Returns the events
// converted to domain entities.
func (a *EventRelayActivities) FetchUnpublishedEvents(ctx context.Context, batchSize int) ([]entities.DomainEvent, error) {
	var records []postgres.DomainEventRecord
	result := a.db.WithContext(ctx).
		Where("published = ?", false).
		Order("occurred_at ASC").
		Limit(batchSize).
		Find(&records)
	if result.Error != nil {
		return nil, fmt.Errorf("FetchUnpublishedEvents(batchSize=%d): %w", batchSize, result.Error)
	}

	activity.RecordHeartbeat(ctx, fmt.Sprintf("fetched %d unpublished events", len(records)))

	events := make([]entities.DomainEvent, 0, len(records))
	for _, rec := range records {
		evt, err := rec.ToDomainEvent()
		if err != nil {
			return nil, fmt.Errorf("FetchUnpublishedEvents: failed to convert record %s: %w", rec.ID, err)
		}
		events = append(events, evt)
	}

	return events, nil
}

// ArchiveEventsToS3 marshals the given events as gzip-compressed JSONL and
// uploads them to S3. Returns the S3 key of the uploaded archive.
// Key format: events/archive/{year}/{month}/{day}/events-{timestamp}-{count}.jsonl.gz
func (a *EventRelayActivities) ArchiveEventsToS3(ctx context.Context, events []entities.DomainEvent) (string, error) {
	if len(events) == 0 {
		return "", fmt.Errorf("ArchiveEventsToS3: no events to archive")
	}

	// Build JSONL content
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	enc := json.NewEncoder(gz)

	for _, evt := range events {
		if err := enc.Encode(evt); err != nil {
			gz.Close()
			return "", fmt.Errorf("ArchiveEventsToS3: failed to encode event %s: %w", evt.ID, err)
		}
	}
	if err := gz.Close(); err != nil {
		return "", fmt.Errorf("ArchiveEventsToS3: failed to close gzip writer: %w", err)
	}

	// Build S3 key
	now := time.Now().UTC()
	key := fmt.Sprintf("events/archive/%d/%02d/%02d/events-%d-%d.jsonl.gz",
		now.Year(), now.Month(), now.Day(),
		now.Unix(), len(events),
	)

	activity.RecordHeartbeat(ctx, fmt.Sprintf("uploading %d events to %s", len(events), key))

	// Upload
	if err := a.s3.UploadStream(ctx, key, &buf, "application/gzip"); err != nil {
		return "", fmt.Errorf("ArchiveEventsToS3(key=%s): %w", key, err)
	}

	return key, nil
}

// MarkEventsPublished sets published = true for the given event IDs.
// Returns the number of rows affected.
func (a *EventRelayActivities) MarkEventsPublished(ctx context.Context, eventIDs []string) (int64, error) {
	if len(eventIDs) == 0 {
		return 0, nil
	}

	result := a.db.WithContext(ctx).
		Model(&postgres.DomainEventRecord{}).
		Where("id IN ?", eventIDs).
		Update("published", true)
	if result.Error != nil {
		return 0, fmt.Errorf("MarkEventsPublished(%d events): %w", len(eventIDs), result.Error)
	}

	activity.RecordHeartbeat(ctx, fmt.Sprintf("marked %d events as published", result.RowsAffected))

	return result.RowsAffected, nil
}

// CountUnpublishedEvents returns the number of domain events where published = false.
func (a *EventRelayActivities) CountUnpublishedEvents(ctx context.Context) (int64, error) {
	var count int64
	result := a.db.WithContext(ctx).
		Model(&postgres.DomainEventRecord{}).
		Where("published = ?", false).
		Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("CountUnpublishedEvents: %w", result.Error)
	}
	return count, nil
}

// CountPrunableEvents returns the number of published events older than retentionDays.
func (a *EventRelayActivities) CountPrunableEvents(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	var count int64
	result := a.db.WithContext(ctx).
		Model(&postgres.DomainEventRecord{}).
		Where("published = ? AND occurred_at < ?", true, cutoff).
		Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("CountPrunableEvents(retentionDays=%d): %w", retentionDays, result.Error)
	}
	return count, nil
}

// PruneEvents deletes published events older than retentionDays in batches.
// PostgreSQL doesn't support DELETE ... LIMIT, so we use a subquery pattern.
// Returns the total number of rows deleted across all batches.
func (a *EventRelayActivities) PruneEvents(ctx context.Context, retentionDays int, batchSize int) (int64, error) {
	cutoff := time.Now().UTC().AddDate(0, 0, -retentionDays)
	var totalDeleted int64

	for {
		// PostgreSQL DELETE with subquery LIMIT pattern
		result := a.db.WithContext(ctx).Exec(
			`DELETE FROM domain_events WHERE id IN (
				SELECT id FROM domain_events
				WHERE published = ? AND occurred_at < ?
				ORDER BY occurred_at ASC
				LIMIT ?
			)`,
			true, cutoff, batchSize,
		)
		if result.Error != nil {
			return totalDeleted, fmt.Errorf("PruneEvents(retentionDays=%d, deleted=%d): %w", retentionDays, totalDeleted, result.Error)
		}

		totalDeleted += result.RowsAffected

		activity.RecordHeartbeat(ctx, fmt.Sprintf("pruned %d events so far", totalDeleted))

		// If fewer rows deleted than batchSize, we're done
		if result.RowsAffected < int64(batchSize) {
			break
		}

		// Check for context cancellation between batches
		if ctx.Err() != nil {
			return totalDeleted, ctx.Err()
		}
	}

	return totalDeleted, nil
}
