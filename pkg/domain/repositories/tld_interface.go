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
	DeleteByName(ctx context.Context, name string) error
	Count(ctx context.Context, filter queries.ListTldsFilter) (int64, error)
}
