package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// SerialDriftRepository defines the interface for serial drift observation storage.
type SerialDriftRepository interface {
	// ZoneSlaving CRUD
	CreateSlaving(ctx context.Context, s *entities.ZoneSlaving) error
	GetSlaving(ctx context.Context, tenantID string, id uuid.UUID) (*entities.ZoneSlaving, error)
	UpdateSlavingStatus(ctx context.Context, tenantID string, id uuid.UUID, status entities.ZoneSlavingStatus) error
	ListActiveSlavings(ctx context.Context, tenantID string) ([]*entities.ZoneSlaving, error)

	// Observation writes (append-only)
	CreateRun(ctx context.Context, run *entities.SerialCheckRun) error
	CreateObservations(ctx context.Context, obs []*entities.SerialObservation) error

	// Observation reads (cursor-paginated)
	ListObservations(ctx context.Context, tenantID string, slavingID uuid.UUID, pageSize int, cursor string) ([]*entities.SerialObservation, string, error)

	// Confidence rollup (computed from observation history)
	GetConfidenceRollup(ctx context.Context, tenantID string, slavingID uuid.UUID) (*entities.SlavingConfidenceRollup, error)

	// Recent history for drift evaluation
	GetRecentObservations(ctx context.Context, tenantID string, slavingID uuid.UUID, limit int) ([]*entities.SerialObservation, error)
	GetRecentRuns(ctx context.Context, tenantID string, slavingID uuid.UUID, limit int) ([]*entities.SerialCheckRun, error)
}
