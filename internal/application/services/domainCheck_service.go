package services

import (
	"context"
	"errors"
	"strings"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

// DomainCheckService is the implementation of the DomainCheckService interface
type DomainCheckService struct {
	domainRepo       repositories.DomainRepository
	nndnRepo         repositories.NNDNRepository
	tldRepo          repositories.TLDRepository
	phaseRepo        repositories.PhaseRepository
	premiumLabelRepo repositories.PremiumLabelRepository
}

// NewDomainCheckService returns a new instance of DomainCheckService
func NewDomainCheckService(dr repositories.DomainRepository, nr repositories.NNDNRepository, tr repositories.TLDRepository, plr repositories.PremiumLabelRepository, phr repositories.PhaseRepository) *DomainCheckService {
	return &DomainCheckService{
		domainRepo:       dr,
		nndnRepo:         nr,
		tldRepo:          tr,
		phaseRepo:        phr,
		premiumLabelRepo: plr,
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
	result.Available = true
	// Check the availability
	dom, err := svc.domainRepo.GetDomainByName(ctx, q.DomainName.String(), false)
	if err != nil && !errors.Is(err, entities.ErrDomainNotFound) {
		return nil, err
	}
	// If the domain exists, set availability
	if dom != nil {
		result.Available = false
		result.Reason = "In Use"
		// Return the result now if fees are not required
		if !q.IncludeFees {
			return result, nil
		}
	}
	nndn, err := svc.nndnRepo.GetNNDN(ctx, q.DomainName.String())
	if err != nil && !errors.Is(err, entities.ErrNNDNNotFound) {
		return nil, err
	}
	// If the domain exists in the NNDN, it is blocked, return the result immediately
	if nndn != nil {
		result.Available = false
		result.Reason = string(nndn.NameState)
		// Return the result now if fees are not required
		if !q.IncludeFees {
			return result, nil
		}
	}

	// So far so good, the domain doesn't exist and is not blocked
	// Return the result now if fees are not required
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

	// Find the price entry for the currency
	for _, price := range phase.Prices {
		if price.Currency == strings.ToUpper(q.Currency) {
			result.PricePoints.Price = &price
			break
		}
	}
	// Find all fees for the currency
	for _, fee := range phase.Fees {
		if fee.Currency == strings.ToUpper(q.Currency) {
			result.PricePoints.Fees = append(result.PricePoints.Fees, fee)
		}
	}
	// Get the PremiumLabels for the premiumList associated with the phase
	if phase.PremiumListName != nil {
		result.PricePoints.PremiumPrice, err = svc.premiumLabelRepo.GetByLabelListAndCurrency(ctx, q.DomainName.Label(), *phase.PremiumListName, q.Currency)
		if err != nil && !errors.Is(err, entities.ErrPremiumLabelNotFound) {
			return nil, err
		}
	}

	// retrun the result
	return result, nil
}
