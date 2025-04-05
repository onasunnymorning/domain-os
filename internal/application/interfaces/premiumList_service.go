package interfaces

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// PremiumListService is the interface for the PremiumListService
type PremiumListService interface {
	CreateList(ctx context.Context, cmd commands.CreatePremiumListCommand) (*entities.PremiumList, error)
	GetListByName(ctx context.Context, name string) (*entities.PremiumList, error)
	List(ctx context.Context, params queries.ListItemsQuery) ([]*entities.PremiumList, string, error)
	DeleteListByName(ctx context.Context, name string) error
}
