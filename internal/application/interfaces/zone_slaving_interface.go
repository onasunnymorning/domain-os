package interfaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// CreateSlavingRequest describes the inputs needed to create a new zone slaving monitor.
type CreateSlavingRequest struct {
	Zone            string   `json:"zone" binding:"required"`
	MasterNS        []string `json:"masterNS" binding:"required,min=1"`
	SlaveNS         []string `json:"slaveNS" binding:"required,min=1"`
	CheckIntervalS  int      `json:"checkIntervalSeconds,omitempty"`  // default 300
	StalledAfterN   int      `json:"stalledAfterN,omitempty"`         // default 3
	ConfidenceN     int      `json:"confidenceN,omitempty"`           // default 5
	GraceMultiplier float64  `json:"graceMultiplier,omitempty"`       // default 2.5
}

// ZoneSlavingService defines the application-level interface for zone slaving monitor operations.
//
// Every method is operator-scoped: the scope parameter is an
// entities.OperatorID (a RegistryOperator RyID) and follows ctx immediately, as
// required by INV-02 / ADR-0006. It is typed, required, and never ambient — a
// caller cannot omit it, and cannot pass a registrar ClID in its place.
type ZoneSlavingService interface {
	// CreateSlaving creates a new ZoneSlaving monitor and starts its Temporal schedule.
	CreateSlaving(ctx context.Context, scope entities.OperatorID, req CreateSlavingRequest) (*entities.ZoneSlaving, error)

	// GetSlaving retrieves a ZoneSlaving monitor by ID.
	GetSlaving(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.ZoneSlaving, error)

	// CompleteSlaving marks a slaving monitor as completed and deletes its schedule.
	CompleteSlaving(ctx context.Context, scope entities.OperatorID, id uuid.UUID) error

	// AbandonSlaving marks a slaving monitor as abandoned and deletes its schedule.
	AbandonSlaving(ctx context.Context, scope entities.OperatorID, id uuid.UUID) error

	// ListActiveSlavings lists all active slaving monitors for an operator.
	ListActiveSlavings(ctx context.Context, scope entities.OperatorID) ([]*entities.ZoneSlaving, error)

	// GetConfidenceRollup returns the current confidence state for a slaving monitor.
	GetConfidenceRollup(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.SlavingConfidenceRollup, error)

	// ListObservationHistory returns cursor-paginated observation history.
	ListObservationHistory(ctx context.Context, scope entities.OperatorID, slavingID uuid.UUID, pageSize int, cursor string) ([]*entities.SerialObservation, string, error)
}
