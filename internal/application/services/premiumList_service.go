package services

import (
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

// PremiumListService implements the PremiumListService interface
type PremiumListService struct {
	listRepo repositories.PremiumListRepository
}

// NewPremiumListService creates a new PremiumListService
func NewPremiumListService(listRepo repositories.PremiumListRepository) *PremiumListService {
	return &PremiumListService{listRepo: listRepo}
}

// CreateList creates a new premium list
func (pls *PremiumListService) CreateList(ctx context.Context, cmd commands.CreatePremiumListCommand) (*entities.PremiumList, error) {
	// Create a new premium list
	pl, err := entities.NewPremiumList(cmd.Name, cmd.RyID)
	if err != nil {
		return nil, err
	}
	// Save the premium list
	return pls.listRepo.Create(ctx, pl)
}

// GetListByName retrieves a premium list by name
func (pls *PremiumListService) GetListByName(ctx context.Context, name string) (*entities.PremiumList, error) {
	return pls.listRepo.GetByName(ctx, name)
}

// List retrieves a list of premium lists
func (pls *PremiumListService) List(ctx context.Context, pagesize int, pagecursor string) ([]*entities.PremiumList, error) {
	return pls.listRepo.List(ctx, pagesize, pagecursor)
}

// DeleteListByName deletes a premium list by name
func (pls *PremiumListService) DeleteListByName(ctx context.Context, name string) error {
	return pls.listRepo.DeleteByName(ctx, name)
}
