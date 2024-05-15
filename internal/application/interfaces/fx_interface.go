package interfaces

import (
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// FXService is the interface for managing FX
type FXService interface {
	ListByBaseCurrency(baseCurrency string) ([]*entities.FX, error)
	GetByBaseAndTargetCurrency(baseCurrency, targetCurrency string) (*entities.FX, error)
}
