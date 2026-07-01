package postgres

import (
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// ZoneSlavingRecord is the GORM model for the zone_slavings table.
type ZoneSlavingRecord struct {
	ID                uuid.UUID      `gorm:"type:uuid;primaryKey"`
	TenantID          string         `gorm:"not null;uniqueIndex:idx_zone_slavings_tenant_zone"`
	Zone              string         `gorm:"not null;uniqueIndex:idx_zone_slavings_tenant_zone"`
	MasterNS          pq.StringArray `gorm:"type:text[];not null"`
	SlaveNS           pq.StringArray `gorm:"type:text[];not null"`
	Status            string         `gorm:"not null;index;default:active"`
	CheckIntervalSecs int64          `gorm:"not null;default:300"`
	StalledAfterN     int            `gorm:"not null;default:3"`
	ConfidenceN       int            `gorm:"not null;default:5"`
	GraceMultiplier   float64        `gorm:"not null;default:2.5"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// TableName specifies the table name for ZoneSlavingRecord.
func (ZoneSlavingRecord) TableName() string { return "zone_slavings" }

// toDBZoneSlaving converts a domain entity to a GORM record.
func toDBZoneSlaving(s *entities.ZoneSlaving) *ZoneSlavingRecord {
	return &ZoneSlavingRecord{
		ID:                s.ID,
		TenantID:          s.TenantID,
		Zone:              s.Zone,
		MasterNS:          pq.StringArray(s.MasterNS),
		SlaveNS:           pq.StringArray(s.SlaveNS),
		Status:            string(s.Status),
		CheckIntervalSecs: int64(s.CheckInterval.Seconds()),
		StalledAfterN:     s.StalledAfterN,
		ConfidenceN:       s.ConfidenceN,
		GraceMultiplier:   s.GraceMultiplier,
		CreatedAt:         s.CreatedAt,
		UpdatedAt:         s.UpdatedAt,
	}
}

// fromDBZoneSlaving converts a GORM record to a domain entity.
func fromDBZoneSlaving(r *ZoneSlavingRecord) *entities.ZoneSlaving {
	return &entities.ZoneSlaving{
		ID:              r.ID,
		TenantID:        r.TenantID,
		Zone:            r.Zone,
		MasterNS:        []string(r.MasterNS),
		SlaveNS:         []string(r.SlaveNS),
		Status:          entities.ZoneSlavingStatus(r.Status),
		CheckInterval:   time.Duration(r.CheckIntervalSecs) * time.Second,
		StalledAfterN:   r.StalledAfterN,
		ConfidenceN:     r.ConfidenceN,
		GraceMultiplier: r.GraceMultiplier,
		CreatedAt:       r.CreatedAt,
		UpdatedAt:       r.UpdatedAt,
	}
}
