package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

type TLDRepository interface {
	Create(ctx context.Context, tld *entities.TLD) error
	GetByName(ctx context.Context, name string, preloadAll bool) (*entities.TLD, error)
	List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.TLD, string, error)
	Update(ctx context.Context, tld *entities.TLD) error
	DeleteByName(ctx context.Context, name string) error
	Count(ctx context.Context, filter queries.ListTldsFilter) (int64, error)
}
