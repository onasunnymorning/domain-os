package services

import (
	"errors"
	"log"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/commands"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
	"golang.org/x/net/context"
)

// FeeService implements the FeeService interface
type FeeService struct {
	phaseRepo repositories.PhaseRepository
	feeRepo   repositories.FeeRepository
}

// NewFeeService returns a new instance of FeeService
func NewFeeService(phaseRepo repositories.PhaseRepository, feeRepo repositories.FeeRepository) *FeeService {
	return &FeeService{
		phaseRepo: phaseRepo,
		feeRepo:   feeRepo,
	}
}

// CreateFee creates a new fee
func (svc *FeeService) CreateFee(ctx context.Context, cmd *commands.CreateFeeCommand) (*entities.Fee, error) {
	// Retrieve the phase
	phase, err := svc.phaseRepo.GetPhaseByTLDAndName(ctx, cmd.TLDName, cmd.PhaseName)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidFee, err)
	}

	// Create a new fee
	fee, err := entities.NewFee(cmd.Currency, cmd.Name, cmd.Amount, &cmd.Refundable)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidFee, err)
	}

	// Add the fee to the phase using our domain logic
	_, err = phase.AddFee(*fee)
	if err != nil {
		return nil, errors.Join(entities.ErrInvalidFee, err)
	}

	// Set the phase ID
	fee.PhaseID = phase.ID

	// If there are no errors, save the fee to the database
	dbFee, err := svc.feeRepo.CreateFee(ctx, fee)
	if err != nil {
		return nil, err
	}

	return dbFee, nil
}

// ListFees lists all fees for a given phase
func (svc *FeeService) ListFees(ctx context.Context, phaseName, TLDName string) ([]entities.Fee, error) {
	// Retrieve the phase including the fees
	phase, err := svc.phaseRepo.GetPhaseByTLDAndName(ctx, TLDName, phaseName)
	if err != nil {
		return nil, err
	}

	// Copy the fees from phase.Fees to response slice
	response := make([]entities.Fee, len(phase.Fees))
	copy(response, phase.Fees)

	return response, nil
}

// DeleteFee deletes a fee
func (svc *FeeService) DeleteFee(ctx context.Context, phaseName, TLDName, feeName, currency string) error {
	// Retrieve the phase
	phase, err := svc.phaseRepo.GetPhaseByTLDAndName(ctx, TLDName, phaseName)
	if err != nil {
		return err
	}

	// Use our domain logic to delete the fee
	err = phase.DeleteFee(feeName, currency)
	if err != nil {
		return err
	}

	log.Printf("Deleting fee %s %s from phase %s %s", feeName, currency, TLDName, phaseName)
	// If there are no errors, delete the fee from the repository
	err = svc.feeRepo.DeleteFee(ctx, phase.ID, feeName, strings.ToUpper(currency))
	if err != nil {
		return err
	}
	return nil
}
