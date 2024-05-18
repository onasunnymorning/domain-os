package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// FXService is the interface for managing FX
type FXService interface {
	ListByBaseCurrency(ctx context.Context, baseCurrency string) ([]*entities.FX, error)
	GetByBaseAndTargetCurrency(ctx context.Context, baseCurrency, targetCurrency string) (*entities.FX, error)
}
