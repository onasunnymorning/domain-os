package repositories

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PriceRepository is the interface for the PriceRepository
type PriceRepository interface {
	CreatePrice(ctx context.Context, price *entities.Price) (*entities.Price, error)
	GetPrice(ctx context.Context, phaseID int64, currency string) (*entities.Price, error)
	DeletePrice(ctx context.Context, phaseID int64, currency string) error
}
