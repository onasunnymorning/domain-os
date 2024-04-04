package commands

import (
	"time"
)

// CreatePhaseCommand is a command for creating a phase
type CreatePhaseCommand struct {
	Name    string     `json:"name" binding:"required"`
	Type    string     `json:"type" binding:"required"`
	Starts  time.Time  `json:"starts" binding:"required"`
	Ends    *time.Time `json:"ends"`
	TLDName string
	// TODO: Allow policy tuning
	// Policy entities.PhasePolicy `json:"policy"`
}

// EndPhaseCommand is a command for ending a phase
type EndPhaseCommand struct {
	Ends      time.Time `json:"ends" binding:"required"`
	TLDName   string
	PhaseName string
}
