package dnsevents

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// BatchPublisher batches DNS changes and publishes at fixed intervals
// Uses database-backed queue for durability (survives crashes)
type BatchPublisher struct {
	db        *gorm.DB
	publisher *EventPublisher

	// Configuration
	batchInterval time.Duration
	maxBatchSize  int

	// Control
	stopCh  chan struct{}
	wg      sync.WaitGroup
	running bool
	mu      sync.Mutex
}

// BatchPublisherConfig holds configuration for the batch publisher
type BatchPublisherConfig struct {
	BatchInterval time.Duration // How often to flush batches (e.g., 1 minute)
	MaxBatchSize  int           // Maximum changes per batch (e.g., 10000)
}

// DefaultBatchPublisherConfig returns sensible defaults
func DefaultBatchPublisherConfig() *BatchPublisherConfig {
	return &BatchPublisherConfig{
		BatchInterval: 1 * time.Minute,
		MaxBatchSize:  10000,
	}
}

// NewBatchPublisher creates a new batch publisher with database-backed queue
func NewBatchPublisher(db *gorm.DB, config *BatchPublisherConfig) *BatchPublisher {
	if config == nil {
		config = DefaultBatchPublisherConfig()
	}

	bp := &BatchPublisher{
		db:            db,
		publisher:     NewEventPublisher(db),
		batchInterval: config.BatchInterval,
		maxBatchSize:  config.MaxBatchSize,
		stopCh:        make(chan struct{}),
	}

	return bp
}

// Start begins the background batch processing worker
func (bp *BatchPublisher) Start() error {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if bp.running {
		return fmt.Errorf("batch publisher already running")
	}

	bp.running = true
	bp.wg.Add(1)
	go bp.worker()

	log.Info().
		Dur("batch_interval", bp.batchInterval).
		Int("max_batch_size", bp.maxBatchSize).
		Msg("DNS batch publisher started")

	return nil
}

// Stop gracefully shuts down the batch publisher
func (bp *BatchPublisher) Stop() error {
	bp.mu.Lock()
	if !bp.running {
		bp.mu.Unlock()
		return nil
	}
	bp.running = false
	bp.mu.Unlock()

	close(bp.stopCh)
	bp.wg.Wait()

	// Final flush
	bp.flushAll()

	log.Info().Msg("DNS batch publisher stopped")
	return nil
}

// QueueChange adds a DNS change to the database queue
// This is synchronous (writes to DB immediately) but non-blocking for the caller
func (bp *BatchPublisher) QueueChange(ctx context.Context, change *DNSChange) error {
	// Validate change
	if err := validateChange(change); err != nil {
		return fmt.Errorf("invalid DNS change: %w", err)
	}

	// Insert into queue table
	err := bp.db.WithContext(ctx).Exec(`
		INSERT INTO dns_change_queue (
			zone_name, change_type, record_type,
			record_name, record_data, ttl,
			source_operation, domain_name
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		change.ZoneName,
		string(change.ChangeType),
		string(change.RecordType),
		change.RecordName,
		change.RecordData,
		change.TTL,
		change.SourceOperation,
		change.DomainName,
	).Error

	if err != nil {
		return fmt.Errorf("failed to queue DNS change: %w", err)
	}

	log.Debug().
		Str("zone", change.ZoneName).
		Str("record_name", change.RecordName).
		Str("operation", change.SourceOperation).
		Msg("DNS change queued")

	return nil
}

// QueueChanges adds multiple DNS changes to the queue in a single transaction
func (bp *BatchPublisher) QueueChanges(ctx context.Context, changes []*DNSChange) error {
	if len(changes) == 0 {
		return nil
	}

	return bp.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, change := range changes {
			if err := validateChange(change); err != nil {
				return fmt.Errorf("invalid DNS change: %w", err)
			}

			err := tx.Exec(`
				INSERT INTO dns_change_queue (
					zone_name, change_type, record_type,
					record_name, record_data, ttl,
					source_operation, domain_name
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				change.ZoneName,
				string(change.ChangeType),
				string(change.RecordType),
				change.RecordName,
				change.RecordData,
				change.TTL,
				change.SourceOperation,
				change.DomainName,
			).Error

			if err != nil {
				return fmt.Errorf("failed to queue DNS change: %w", err)
			}
		}

		log.Debug().
			Int("count", len(changes)).
			Msg("DNS changes queued")

		return nil
	})
}

