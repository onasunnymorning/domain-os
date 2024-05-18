package services

import (
	"context"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// FXService implements the FXService interface
type FXService struct {
	fxRepo repositories.FXRepository
}

// NewFXService returns a new FXService
func NewFXService(fxRepo repositories.FXRepository) *FXService {
	return &FXService{
		fxRepo: fxRepo,
	}
}

// ListByBaseCurrency lists all exchange rates by base currency
func (s *FXService) ListByBaseCurrency(ctx context.Context, baseCurrency string) ([]*entities.FX, error) {
	return s.fxRepo.ListByBaseCurrency(ctx, baseCurrency)
}

// GetByBaseAndTargetCurrency gets the exchange rate for a base and target currency
func (s *FXService) GetByBaseAndTargetCurrency(ctx context.Context, baseCurrency, targetCurrency string) (*entities.FX, error) {
	return s.fxRepo.GetByBaseAndTargetCurrency(ctx, baseCurrency, targetCurrency)
}
