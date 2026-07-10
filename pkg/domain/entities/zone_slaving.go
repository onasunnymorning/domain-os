package entities

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrInvalidZoneSlaving  = errors.New("invalid zone slaving configuration")
	ErrZoneSlavingNotFound = errors.New("zone slaving not found")
)

// ZoneSlavingStatus represents the lifecycle state of a zone slaving monitor.
type ZoneSlavingStatus string

const (
	ZoneSlavingStatusActive    ZoneSlavingStatus = "active"
	ZoneSlavingStatusCompleted ZoneSlavingStatus = "completed"
	ZoneSlavingStatusAbandoned ZoneSlavingStatus = "abandoned"
)

// ZoneSlaving represents a zone slaving monitor that tracks SOA serial drift
// between a master NS set and a slaving NS set during zone migration.
type ZoneSlaving struct {
	ID              uuid.UUID
	TenantID        string            // Required. RegistryOperator RyID.
	Zone            string            // e.g. "example.com"
	MasterNS        []string          // Master nameserver set
	SlaveNS         []string          // Slave nameserver set being validated
	Status          ZoneSlavingStatus
	CheckInterval   time.Duration // Polling interval (default 5m)
	StalledAfterN   int           // Consecutive checks with no slave advance -> stalled (default 3)
	ConfidenceN     int           // Consecutive converged checks required for confidence-ready (default 5)
	GraceMultiplier float64       // Warning tier multiplier on retry (default 2.5)
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewZoneSlaving creates a new ZoneSlaving with validated inputs and sensible defaults.
func NewZoneSlaving(tenantID, zone string, masterNS, slaveNS []string) (*ZoneSlaving, error) {
	if tenantID == "" {
		return nil, errors.Join(ErrInvalidZoneSlaving, errors.New("tenantID is required"))
	}
	if zone == "" {
		return nil, errors.Join(ErrInvalidZoneSlaving, errors.New("zone is required"))
	}
	if len(masterNS) == 0 {
		return nil, errors.Join(ErrInvalidZoneSlaving, errors.New("at least one master nameserver is required"))
	}
	if len(slaveNS) == 0 {
		return nil, errors.Join(ErrInvalidZoneSlaving, errors.New("at least one slave nameserver is required"))
	}

	now := RoundTime(time.Now().UTC())
	return &ZoneSlaving{
		ID:              uuid.New(),
		TenantID:        tenantID,
		Zone:            NormalizeString(zone),
		MasterNS:        masterNS,
		SlaveNS:         slaveNS,
		Status:          ZoneSlavingStatusActive,
		CheckInterval:   5 * time.Minute,
		StalledAfterN:   3,
		ConfidenceN:     5,
		GraceMultiplier: 2.5,
		CreatedAt:       now,
		UpdatedAt:       now,
	}, nil
}
