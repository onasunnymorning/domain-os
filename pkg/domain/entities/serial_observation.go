package entities

import (
	"time"

	"github.com/google/uuid"
)

// SlaveStatus describes the per-check status of a single slave nameserver.
type SlaveStatus string

const (
	SlaveStatusConverged   SlaveStatus = "converged"
	SlaveStatusLagging     SlaveStatus = "lagging"
	SlaveStatusStalled     SlaveStatus = "stalled"
	SlaveStatusUnreachable SlaveStatus = "unreachable"
)

// DriftTier describes the severity of serial drift.
type DriftTier string

const (
	DriftTierExpected DriftTier = "expected"
	DriftTierWarning  DriftTier = "warning"
	DriftTierCritical DriftTier = "critical"
)

// SerialObservation is an append-only record of a single nameserver's SOA serial
// at a point in time, produced by a SerialCheckRun.
type SerialObservation struct {
	ID         uuid.UUID
	TenantID   string
	SlavingID  uuid.UUID
	RunID      uuid.UUID
	Nameserver string
	IsMaster   bool
	Serial     uint32
	Status     SlaveStatus
	DriftTier  DriftTier
	Error      string // non-empty if unreachable
	ObservedAt time.Time
}
