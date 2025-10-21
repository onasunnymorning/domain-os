package dnsevents

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// DNSChangeType represents the type of DNS change
type DNSChangeType string

const (
	DNSChangeTypeAdd    DNSChangeType = "ADD"
	DNSChangeTypeDelete DNSChangeType = "DELETE"
)

// DNSRecordType represents DNS record types
type DNSRecordType string

const (
	DNSRecordTypeNS   DNSRecordType = "NS"
	DNSRecordTypeA    DNSRecordType = "A"
	DNSRecordTypeAAAA DNSRecordType = "AAAA"
)

// DNSChange represents a single DNS record change
type DNSChange struct {
	ZoneName        string
	ChangeType      DNSChangeType
	RecordType      DNSRecordType
	RecordName      string // FQDN with trailing dot (e.g., "example.tld.")
	RecordData      string // Value (e.g., "ns1.example.com.")
	TTL             uint32
	SourceOperation string // e.g., "CreateDomain", "AddHost"
	DomainName      string // Which domain triggered this (for debugging)
}

// EventPublisher publishes DNS zone changes to the journal
type EventPublisher struct {
	db *gorm.DB
}

// NewEventPublisher creates a new DNS event publisher
func NewEventPublisher(db *gorm.DB) *EventPublisher {
	return &EventPublisher{db: db}
}

