package entities

import (
	"time"

	"github.com/google/uuid"
)

// SerialCheckRun represents a single execution of the serial drift check workflow.
type SerialCheckRun struct {
	ID           uuid.UUID
	TenantID     string
	SlavingID    uuid.UUID
	Zone         string
	StartedAt    time.Time
	CompletedAt  time.Time
	MasterSerial uint32
	// Live SOA timers from this run's master query.
	SOARefresh  uint32
	SOARetry    uint32
	SOAExpire   uint32
	DriftStatus DriftTier // overall: expected | warning | critical
	Notes       []string
}
