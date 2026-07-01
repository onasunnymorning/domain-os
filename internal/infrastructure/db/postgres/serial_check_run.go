package postgres

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// SerialCheckRunRecord is the GORM model for the serial_check_runs table.
type SerialCheckRunRecord struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey"`
	TenantID     string         `gorm:"not null;index"`
	SlavingID    uuid.UUID      `gorm:"type:uuid;not null;index"`
	Zone         string         `gorm:"not null"`
	StartedAt    time.Time      `gorm:"not null"`
	CompletedAt  time.Time
	MasterSerial uint32         `gorm:"not null"`
	SOARefresh   uint32
	SOARetry     uint32
	SOAExpire    uint32
	DriftStatus  string         `gorm:"not null;default:expected"`
	Notes        pq.StringArray `gorm:"type:text[]"`
}

// TableName specifies the table name for SerialCheckRunRecord.
func (SerialCheckRunRecord) TableName() string { return "serial_check_runs" }

// toDBSerialCheckRun converts a domain entity to a GORM record.
func toDBSerialCheckRun(r *entities.SerialCheckRun) *SerialCheckRunRecord {
	return &SerialCheckRunRecord{
		ID:           r.ID,
		TenantID:     r.TenantID,
		SlavingID:    r.SlavingID,
		Zone:         r.Zone,
		StartedAt:    r.StartedAt,
		CompletedAt:  r.CompletedAt,
		MasterSerial: r.MasterSerial,
		SOARefresh:   r.SOARefresh,
		SOARetry:     r.SOARetry,
		SOAExpire:    r.SOAExpire,
		DriftStatus:  string(r.DriftStatus),
		Notes:        pq.StringArray(r.Notes),
	}
}

// fromDBSerialCheckRun converts a GORM record to a domain entity.
func fromDBSerialCheckRun(r *SerialCheckRunRecord) *entities.SerialCheckRun {
	return &entities.SerialCheckRun{
		ID:           r.ID,
		TenantID:     r.TenantID,
		SlavingID:    r.SlavingID,
		Zone:         r.Zone,
		StartedAt:    r.StartedAt,
		CompletedAt:  r.CompletedAt,
		MasterSerial: r.MasterSerial,
		SOARefresh:   r.SOARefresh,
		SOARetry:     r.SOARetry,
		SOAExpire:    r.SOAExpire,
		DriftStatus:  entities.DriftTier(r.DriftStatus),
		Notes:        []string(r.Notes),
	}
}
