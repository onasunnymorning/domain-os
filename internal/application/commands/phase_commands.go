package commands

import (
	"time"
)

// CreatePhaseCommand is a command for creating a phase
type CreatePhaseCommand struct {
	Name    string     `json:"name" binding:"required" example:"sunrise"`
	Type    string     `json:"type" binding:"required" example:"GA"`
	Starts  time.Time  `json:"starts" binding:"required" example:"2021-01-01T00:00:00Z"`
	Ends    *time.Time `json:"ends" example:"2022-01-01T00:00:00Z"`
	TLDName string     `json:"-"`
	// TODO: Allow policy tuning
	// Policy entities.PhasePolicy `json:"policy"`
}

// EndPhaseCommand is a command for ending a phase
type EndPhaseCommand struct {
	Ends      time.Time `json:"ends" binding:"required" example:"2022-01-01T00:00:00Z"`
	TLDName   string    `json:"-"`
	PhaseName string    `json:"-"`
}
