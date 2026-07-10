package entities

import (
	"time"

	"github.com/google/uuid"
)

// SlaveConfidence tracks the convergence confidence state for a single slave nameserver.
type SlaveConfidence struct {
	Nameserver           string
	LatestSerial         uint32
	Converged            bool
	IncrementsTracked    int  // # of master serial increments observed while this slave was tracked
	ConsecutiveConverged int  // current streak of consecutive converged checks
	ConfidenceReady      bool // true iff IncrementsTracked >= 1 AND ConsecutiveConverged >= N
	LatestStatus         SlaveStatus
	LatestDriftTier      DriftTier
}

// SlavingConfidenceRollup is the overall confidence state for a ZoneSlaving monitor.
type SlavingConfidenceRollup struct {
	SlavingID          uuid.UUID
	Zone               string
	MasterSerial       uint32
	Slaves             []SlaveConfidence
	AllConfidenceReady bool // true iff every slave is ConfidenceReady
	TotalRuns          int
	LastRunAt          time.Time
}
