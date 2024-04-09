package services

import (
	"context"
	"errors"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// PriceService is the implementation of the PriceService interface
type PriceService struct {
	phaseRepo repositories.PhaseRepository
	priceRepo repositories.PriceRepository
}

// NewPriceService returns a new instance of PriceService
func NewPriceService(phaseRepo repositories.PhaseRepository, priceRepo repositories.PriceRepository) *PriceService {
	return &PriceService{
		phaseRepo: phaseRepo,
		priceRepo: priceRepo,
	}
}

// CreatePrice creates a new price
func (s *PriceService) CreatePrice(ctx context.Context, cmd *commands.CreatePriceCommand) (*entities.Price, error) {
	// retrieve the phase
	phase, err := s.phaseRepo.GetPhaseByTLDAndName(ctx, cmd.TLDName, cmd.PhaseName)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidPrice, err)
	}
	// create a new price
	price, err := entities.NewPrice(cmd.Currency, cmd.RegistrationAmount, cmd.RenewalAmount, cmd.TransferAmount, cmd.RestoreAmount)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidPrice, err)
	}

	// add the price to the phase using our domain logic
	_, err = phase.AddPrice(*price)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidPrice, err)
	}
	// set the phase ID
	price.PhaseID = phase.ID

	// if there are no errors, save the price to the database
	dbPrice, err := s.priceRepo.CreatePrice(ctx, price)
	if err != nil {
		return nil, err
	}

	return dbPrice, nil
}

// ListPrices lists all prices for a given phase
func (s *PriceService) ListPrices(ctx context.Context, phaseName, TLDName string) ([]entities.Price, error) {
	// retrieve the phase including the fees
	phase, err := s.phaseRepo.GetPhaseByTLDAndName(ctx, TLDName, phaseName)
	if err != nil {
		return nil, err
	}

	// copy the prices to a new slice
	prices := make([]entities.Price, len(phase.Prices))
	copy(prices, phase.Prices)

	return prices, nil
}

// DeletePrice deletes a price
func (s *PriceService) DeletePrice(ctx context.Context, phaseName, TLDName, currency string) error {
	// retrieve the phase
	phase, err := s.phaseRepo.GetPhaseByTLDAndName(ctx, TLDName, phaseName)
	if err != nil {
		return err
	}

	// use our domain logic to delete the price
	err = phase.DeletePrice(currency)
	if err != nil {
		return err
	}

	// if there are no errors, delete the price from the repository, making sure the currency code is uppercase as we always store it in uppercase
	return s.priceRepo.DeletePrice(ctx, phase.ID, strings.ToUpper(currency))
}
