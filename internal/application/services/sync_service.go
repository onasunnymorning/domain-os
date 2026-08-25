package services

import (
	"context"
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/infrastructure/api/frankfurter"
	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
	"github.com/onasunnymorning/domain-os/pkg/domain/repositories"
)

var (
	ErrRetrievingFXRates = fmt.Errorf("error retrieving FX rates")
)

// FXRatesSource fetches the latest exchange rates for a base currency.
// Implemented by frankfurter.Client; abstracted for testability.
type FXRatesSource interface {
	GetLatestRates(ctx context.Context, base string, quotes []string) ([]frankfurter.Rate, error)
}

// SyncService is a service for synchronizing data from external sources and storing it in the database
// SyncService implements the SyncService interface
type SyncService struct {
	registrarRepository repositories.IANARegistrarRepository
	Spec5Repository     repositories.Spec5LabelRepository
	IcannRepository     repositories.ICANNRepository
	IanaRepository      repositories.IANARepository
	FXRepository        repositories.FXRepository
	fxRates             FXRatesSource
}

// NewSyncService returns a new SyncService backed by the Frankfurter API for FX rates.
func NewSyncService(
	registrarRepository repositories.IANARegistrarRepository,
	spec5Repository repositories.Spec5LabelRepository,
	icannRepository repositories.ICANNRepository,
	ianaRepository repositories.IANARepository,
	fxRepository repositories.FXRepository,
) *SyncService {
	return &SyncService{
		registrarRepository: registrarRepository,
		Spec5Repository:     spec5Repository,
		IcannRepository:     icannRepository,
		IanaRepository:      ianaRepository,
		FXRepository:        fxRepository,
		fxRates:             frankfurter.NewClient(),
	}
}

// SetFXRatesSource overrides the FX rates source (used in tests).
func (s *SyncService) SetFXRatesSource(src FXRatesSource) {
	s.fxRates = src
}

// RefreshSpec5Labels deletes and recreates all Spec5Labels using the ICANN XML registry as a source
// This is only needed when ICANN updates their XML registry. This happens very infrequently.
// Use this when the system is initialized, after that only when ICANN notifies you of an update to the XML registry
func (s *SyncService) RefreshSpec5Labels(ctx context.Context) error {
	// Get the list of labels from the ICANN XML registry
	labels, err := s.IcannRepository.ListSpec5Labels()
	if err != nil {
		return err
	}
	// Replace the existing list of labels in the database with the new list
	err = s.Spec5Repository.UpdateAll(ctx, labels)
	if err != nil {
		return err
	}
	return nil
}

// RefreshIANARegistrars deletes and recreates all IANARegistrars using the IANA XML registry as a source
// This is only needed when IANA updates their XML registry. This happens not very frequently
// Use this when the system is initialized, after that only when IANA or ICANN notifies you of an update to the XML registry
// Or you receive a termination notice from ICANN for a registrar
func (s *SyncService) RefreshIANARegistrars(ctx context.Context) error {
	// Get the list of registrars from the IANA XML registry
	registrars, err := s.IanaRepository.ListRegistrars()
	if err != nil {
		return err
	}

	// Replace the existing list of registrars in the database with the new list
	err = s.registrarRepository.UpdateAll(ctx, registrars)
	if err != nil {
		return err
	}
	return nil
}

// RefreshFXRates atomically replaces all FX rates for the given base currency
// using the Frankfurter API (https://frankfurter.dev/) as the source.
func (s *SyncService) RefreshFXRates(ctx context.Context, baseCurrency string) error {
	rates, err := s.fxRates.GetLatestRates(ctx, baseCurrency, nil)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRetrievingFXRates, err)
	}

	fxs, err := frankfurterRatesToFX(rates)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrRetrievingFXRates, err)
	}

	// Replace the existing rates for this base currency with the new list
	return s.FXRepository.UpdateAll(ctx, fxs)
}

// frankfurterRatesToFX converts Frankfurter API rates to domain FX entities.
func frankfurterRatesToFX(rates []frankfurter.Rate) ([]*entities.FX, error) {
	fxs := make([]*entities.FX, 0, len(rates))
	for _, r := range rates {
		date, err := r.ParsedDate()
		if err != nil {
			return nil, fmt.Errorf("invalid rate date %q for %s/%s: %w", r.Date, r.Base, r.Quote, err)
		}
		fxs = append(fxs, &entities.FX{
			Date:           date,
			BaseCurrency:   r.Base,
			TargetCurrency: r.Quote,
			Rate:           r.Rate,
		})
	}
	return fxs, nil
}
