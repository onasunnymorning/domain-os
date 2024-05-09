package services

import (
	"context"
	"errors"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// DomainCheckService is the implementation of the DomainCheckService interface
type DomainCheckService struct {
	domainRepo       repositories.DomainRepository
	tldRepo          repositories.TLDRepository
	phaseRepo        repositories.PhaseRepository
	premiumLabelRepo repositories.PremiumLabelRepository
}

// NewDomainCheckService returns a new instance of DomainCheckService
func NewDomainCheckService(domainRepo repositories.DomainRepository, tldRepo repositories.TLDRepository, premiumLabelRepo repositories.PremiumLabelRepository, phaseRepo repositories.PhaseRepository) *DomainCheckService {
	return &DomainCheckService{
		domainRepo:       domainRepo,
		tldRepo:          tldRepo,
		phaseRepo:        phaseRepo,
		premiumLabelRepo: premiumLabelRepo,
	}
}

// CheckDomain checks the availability of a domain name
func (svc *DomainCheckService) CheckDomain(ctx context.Context, q *queries.DomainCheckQuery) (*queries.DomainCheckResult, error) {
	// check if the TLD exists
	tld, err := svc.tldRepo.GetByName(ctx, q.DomainName.ParentDomain(), true)
	if err != nil {
		return nil, err
	}

	// if the phase is not provided, get the current GA phase
	var phase *entities.Phase
	if q.PhaseName == "" {
		phase, err = tld.GetCurrentGAPhase()
		if err != nil {
			return nil, err
		}
	} else {
		phases := tld.GetCurrentPhases()
		for _, p := range phases {
			if p.Name.String() == q.PhaseName {
				phase = &p
				break
			}
		}
		// if we didn't find the phase by name, return an error
		return nil, entities.ErrPhaseNotFound
	}

	// Create the result object
	result := queries.NewDomainCheckQueryResult(q.DomainName)
	// Check the availability
	_, err = svc.domainRepo.GetDomainByName(ctx, q.DomainName.String(), false)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			// If the domain is not found, it is available
			// TODO: FIXME: check if it is blocked
			result.Available = true
		} else {
			// if there is an other error, return it
			return nil, err
		}
	}
	// Return the result if fees are not required
	//TODO: FIXME: return a different struct without the fees if they are not required
	if !q.IncludeFees {
		return result, nil
	}
	// If fees are required, prepare the result
	result.PricePoints = queries.DomainPricePoints{}

	// GET THE PRICE and FEE objects for the phase and currency

	// Get the phase again preloading the price and fee objects
	phase, err = svc.phaseRepo.GetPhaseByTLDAndName(ctx, tld.Name.String(), phase.Name.String())
	if err != nil {
		return nil, err
	}

	// Add FEE and PRICE to the result
	for _, price := range phase.Prices {
		if price.Currency == q.Currency {
			result.PricePoints.Price = &price
			break
		}
	}
	for _, fee := range phase.Fees {
		if fee.Currency == q.Currency {
			result.PricePoints.Fees = append(result.PricePoints.Fees, fee)
		}
	}
	// Get the PremiumLabels for the premiumList associated with the phase
	if phase.PremiumListName != nil {
		result.PricePoints.PremiumPrice, err = svc.premiumLabelRepo.GetByLabelListAndCurrency(ctx, q.DomainName.Label(), *phase.PremiumListName, q.Currency)
		if err != nil {
			return nil, err
		}
	}

	// retrun the result
	return result, nil
}
