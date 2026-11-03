package commands

import (
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// CreatePhaseCommand is a command for creating a phase
type CreatePhaseCommand struct {
	Name    string     `json:"name" binding:"required" example:"sunrise"`
	Type    string     `json:"type" binding:"required" example:"GA"`
	Starts  time.Time  `json:"starts" binding:"required" example:"2021-01-01T00:00:00Z"`
	Ends    *time.Time `json:"ends" example:"2022-01-01T00:00:00Z"`
	TLDName string     `json:"-"`
	// Launch sets a launch phase's launch settings at creation (ADR 0024);
	// omitted, a launch phase starts open and first come. Refused on a GA
	// phase.
	Launch *entities.LaunchSettings `json:"launch,omitempty"`
	// TODO: Allow policy tuning
	// Policy entities.PhasePolicy `json:"policy"`
}

// SetLaunchSettingsCommand replaces a launch phase's launch settings
// (PUT /tlds/{tld}/phases/{phase}/launch).
type SetLaunchSettingsCommand struct {
	entities.LaunchSettings
	TLDName   string `json:"-"`
	PhaseName string `json:"-"`
}

// UpdatePhasePolicyCommand is a command for updating a phase
type UpdatePhasePolicyCommand struct {
	Policy *entities.PhasePolicy
}

// EndPhaseCommand is a command for ending a phase
type EndPhaseCommand struct {
	Ends      time.Time `json:"ends" binding:"required" example:"2022-01-01T00:00:00Z"`
	TLDName   string    `json:"-"`
	PhaseName string    `json:"-"`
}