// PublishChange publishes a DNS change event
// Must be called within an existing database transaction
func (ep *EventPublisher) PublishChange(ctx context.Context, tx *gorm.DB, change *DNSChange) error {
	// Validate change
	if err := ep.validateChange(change); err != nil {
		return fmt.Errorf("invalid DNS change: %w", err)
	}

	// Get next serial for this zone
	var serial int64
	err := tx.WithContext(ctx).Raw(
		"SELECT get_next_serial(?)",
		change.ZoneName,
	).Scan(&serial).Error
	if err != nil {
		return fmt.Errorf("failed to get next serial: %w", err)
	}

	// Insert journal entry
	err = tx.WithContext(ctx).Exec(`
		INSERT INTO dns_zone_journal (
			zone_name, serial, change_type, record_type,
			record_name, record_data, ttl,
			source_operation, domain_name
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		change.ZoneName,
		serial,
		string(change.ChangeType),
		string(change.RecordType),
		change.RecordName,
		change.RecordData,
		change.TTL,
		change.SourceOperation,
		change.DomainName,
	).Error

	if err != nil {
		return fmt.Errorf("failed to insert journal entry: %w", err)
	}

	log.Info().
		Str("zone", change.ZoneName).
		Int64("serial", serial).
		Str("change_type", string(change.ChangeType)).
		Str("record_type", string(change.RecordType)).
		Str("record_name", change.RecordName).
		Str("record_data", change.RecordData).
		Str("operation", change.SourceOperation).
		Msg("DNS change published")

	return nil
}

// PublishChanges publishes multiple DNS changes in a batch
// All changes increment the serial and are logged atomically
func (ep *EventPublisher) PublishChanges(ctx context.Context, tx *gorm.DB, changes []*DNSChange) error {
	for _, change := range changes {
		if err := ep.PublishChange(ctx, tx, change); err != nil {
			return err
		}
	}
	return nil
}

// validateChange validates a DNS change
func (ep *EventPublisher) validateChange(change *DNSChange) error {
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

// GetCurrentSerial returns the current serial for a zone without incrementing
func (ep *EventPublisher) GetCurrentSerial(ctx context.Context, zoneName string) (int64, error) {
	var serial int64
	err := ep.db.WithContext(ctx).Raw(
		"SELECT get_current_serial(?)",
		zoneName,
	).Scan(&serial).Error
	if err != nil {
		return 0, fmt.Errorf("failed to get current serial: %w", err)
	}
	return serial, nil
}

// CleanupJournal removes old journal entries
func (ep *EventPublisher) CleanupJournal(ctx context.Context, keepCount int) error {
	if keepCount <= 0 {
		keepCount = 100 // Default: keep last 100 serials
	}

	rows, err := ep.db.WithContext(ctx).Raw(
		"SELECT * FROM cleanup_dns_journal(?)",
		keepCount,
	).Rows()
	if err != nil {
		return fmt.Errorf("failed to cleanup journal: %w", err)
	}
	defer rows.Close()

	totalDeleted := int64(0)
	for rows.Next() {
		var zoneName string
		var deletedCount int64
		if err := rows.Scan(&zoneName, &deletedCount); err != nil {
			continue
		}
		totalDeleted += deletedCount
		log.Info().
			Str("zone", zoneName).
			Int64("deleted", deletedCount).
			Msg("DNS journal cleaned up")
	}

	log.Info().
		Int64("total_deleted", totalDeleted).
		Int("keep_count", keepCount).
		Msg("DNS journal cleanup completed")

	return nil
}

// GetZoneStatus returns status information for all zones
func (ep *EventPublisher) GetZoneStatus(ctx context.Context) ([]ZoneStatus, error) {
	var statuses []ZoneStatus
	err := ep.db.WithContext(ctx).Raw("SELECT * FROM dns_zone_status ORDER BY zone_name").Scan(&statuses).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get zone status: %w", err)
	}
	return statuses, nil
}

// ZoneStatus represents the status of a DNS zone
type ZoneStatus struct {
	ZoneName          string       `json:"zone_name"`
	CurrentSerial     int64        `json:"current_serial"`
	LastUpdated       time.Time    `json:"last_updated"`
	LastNotifyAt      sql.NullTime `json:"last_notify_at"`
	NotifyCount       int          `json:"notify_count"`
	ChangesInLastHour int          `json:"changes_in_last_hour"`
}

// Helper functions for common DNS change patterns

// PublishDomainNSRecords publishes NS record changes for a domain
func (ep *EventPublisher) PublishDomainNSRecords(
	ctx context.Context,
	tx *gorm.DB,
	zoneName string,
	domainName string,
	hostNames []string,
	changeType DNSChangeType,
	sourceOperation string,
) error {
	changes := make([]*DNSChange, 0, len(hostNames))

	for _, hostName := range hostNames {
		changes = append(changes, &DNSChange{
			ZoneName:        zoneName,
			ChangeType:      changeType,
			RecordType:      DNSRecordTypeNS,
			RecordName:      ensureTrailingDot(domainName),
			RecordData:      ensureTrailingDot(hostName),
			TTL:             3600,
			SourceOperation: sourceOperation,
			DomainName:      domainName,
		})
	}

	return ep.PublishChanges(ctx, tx, changes)
}

// PublishGlueRecords publishes A/AAAA glue records for a host
func (ep *EventPublisher) PublishGlueRecords(
	ctx context.Context,
	tx *gorm.DB,
	zoneName string,
	hostName string,
	addresses map[string]int, // address -> IP version (4 or 6)
	changeType DNSChangeType,
	sourceOperation string,
) error {
	changes := make([]*DNSChange, 0, len(addresses))

	for address, version := range addresses {
		recordType := DNSRecordTypeA
		if version == 6 {
			recordType = DNSRecordTypeAAAA
		}

		changes = append(changes, &DNSChange{
			ZoneName:        zoneName,
			ChangeType:      changeType,
			RecordType:      recordType,
			RecordName:      ensureTrailingDot(hostName),
			RecordData:      address,
			TTL:             3600,
			SourceOperation: sourceOperation,
			DomainName:      "", // Glue records don't have associated domain
		})
	}

	return ep.PublishChanges(ctx, tx, changes)
}

// ensureTrailingDot ensures a DNS name has a trailing dot
func ensureTrailingDot(name string) string {
	if name == "" {
		return ""
	}
	if name[len(name)-1] != '.' {
		return name + "."
	}
	return name
}
