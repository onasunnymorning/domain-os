package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/queries"
)

type PremiumLabelRepository interface {
	Create(ctx context.Context, pl *entities.PremiumLabel) (*entities.PremiumLabel, error)
	GetByLabelListAndCurrency(ctx context.Context, label, list, currency string) (*entities.PremiumLabel, error)
	DeleteByLabelListAndCurrency(ctx context.Context, label, list, currency string) error
	List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.PremiumLabel, string, error)
}
