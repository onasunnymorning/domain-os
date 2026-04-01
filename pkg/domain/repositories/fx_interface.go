package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// FXRepository is the interface for the FXRepository
type FXRepository interface {
	UpdateAll(ctx context.Context, fxs []*postgres.FX) error
	ListByBaseCurrency(ctx context.Context, baseCurrency string) ([]*entities.FX, error)
	GetByBaseAndTargetCurrency(ctx context.Context, baseCurrency, targetCurrency string) (*entities.FX, error)
}
