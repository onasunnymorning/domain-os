package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PhasePriceRepository is the interface for the PhasePriceRepository
type PhasePriceRepository interface {
	CreatePhasePrice(ctx context.Context, price *entities.Price) (*entities.Price, error)
	GetPhasePrice(ctx context.Context, phase, currency string) (*entities.Price, error)
	DeletePhasePrice(ctx context.Context, phase, currency string) error
}
