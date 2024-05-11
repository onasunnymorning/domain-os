package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

type PremiumLabelRepository interface {
	Create(ctx context.Context, pl *entities.PremiumLabel) (*entities.PremiumLabel, error)
	GetByLabelListAndCurrency(ctx context.Context, label, list, currency string) (*entities.PremiumLabel, error)
	DeleteByLabelListAndCurrency(ctx context.Context, label, list, currency string) error
	List(ctx context.Context, pagesize int, cursor, listName, currency, label string) ([]*entities.PremiumLabel, error)
}
