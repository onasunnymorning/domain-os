package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

// EscrowDepositRepository persists immutable escrow artifact-pair records.
//
// Tenant scope follows ADR-0006: writes carry the scope inside the entity,
// reads take it as a typed parameter immediately after ctx and enforce it in
// SQL. There is deliberately no Update or Delete — a deposit record is
// immutable by absence of any mutating method.
type EscrowDepositRepository interface {
	Create(ctx context.Context, d *entities.EscrowDeposit) error
	GetByID(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.EscrowDeposit, error)
	// FindByDigests returns the deposit bound to this exact artifact pair, or
	// entities.ErrEscrowDepositNotFound.
	FindByDigests(ctx context.Context, scope entities.OperatorID, tld, rydeSHA256, sigSHA256 string) (*entities.EscrowDeposit, error)
	List(ctx context.Context, scope entities.OperatorID, q queries.ListItemsQuery) ([]*entities.EscrowDeposit, string, error)
}

// EscrowValidationRunRepository persists validation runs. A run is created
// RUNNING and finalised exactly once; Finalize must be a single conditional
// update that fails with entities.ErrEscrowValidationRunAlreadyFinal if the
// row is no longer RUNNING.
type EscrowValidationRunRepository interface {
	Create(ctx context.Context, r *entities.EscrowValidationRun) error
	Finalize(ctx context.Context, scope entities.OperatorID, r *entities.EscrowValidationRun) error
	GetByID(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.EscrowValidationRun, error)
	ListByDeposit(ctx context.Context, scope entities.OperatorID, depositID uuid.UUID) ([]*entities.EscrowValidationRun, error)
	List(ctx context.Context, scope entities.OperatorID, q queries.ListItemsQuery) ([]*entities.EscrowValidationRun, string, error)
}

// EscrowTrustedKeyRepository persists the registry signing keys trusted per
// tenant/TLD. Retire is the only state change; keys are never deleted.
type EscrowTrustedKeyRepository interface {
	Create(ctx context.Context, k *entities.EscrowTrustedKey) error
	Retire(ctx context.Context, scope entities.OperatorID, id uuid.UUID, at time.Time) error
	GetByID(ctx context.Context, scope entities.OperatorID, id uuid.UUID) (*entities.EscrowTrustedKey, error)
	// ListActive returns keys usable to verify a signature at the given instant.
	ListActive(ctx context.Context, scope entities.OperatorID, tld string, at time.Time) ([]*entities.EscrowTrustedKey, error)
	List(ctx context.Context, scope entities.OperatorID, tld string) ([]*entities.EscrowTrustedKey, error)
}
