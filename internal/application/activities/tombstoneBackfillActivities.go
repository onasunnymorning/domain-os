package activities

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.temporal.io/sdk/activity"
	"gorm.io/gorm"

	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// TombstoneBackfillActivities holds dependencies for the tombstone backfill
// workflow activities that scan domain.purged events and create tombstone
// records for domains purged before the tombstone system existed.
type TombstoneBackfillActivities struct {
	db            *gorm.DB
	tombstoneRepo *postgres.GormTombstoneRepository
}

// NewTombstoneBackfillActivities creates a new TombstoneBackfillActivities with
// initialized DB and tombstone repository. Follows the same DB init pattern as
// EventRelayActivities and LifecycleActivities.
func NewTombstoneBackfillActivities() (*TombstoneBackfillActivities, error) {
	// Initialize gorm DB
	var db *gorm.DB
	var err error
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
		return nil, fmt.Errorf("failed to initialize DB for tombstone backfill activities: %w", err)
	}

	tombstoneRepo := postgres.NewGormTombstoneRepository(db)

	return &TombstoneBackfillActivities{
		db:            db,
		tombstoneRepo: tombstoneRepo,
	}, nil
}

// TombstoneBackfillBatchResult is returned by the BackfillTombstonesBatch
// activity. This type mirrors workflows.TombstoneBackfillBatchResult; it is
// duplicated here to avoid an import cycle (workflows already imports
// activities). Temporal serializes activity results as JSON, so identical
// struct shapes in both packages deserialize correctly.
type TombstoneBackfillBatchResult struct {
	EventsScanned     int64  `json:"eventsScanned"`
	TombstonesCreated int64  `json:"tombstonesCreated"`
	TombstonesSkipped int64  `json:"tombstonesSkipped"`
	Errors            int64  `json:"errors"`
	LastCursor        string `json:"lastCursor"`
}

// CountPurgeEvents returns the total number of domain.purged events in the
// domain_events table. This gives an initial size estimate before backfilling.
func (a *TombstoneBackfillActivities) CountPurgeEvents(ctx context.Context) (int64, error) {
	var count int64
	result := a.db.WithContext(ctx).
		Model(&postgres.DomainEventRecord{}).
		Where("type = ?", "domain.purged").
		Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("CountPurgeEvents: %w", result.Error)
	}
	return count, nil
}

