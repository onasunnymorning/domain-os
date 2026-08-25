package activities

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/onasunnymorning/domain-os/internal/application/serialdrift"
	dnsresolver "github.com/onasunnymorning/domain-os/internal/infrastructure/dns"

	postgres "github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"go.temporal.io/sdk/activity"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Activity struct
// ---------------------------------------------------------------------------

// SerialDriftActivities holds dependencies for the serial drift monitor
// workflow activities.
type SerialDriftActivities struct {
	db       *gorm.DB
	resolver *dnsresolver.Resolver
}

// NewSerialDriftActivities creates a new SerialDriftActivities with initialized
// DB and DNS resolver dependencies. Follows the same DB init pattern as
// EventRelayActivities.
func NewSerialDriftActivities() (*SerialDriftActivities, error) {
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
		return nil, fmt.Errorf("failed to initialize DB for serial drift activities: %w", err)
	}

	resolver := dnsresolver.NewResolver(5 * time.Second)

	return &SerialDriftActivities{
		db:       db,
		resolver: resolver,
	}, nil
}

// ---------------------------------------------------------------------------
// GORM models for drift runs and observations
// ---------------------------------------------------------------------------

// ZoneSlavingRecord represents a zone slaving configuration in the DB.
// Used only by GetSlavingConfig to read the config; the canonical entity
// lives in pkg/domain/entities.
type ZoneSlavingRecord struct {
	ID              string  `gorm:"primaryKey;column:id"`
	TenantID        string  `gorm:"column:tenant_id;index"`
	Zone            string  `gorm:"column:zone"`
	MasterNS        string  `gorm:"column:master_ns"`        // comma-separated
	SlaveNS         string  `gorm:"column:slave_ns"`          // comma-separated
	StalledAfterN   int     `gorm:"column:stalled_after_n;default:3"`
	ConfidenceN     int     `gorm:"column:confidence_n;default:5"`
	GraceMultiplier float64 `gorm:"column:grace_multiplier;default:2.0"`
}

// TableName returns the table name for ZoneSlavingRecord.
func (ZoneSlavingRecord) TableName() string {
	return "zone_slavings"
}

