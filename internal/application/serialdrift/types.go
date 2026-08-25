// Package serialdrift defines the shared serializable types used by both
// the serial drift workflow and its activities. These types are extracted
// to their own package to avoid import cycles between workflows and activities.
package serialdrift

import "github.com/onasunnymorning/domain-os/pkg/domain/entities"

// Params is the input to CheckSerialDriftWorkflow.
// When SlavingID is set, the workflow loads config from the DB record.
// When SlavingID is empty (ad-hoc run), the workflow uses the inline
// MasterNS/SlaveNS fields instead.
type Params struct {
	// TenantID is the operator tenant scope for this run — a RegistryOperator
	// RyID. Typed per ADR-0006; it serializes as a plain JSON string, so
	// existing Temporal schedules and histories are unaffected.
	TenantID  entities.OperatorID `json:"tenantId"`
	SlavingID string              `json:"slavingId"`
	Zone      string              `json:"zone"`

	// Inline config — used when SlavingID is empty (ad-hoc runs from UI).
	MasterNS        []string `json:"masterNS,omitempty"`
	SlaveNS         []string `json:"slaveNS,omitempty"`
	StalledAfterN   int      `json:"stalledAfterN,omitempty"`   // default 3
	ConfidenceN     int      `json:"confidenceN,omitempty"`     // default 5
	GraceMultiplier float64  `json:"graceMultiplier,omitempty"` // default 2.5
}

// Result is the output of CheckSerialDriftWorkflow.
type Result struct {
	RunID        string           `json:"runId"`
	StartedAt    string           `json:"startedAt"`
	CompletedAt  string           `json:"completedAt"`
	MasterSerial uint32           `json:"masterSerial"`
	SOARefresh   uint32           `json:"soaRefresh"`
	SOARetry     uint32           `json:"soaRetry"`
	SOAExpire    uint32           `json:"soaExpire"`
	Observations []ObservationRef `json:"observations"`
	DriftStatus  string           `json:"driftStatus"`
	Notes        []string         `json:"notes"`
}

// ObservationRef is a lightweight reference to a persisted observation.
type ObservationRef struct {
	ID         string `json:"id"`
	Nameserver string `json:"nameserver"`
	Serial     uint32 `json:"serial"`
	Status     string `json:"status"`
}

// Config holds the ZoneSlaving configuration needed by the workflow.
type Config struct {
	MasterNS        []string `json:"masterNS"`
	SlaveNS         []string `json:"slaveNS"`
	StalledAfterN   int      `json:"stalledAfterN"`
	ConfidenceN     int      `json:"confidenceN"`
	GraceMultiplier float64  `json:"graceMultiplier"`
}

// SOAQueryResult holds the result of a single DNS SOA query.
type SOAQueryResult struct {
	Nameserver string `json:"nameserver"`
	Serial     uint32 `json:"serial"`
	Refresh    uint32 `json:"refresh"`
	Retry      uint32 `json:"retry"`
	Expire     uint32 `json:"expire"`
	Error      string `json:"error,omitempty"`
}

// ObservationHistoryEntry is a single historical observation for stall detection.
type ObservationHistoryEntry struct {
	Nameserver   string `json:"nameserver"`
	Serial       uint32 `json:"serial"`
	MasterSerial uint32 `json:"masterSerial"`
}

// ObservationResult is the per-nameserver evaluation outcome.
type ObservationResult struct {
	Nameserver string `json:"nameserver"`
	Serial     uint32 `json:"serial"`
	IsMaster   bool   `json:"isMaster"`
	Status     string `json:"status"`
	DriftTier  string `json:"driftTier"`
	Error      string `json:"error,omitempty"`
}

// PersistObservationsInput is the input to the PersistObservations activity.
type PersistObservationsInput struct {
	TenantID     entities.OperatorID `json:"tenantId"`
	SlavingID    string              `json:"slavingId"`
	RunID        string              `json:"runId"`
	Zone         string              `json:"zone"`
	MasterSerial uint32              `json:"masterSerial"`
	SOARefresh   uint32              `json:"soaRefresh"`
	SOARetry     uint32              `json:"soaRetry"`
	SOAExpire    uint32              `json:"soaExpire"`
	DriftStatus  string              `json:"driftStatus"`
	Notes        []string            `json:"notes"`
	Observations []ObservationResult `json:"observations"`
}

// RaiseAlertInput is the input to the RaiseAlert activity.
type RaiseAlertInput struct {
	TenantID  entities.OperatorID `json:"tenantId"`
	SlavingID string              `json:"slavingId"`
	RunID     string              `json:"runId"`
	Details   string              `json:"details"`
}
