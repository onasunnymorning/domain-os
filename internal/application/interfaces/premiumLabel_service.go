package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PremiumLabelService is the interface for the PremiumLabelService
type PremiumLabelService interface {
	CreateLabel(ctx context.Context, cmd commands.CreatePremiumLabelCommand) (*entities.PremiumLabel, error)
	GetLabelByLabelListAndCurrency(ctx context.Context, label, list, currency string) (*entities.PremiumLabel, error)
	DeleteLabelByLabelListAndCurrency(ctx context.Context, label, list, currency string) error
	ListLabels(ctx context.Context, pagesize int, cursor string) ([]*entities.PremiumLabel, error)
}
