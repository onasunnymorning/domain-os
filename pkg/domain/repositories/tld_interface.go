package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

type TLDRepository interface {
	Create(ctx context.Context, tld *entities.TLD) error
	GetByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error)
	// GetByNameForOperator returns the TLD only if it is operated by scope; it
	// returns entities.ErrTLDNotFound otherwise so tenants cannot enumerate each
	// other's TLDs. Enforced in SQL per ADR-0006 (`WHERE name = ? AND ry_id = ?`).
	GetByNameForOperator(ctx context.Context, scope entities.OperatorID, name string) (*entities.TLD, error)
	List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.TLD, string, error)
	Update(ctx context.Context, tld *entities.TLD) error
	// DeleteByName deletes within scope; another operator's TLD reads as not found.
	DeleteByName(ctx context.Context, scope entities.RegistryScope, name string) error
	Count(ctx context.Context, filter queries.ListTldsFilter) (int64, error)
}

// TLDPhaseReader reads a TLD with its phases and their policies, and nothing
// else: no prices, fees or counts. A per-request reader (RDAP) uses it, so a
// lookup's cost does not grow with the TLD.
type TLDPhaseReader interface {
	// GetWithPhases returns the TLD and its phases, or
	// entities.ErrTLDNotFound.
	GetWithPhases(ctx context.Context, name string) (*entities.TLD, error)
}