// DriftRunRecord represents a single workflow run's observation set.
type DriftRunRecord struct {
	ID           string    `gorm:"primaryKey;column:id"`
	TenantID     string    `gorm:"column:tenant_id;index"`
	SlavingID    string    `gorm:"column:slaving_id;index"`
	Zone         string    `gorm:"column:zone"`
	MasterSerial uint32    `gorm:"column:master_serial"`
	SOARefresh   uint32    `gorm:"column:soa_refresh"`
	SOARetry     uint32    `gorm:"column:soa_retry"`
	SOAExpire    uint32    `gorm:"column:soa_expire"`
	DriftStatus  string    `gorm:"column:drift_status"`
	Notes        string    `gorm:"column:notes"`
	CreatedAt    time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName returns the table name for DriftRunRecord.
func (DriftRunRecord) TableName() string {
	return "drift_runs"
}

// DriftObservationRecord represents a single nameserver observation within a run.
type DriftObservationRecord struct {
	ID         string    `gorm:"primaryKey;column:id"`
	RunID      string    `gorm:"column:run_id;index"`
	Nameserver string    `gorm:"column:nameserver"`
	Serial     uint32    `gorm:"column:serial"`
	IsMaster   bool      `gorm:"column:is_master"`
	Status     string    `gorm:"column:status"`
	DriftTier  string    `gorm:"column:drift_tier"`
	Error      string    `gorm:"column:error"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime"`
}

// TableName returns the table name for DriftObservationRecord.
func (DriftObservationRecord) TableName() string {
	return "drift_observations"
}

// ---------------------------------------------------------------------------
// Activities
// ---------------------------------------------------------------------------

// GetSlavingConfig retrieves the zone slaving configuration from the database.
func (a *SerialDriftActivities) GetSlavingConfig(ctx context.Context, scope entities.OperatorID, slavingID string) (*serialdrift.Config, error) {
	activity.RecordHeartbeat(ctx, "fetching slaving config")

	var record ZoneSlavingRecord
	result := a.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", slavingID, scope).
		First(&record)
	if result.Error != nil {
		return nil, fmt.Errorf("GetSlavingConfig(operator=%s, slavingID=%s): %w", scope, slavingID, result.Error)
	}

	return &serialdrift.Config{
		MasterNS:        splitCSV(record.MasterNS),
		SlaveNS:         splitCSV(record.SlaveNS),
		StalledAfterN:   record.StalledAfterN,
		ConfidenceN:     record.ConfidenceN,
		GraceMultiplier: record.GraceMultiplier,
	}, nil
}

// QuerySOASerial queries a single nameserver for the zone's SOA serial.
// Always returns (result, nil) so the workflow can handle errors in the result.
func (a *SerialDriftActivities) QuerySOASerial(ctx context.Context, zone, nameserver string) (*serialdrift.SOAQueryResult, error) {
	activity.RecordHeartbeat(ctx, fmt.Sprintf("querying SOA from %s", nameserver))

	soaResult, err := a.resolver.QuerySOA(ctx, zone, nameserver)
	if err != nil {
		// Return error in the result, not as an activity error
		return &serialdrift.SOAQueryResult{
			Nameserver: nameserver,
			Error:      err.Error(),
		}, nil
	}

	return &serialdrift.SOAQueryResult{
		Nameserver: nameserver,
		Serial:     soaResult.Serial,
		Refresh:    soaResult.Refresh,
		Retry:      soaResult.Retry,
		Expire:     soaResult.Expire,
	}, nil
}

// PersistObservations persists the drift run and its observations in a DB transaction.
func (a *SerialDriftActivities) PersistObservations(ctx context.Context, input serialdrift.PersistObservationsInput) error {
	activity.RecordHeartbeat(ctx, fmt.Sprintf("persisting %d observations", len(input.Observations)))

	return a.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Create the run record
		run := DriftRunRecord{
			ID:           input.RunID,
			TenantID:     input.TenantID.String(),
			SlavingID:    input.SlavingID,
			Zone:         input.Zone,
			MasterSerial: input.MasterSerial,
			SOARefresh:   input.SOARefresh,
			SOARetry:     input.SOARetry,
			SOAExpire:    input.SOAExpire,
			DriftStatus:  input.DriftStatus,
			Notes:        joinNotes(input.Notes),
		}
		if err := tx.Create(&run).Error; err != nil {
			return fmt.Errorf("PersistObservations: create run %s: %w", input.RunID, err)
		}

		// Create observation records
		for i, obs := range input.Observations {
			record := DriftObservationRecord{
				ID:         fmt.Sprintf("%s-%d", input.RunID, i),
				RunID:      input.RunID,
				Nameserver: obs.Nameserver,
				Serial:     obs.Serial,
				IsMaster:   obs.IsMaster,
				Status:     obs.Status,
				DriftTier:  obs.DriftTier,
				Error:      obs.Error,
			}
			if err := tx.Create(&record).Error; err != nil {
				return fmt.Errorf("PersistObservations: create observation %s: %w", record.ID, err)
			}
		}

		return nil
	})
}

// RaiseAlert is a stub that logs the alert details. In the future this could
// integrate with PagerDuty, Slack, or another alerting system.
func (a *SerialDriftActivities) RaiseAlert(ctx context.Context, input serialdrift.RaiseAlertInput) error {
	activity.RecordHeartbeat(ctx, "raising alert")
	log.Printf("ALERT [serial-drift] tenant=%s slaving=%s run=%s: %s",
		input.TenantID, input.SlavingID, input.RunID, input.Details)
	return nil
}

// GetRecentHistory retrieves recent drift observations for stall detection.
// Returns the most recent observations grouped by nameserver, ordered newest first.
func (a *SerialDriftActivities) GetRecentHistory(ctx context.Context, scope entities.OperatorID, slavingID string, limit int) ([]serialdrift.ObservationHistoryEntry, error) {
	activity.RecordHeartbeat(ctx, "fetching recent observation history")

	var entries []serialdrift.ObservationHistoryEntry

	// Join drift_observations with drift_runs to get nameserver serial history
	// ordered by run creation time (newest first), limited by the caller.
	rows, err := a.db.WithContext(ctx).Raw(`
		SELECT o.nameserver, o.serial, r.master_serial
		FROM drift_observations o
		INNER JOIN drift_runs r ON r.id = o.run_id
		WHERE r.tenant_id = ? AND r.slaving_id = ? AND o.is_master = false
		ORDER BY r.created_at DESC
		LIMIT ?
	`, scope, slavingID, limit).Rows()
	if err != nil {
		return nil, fmt.Errorf("GetRecentHistory(operator=%s, slavingID=%s): %w", scope, slavingID, err)
	}
	defer rows.Close()

	for rows.Next() {
		var entry serialdrift.ObservationHistoryEntry
		if err := rows.Scan(&entry.Nameserver, &entry.Serial, &entry.MasterSerial); err != nil {
			return nil, fmt.Errorf("GetRecentHistory: scan row: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func joinNotes(notes []string) string {
	return strings.Join(notes, "; ")
}
