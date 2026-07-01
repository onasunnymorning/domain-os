package postgres

import (
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// SerialObservationRecord is the GORM model for the serial_observations table.
type SerialObservationRecord struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	TenantID   string    `gorm:"not null;index:idx_serial_obs_tenant_slaving_ns_at"`
	SlavingID  uuid.UUID `gorm:"type:uuid;not null;index:idx_serial_obs_tenant_slaving_ns_at"`
	RunID      uuid.UUID `gorm:"type:uuid;not null"`
	Nameserver string    `gorm:"not null;index:idx_serial_obs_tenant_slaving_ns_at"`
	IsMaster   bool      `gorm:"not null;default:false"`
	Serial     uint32    `gorm:"not null"`
	Status     string    `gorm:"not null;default:converged"`
	DriftTier  string    `gorm:"not null;default:expected"`
	Error      string
	ObservedAt time.Time `gorm:"not null;index:idx_serial_obs_tenant_slaving_ns_at"`
}

// TableName specifies the table name for SerialObservationRecord.
func (SerialObservationRecord) TableName() string { return "serial_observations" }

// toDBSerialObservation converts a domain entity to a GORM record.
func toDBSerialObservation(o *entities.SerialObservation) *SerialObservationRecord {
	return &SerialObservationRecord{
		ID:         o.ID,
		TenantID:   o.TenantID,
		SlavingID:  o.SlavingID,
		RunID:      o.RunID,
		Nameserver: o.Nameserver,
		IsMaster:   o.IsMaster,
		Serial:     o.Serial,
		Status:     string(o.Status),
		DriftTier:  string(o.DriftTier),
		Error:      o.Error,
		ObservedAt: o.ObservedAt,
	}
}

// fromDBSerialObservation converts a GORM record to a domain entity.
func fromDBSerialObservation(r *SerialObservationRecord) *entities.SerialObservation {
	return &entities.SerialObservation{
		ID:         r.ID,
		TenantID:   r.TenantID,
		SlavingID:  r.SlavingID,
		RunID:      r.RunID,
		Nameserver: r.Nameserver,
		IsMaster:   r.IsMaster,
		Serial:     r.Serial,
		Status:     entities.SlaveStatus(r.Status),
		DriftTier:  entities.DriftTier(r.DriftTier),
		Error:      r.Error,
		ObservedAt: r.ObservedAt,
	}
}
