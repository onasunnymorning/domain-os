package commands

import (
	"time"
)

// CreatePhaseCommand is a command for creating a phase
type CreatePhaseCommand struct {
	Name    string     `json:"name"`
	Type    string     `json:"type"`
	Starts  time.Time  `json:"starts"`
	Ends    *time.Time `json:"ends"`
	TLDName string
	// TODO: Allow policy tuning
	// Policy entities.PhasePolicy `json:"policy"`
}