// CountPurgeEventsWithoutTombstones returns the number of domain.purged events
// where no corresponding tombstone record exists. This is used to determine how
// much work remains after a backfill run.
func (a *TombstoneBackfillActivities) CountPurgeEventsWithoutTombstones(ctx context.Context) (int64, error) {
	var count int64
	result := a.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM domain_events e
		WHERE e.type = 'domain.purged'
		AND e.ro_id != ''
		AND NOT EXISTS (
			SELECT 1 FROM domain_tombstones t WHERE t.ro_id = e.ro_id
		)
	`).Scan(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("CountPurgeEventsWithoutTombstones: %w", result.Error)
	}
	return count, nil
}

// backfillEventData is an intermediate struct for parsing the Data JSON payload
// from domain.purged events to extract fields needed for tombstone creation.
type backfillEventData struct {
	ClientID        string `json:"ClientID"`
	TldName         string `json:"TldName"`
	DomainName      string `json:"DomainName"`
	DomainRoID      string `json:"DomainRoID"`
	TransactionType string `json:"TransactionType"`
}

// backfillBeforeState is an intermediate struct for parsing the BeforeState JSON
// payload from domain.purged events to extract domain snapshot fields.
type backfillBeforeState struct {
	ClID       string     `json:"ClID"`
	TLDName    string     `json:"TLDName"`
	CreatedAt  *time.Time `json:"CreatedAt"`
	ExpiryDate *time.Time `json:"ExpiryDate"`
	DropCatch  bool       `json:"DropCatch"`
	UName      string     `json:"UName"`
}

// BackfillTombstonesBatch processes a batch of domain.purged events and creates
// tombstone records for any that don't already have one. Events are ordered by
// ID ascending and paginated via a cursor (the last processed event ID).
//
// Uses a bulk ROID lookup to avoid N+1 queries — a single SELECT fetches all
// existing tombstone ROIDs for the batch, then creates tombstones only for
// those that are missing.
func (a *TombstoneBackfillActivities) BackfillTombstonesBatch(ctx context.Context, batchSize int, cursor string) (*TombstoneBackfillBatchResult, error) {
	result := &TombstoneBackfillBatchResult{}

	// Query domain.purged events with ROID set, ordered by ID for stable pagination
	query := a.db.WithContext(ctx).
		Where("type = ? AND ro_id != ''", "domain.purged").
		Order("id ASC").
		Limit(batchSize)

	if cursor != "" {
		query = query.Where("id > ?", cursor)
	}

	var records []postgres.DomainEventRecord
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("BackfillTombstonesBatch(cursor=%s): query failed: %w", cursor, err)
	}

	result.EventsScanned = int64(len(records))

	if len(records) == 0 {
		return result, nil
	}

	// Bulk lookup: fetch all existing tombstone ROIDs in one query
	roids := make([]string, len(records))
	for i, rec := range records {
		roids[i] = rec.RoID
	}

	var existingROIDs []string
	if err := a.db.WithContext(ctx).
		Model(&postgres.DomainTombstoneRecord{}).
		Where("ro_id IN ?", roids).
		Pluck("ro_id", &existingROIDs).Error; err != nil {
		return nil, fmt.Errorf("BackfillTombstonesBatch: bulk ROID lookup failed: %w", err)
	}

	existingSet := make(map[string]bool, len(existingROIDs))
	for _, r := range existingROIDs {
		existingSet[r] = true
	}

	activity.RecordHeartbeat(ctx, fmt.Sprintf("scanned %d events, %d already have tombstones", len(records), len(existingROIDs)))

	for i, rec := range records {
		result.LastCursor = rec.ID

		if existingSet[rec.RoID] {
			result.TombstonesSkipped++
			continue
		}

		// Tombstone not found — create one from event data
		tombstone := a.buildTombstoneFromEvent(rec)

		if _, createErr := a.tombstoneRepo.CreateTombstone(ctx, tombstone); createErr != nil {
			result.Errors++
			continue
		}

		result.TombstonesCreated++

		// Heartbeat every 50 events so Temporal knows we're alive
		if (i+1)%50 == 0 {
			activity.RecordHeartbeat(ctx, fmt.Sprintf("processed %d/%d events (created %d, skipped %d)", i+1, len(records), result.TombstonesCreated, result.TombstonesSkipped))
		}

		// Check for context cancellation between events
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
	}

	return result, nil
}

// buildTombstoneFromEvent reconstructs a DomainTombstone from a domain.purged
// event record. It extracts as much information as possible from the event's
// Data and BeforeState JSON payloads, falling back to minimal fields (name,
// roid, purged_at) when snapshot data is unavailable.
func (a *TombstoneBackfillActivities) buildTombstoneFromEvent(rec postgres.DomainEventRecord) *entities.DomainTombstone {
	tombstone := &entities.DomainTombstone{
		RoID:     entities.RoidType(rec.RoID),
		Name:     entities.DomainName(rec.Subject),
		PurgedAt: rec.OccurredAt,
	}

	// Parse Data JSON to extract lifecycle event fields
	if len(rec.Data) > 0 {
		var data backfillEventData
		if err := json.Unmarshal(rec.Data, &data); err == nil {
			if data.ClientID != "" {
				tombstone.RegistrarClID = data.ClientID
			}
			if data.TldName != "" {
				tombstone.TLDName = entities.DomainName(data.TldName)
			}
			// Derive purge reason from transaction type
			if data.TransactionType != "" {
				tombstone.PurgeReason = mapTransactionTypeToPurgeReason(data.TransactionType)
			}
		}
	}

	// Default purge reason if not set
	if tombstone.PurgeReason == "" {
		tombstone.PurgeReason = "expired"
	}

	// Parse BeforeState JSON for domain snapshot fields
	if len(rec.BeforeState) > 0 {
		var before backfillBeforeState
		if err := json.Unmarshal(rec.BeforeState, &before); err == nil {
			if before.ClID != "" {
				tombstone.RegistrarClID = before.ClID
			}
			if before.TLDName != "" {
				tombstone.TLDName = entities.DomainName(before.TLDName)
			}
			if before.CreatedAt != nil {
				tombstone.RegisteredAt = *before.CreatedAt
			}
			if before.ExpiryDate != nil {
				tombstone.ExpiredAt = before.ExpiryDate
			}
			tombstone.DropCatch = before.DropCatch
			if before.UName != "" {
				tombstone.UName = entities.DomainName(before.UName)
			}
		}

		// Store raw BeforeState as LastSnapshot for full audit trail
		var snapshot interface{}
		if err := json.Unmarshal(rec.BeforeState, &snapshot); err == nil {
			tombstone.LastSnapshot = snapshot
		}
	}

	return tombstone
}

// mapTransactionTypeToPurgeReason maps DomainLifeCycleEvent transaction types
// to human-readable purge reasons for the tombstone record.
func mapTransactionTypeToPurgeReason(txType string) string {
	switch txType {
	case "DELETE", "delete":
		return "admin_delete"
	case "PURGE", "purge":
		return "expired"
	default:
		return "expired"
	}
}
