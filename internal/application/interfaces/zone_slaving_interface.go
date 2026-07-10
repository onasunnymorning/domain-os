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
type ZoneSlavingService interface {
	// CreateSlaving creates a new ZoneSlaving monitor and starts its Temporal schedule.
	CreateSlaving(ctx context.Context, tenantID string, req CreateSlavingRequest) (*entities.ZoneSlaving, error)

	// GetSlaving retrieves a ZoneSlaving monitor by ID.
	GetSlaving(ctx context.Context, tenantID string, id uuid.UUID) (*entities.ZoneSlaving, error)

	// CompleteSlaving marks a slaving monitor as completed and deletes its schedule.
	CompleteSlaving(ctx context.Context, tenantID string, id uuid.UUID) error

	// AbandonSlaving marks a slaving monitor as abandoned and deletes its schedule.
	AbandonSlaving(ctx context.Context, tenantID string, id uuid.UUID) error

	// ListActiveSlavings lists all active slaving monitors for a tenant.
	ListActiveSlavings(ctx context.Context, tenantID string) ([]*entities.ZoneSlaving, error)

	// GetConfidenceRollup returns the current confidence state for a slaving monitor.
	GetConfidenceRollup(ctx context.Context, tenantID string, id uuid.UUID) (*entities.SlavingConfidenceRollup, error)

	// ListObservationHistory returns cursor-paginated observation history.
	ListObservationHistory(ctx context.Context, tenantID string, slavingID uuid.UUID, pageSize int, cursor string) ([]*entities.SerialObservation, string, error)
}