// worker runs in background and processes batches at fixed intervals
func (bp *BatchPublisher) worker() {
	defer bp.wg.Done()

	ticker := time.NewTicker(bp.batchInterval)
	defer ticker.Stop()

	log.Info().
		Dur("interval", bp.batchInterval).
		Msg("DNS batch worker started")

	for {
		select {
		case <-ticker.C:
			bp.flushAll()

		case <-bp.stopCh:
			log.Info().Msg("DNS batch worker stopping")
			return
		}
	}
}

// flushAll processes all pending changes from all zones
func (bp *BatchPublisher) flushAll() {
	ctx := context.Background()

	// Get list of zones with pending changes
	var zones []string
	err := bp.db.WithContext(ctx).Raw(`
		SELECT DISTINCT zone_name 
		FROM dns_change_queue 
		WHERE published_at IS NULL
		ORDER BY zone_name
	`).Scan(&zones).Error

	if err != nil {
		log.Error().Err(err).Msg("Failed to get pending zones")
		return
	}

	if len(zones) == 0 {
		log.Debug().Msg("No pending DNS changes to flush")
		return
	}

	log.Info().
		Int("zones", len(zones)).
		Msg("Flushing DNS batches")

	// Process each zone
	successCount := 0
	errorCount := 0

	for _, zone := range zones {
		err := bp.flushZone(ctx, zone)
		if err != nil {
			log.Error().
				Err(err).
				Str("zone", zone).
				Msg("Failed to flush zone batch")
			errorCount++
		} else {
			successCount++
		}
	}

	log.Info().
		Int("success", successCount).
		Int("errors", errorCount).
		Msg("DNS batch flush completed")
}

// flushZone processes all pending changes for a specific zone
func (bp *BatchPublisher) flushZone(ctx context.Context, zoneName string) error {
	// Fetch pending changes for this zone (with row locking to prevent concurrent processing)
	var changes []QueuedChange
	err := bp.db.WithContext(ctx).Raw(`
		SELECT id, change_type, record_type, record_name, record_data, ttl,
		       source_operation, domain_name
		FROM dns_change_queue
		WHERE zone_name = ?
		AND published_at IS NULL
		ORDER BY queued_at, id
		LIMIT ?
		FOR UPDATE SKIP LOCKED
	`, zoneName, bp.maxBatchSize).Scan(&changes).Error

	if err != nil {
		return fmt.Errorf("failed to fetch pending changes: %w", err)
	}

	if len(changes) == 0 {
		return nil
	}

	// Collect IDs for batch update
	ids := make([]int64, len(changes))
	for i, change := range changes {
		ids[i] = change.ID
	}

	// Publish batch to DNS journal
	err = bp.publishBatch(ctx, zoneName, changes, ids)
	if err != nil {
		// Mark as error - use IN clause for GORM compatibility
		bp.db.WithContext(ctx).Exec(`
			UPDATE dns_change_queue
			SET error_count = error_count + 1,
			    last_error = ?,
			    last_error_at = NOW()
			WHERE id IN ?
		`, err.Error(), ids)

		return fmt.Errorf("failed to publish batch: %w", err)
	}

	return nil
}

