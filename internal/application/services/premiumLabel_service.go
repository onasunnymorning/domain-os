package services

import (
	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

// PremiumListService implements the PremiumListService interface
type PremiumLabelService struct {
	labelRepo repositories.PremiumLabelRepository
}

// NewPremiumLabelService creates a new PremiumListService
func NewPremiumLabelService(labelrepo repositories.PremiumLabelRepository) *PremiumLabelService {
	return &PremiumLabelService{labelRepo: labelrepo}
}

// CreateList creates a new premium label
func (pls *PremiumLabelService) CreateLabel(ctx context.Context, cmd commands.CreatePremiumLabelCommand) (*entities.PremiumLabel, error) {
	// Create a new premium label
	pl, err := entities.NewPremiumLabel(cmd.Label, cmd.RegistrationAmount, cmd.RenewalAmount, cmd.TransferAmount, cmd.RestoreAmount, cmd.Currency, cmd.Class, cmd.PremiumListName)
	if err != nil {
		return nil, err
	}
	// Save the premium label
	return pls.labelRepo.Create(ctx, pl)
}

// GetLabelByLabelListAndCurrency retrieves a premium label by label, list, and currency
func (pls *PremiumLabelService) GetLabelByLabelListAndCurrency(ctx context.Context, label, list, currency string) (*entities.PremiumLabel, error) {
	return pls.labelRepo.GetByLabelListAndCurrency(ctx, label, list, currency)
}

// DeleteLabelByLabelListAndCurrency deletes a premium label by label, list, and currency
func (pls *PremiumLabelService) DeleteLabelByLabelListAndCurrency(ctx context.Context, label, list, currency string) error {
	return pls.labelRepo.DeleteByLabelListAndCurrency(ctx, label, list, currency)
}

// ListLabels retrieves a list of premium labels
func (pls *PremiumLabelService) ListLabels(ctx context.Context, params queries.ListItemsQuery) ([]*entities.PremiumLabel, string, error) {
	return pls.labelRepo.List(ctx, params)
}
