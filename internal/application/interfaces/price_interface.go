package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PriceService is the interface for the PriceService
type PriceService interface {
	CreatePrice(ctx context.Context, cmd *commands.CreatePriceCommand) (*entities.Price, error)
	ListPrices(ctx context.Context, phaseName, TLDName string) (*entities.Price, error)
	DeletePrice(ctx context.Context, phaseID int64, currency string) error
}