// publishBatch publishes a batch of changes with a single serial increment
func (bp *BatchPublisher) publishBatch(ctx context.Context, zoneName string, changes []QueuedChange, ids []int64) error {
	batchID := time.Now().UnixNano()

	return bp.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get next serial ONCE for entire batch
		var serial int64
		err := tx.Raw("SELECT get_next_serial(?)", zoneName).Scan(&serial).Error
		if err != nil {
			return fmt.Errorf("failed to get next serial: %w", err)
		}

		// Insert all changes into dns_zone_journal with the SAME serial
		for _, change := range changes {
			err = tx.Exec(`
				INSERT INTO dns_zone_journal (
					zone_name, serial, change_type, record_type,
					record_name, record_data, ttl,
					source_operation, domain_name
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				zoneName,
				serial, // ← Same serial for ALL changes in this batch
				change.ChangeType,
				change.RecordType,
				change.RecordName,
				change.RecordData,
				change.TTL,
				change.SourceOperation,
				change.DomainName,
			).Error

			if err != nil {
				return fmt.Errorf("failed to insert journal entry: %w", err)
			}
		}

		// Mark queue items as published
		// Use IN clause instead of ANY for GORM compatibility
		err = tx.Exec(`
			UPDATE dns_change_queue
			SET published_at = NOW(),
			    batch_id = ?
			WHERE id IN ?
		`, batchID, ids).Error

		if err != nil {
			return fmt.Errorf("failed to mark items as published: %w", err)
		}

		log.Info().
			Str("zone", zoneName).
			Int64("serial", serial).
			Int("batch_size", len(changes)).
			Int64("batch_id", batchID).
			Msg("DNS batch published")

		return nil
	})
}

// GetQueueStats returns statistics about the queue
func (bp *BatchPublisher) GetQueueStats(ctx context.Context) ([]QueueStats, error) {
	var stats []QueueStats
	err := bp.db.WithContext(ctx).Raw(`
		SELECT 
			zone_name,
			pending_count,
			published_count,
			error_count,
			oldest_pending,
			newest_pending,
			avg_wait_seconds
		FROM dns_queue_stats
		ORDER BY pending_count DESC
	`).Scan(&stats).Error

	return stats, err
}

// QueueStats represents queue statistics for a zone
type QueueStats struct {
	ZoneName       string     `json:"zone_name"`
	PendingCount   int64      `json:"pending_count"`
	PublishedCount int64      `json:"published_count"`
	ErrorCount     int64      `json:"error_count"`
	OldestPending  *time.Time `json:"oldest_pending"`
	NewestPending  *time.Time `json:"newest_pending"`
	AvgWaitSeconds *float64   `json:"avg_wait_seconds"`
}

// CleanupPublished removes old published queue items
func (bp *BatchPublisher) CleanupPublished(ctx context.Context, retentionDays int) (int64, error) {
	var deletedCount int64
	err := bp.db.WithContext(ctx).Raw(
		"SELECT cleanup_dns_queue(?)",
		retentionDays,
	).Scan(&deletedCount).Error

	if err != nil {
		return 0, fmt.Errorf("failed to cleanup queue: %w", err)
	}

	log.Info().
		Int64("deleted", deletedCount).
		Int("retention_days", retentionDays).
		Msg("DNS queue cleanup completed")

	return deletedCount, nil
}

// QueuedChange represents a DNS change from the queue
type QueuedChange struct {
	ID              int64
	ChangeType      string
	RecordType      string
	RecordName      string
	RecordData      string
	TTL             uint32
	SourceOperation string
	DomainName      string
}

// GetDB returns the underlying database connection
func (bp *BatchPublisher) GetDB() *gorm.DB {
	return bp.db
}

// validateChange validates a DNS change
func validateChange(change *DNSChange) error {
	if change.ZoneName == "" {
		return fmt.Errorf("zone name is required")
	}
	if change.RecordName == "" {
		return fmt.Errorf("record name is required")
	}
	if change.RecordData == "" {
		return fmt.Errorf("record data is required")
	}
	if change.TTL == 0 {
		change.TTL = 3600 // Default TTL
	}
	return nil
}
