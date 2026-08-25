package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// SerialDriftRepository defines the interface for serial drift observation storage.
//
// Reads and status updates are operator-scoped: scope is an
// entities.OperatorID and every implementation must enforce it in SQL
// (`WHERE tenant_id = ?`), not in the service above it. See INV-02 / ADR-0006.
type SerialDriftRepository interface {
	// ZoneSlaving CRUD
	CreateSlaving(ctx context.Context, s *entities.ZoneSlaving) error
	GetSlaving(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.ZoneSlaving, error)
	UpdateSlavingStatus(ctx context.Context, scope entities.OperatorID, id uuid.UUID, status entities.ZoneSlavingStatus) error
	ListActiveSlavings(ctx context.Context, scope entities.OperatorID) ([]*entities.ZoneSlaving, error)

	// Observation writes (append-only)
	CreateRun(ctx context.Context, run *entities.SerialCheckRun) error
	CreateObservations(ctx context.Context, obs []*entities.SerialObservation) error

	// Observation reads (cursor-paginated)
	ListObservations(ctx context.Context, scope entities.OperatorID, slavingID uuid.UUID, pageSize int, cursor string) ([]*entities.SerialObservation, string, error)

	// Confidence rollup (computed from observation history)
	GetConfidenceRollup(ctx context.Context, scope entities.OperatorID, slavingID uuid.UUID) (*entities.SlavingConfidenceRollup, error)

	// Recent history for drift evaluation
	GetRecentObservations(ctx context.Context, scope entities.OperatorID, slavingID uuid.UUID, limit int) ([]*entities.SerialObservation, error)
	GetRecentRuns(ctx context.Context, scope entities.OperatorID, slavingID uuid.UUID, limit int) ([]*entities.SerialCheckRun, error)
}
