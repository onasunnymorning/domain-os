package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

type PremiumListRepository interface {
	Create(ctx context.Context, pl *entities.PremiumList) (*entities.PremiumList, error)
	GetByName(ctx context.Context, name string) (*entities.PremiumList, error)
	DeleteByName(ctx context.Context, name string) error
	List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.PremiumList, string, error)
}
