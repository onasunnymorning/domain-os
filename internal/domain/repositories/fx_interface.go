package repositories

import (
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/infrastructure/db/postgres"
)

// FXRepository is the interface for the FXRepository
type FXRepository interface {
	UpdateAll(fxs []*postgres.FX) error
	ListByBaseCurrency(baseCurrency string) ([]*entities.FX, error)
	GetByBaseAndTargetCurrency(baseCurrency, targetCurrency string) (*entities.FX, error)
}
