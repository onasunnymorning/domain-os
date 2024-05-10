package services

import (
	"context"
	"errors"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

var (
	// ErrDomainExists is returned when a domain already exists
	ErrDomainExists = errors.New("domain exists")
	// ErrDomainBlocked is returned when a domain is blocked
	ErrDomainBlocked = errors.New("domain is blocked")
	// ErrPhaseRequired is returned when a phase is required to check domain availability
	ErrPhaseRequired = errors.New("phase is required to check domain availability")
	// ErrLabelNotValidInPhase is returned when a label is not valid in a phase
	ErrLabelNotValidInPhase = errors.New("label is not valid in this phase")
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

// CheckDomainExists checks if a domain exists. If the domain exists, the function returns true, otherwise it returns false. If an error occurs, it is returned.
func (svc *DomainCheckService) CheckDomainExists(ctx context.Context, domainName string) (bool, error) {
	_, err := svc.domainRepo.GetDomainByName(ctx, domainName, false)
	if err != nil {
		if errors.Is(err, entities.ErrDomainNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CheckDomainIsBlocked checks if a domain is blocked. If the domain is blocked, the function returns true, otherwise it returns false. If an error occurs, it is returned.
func (svc *DomainCheckService) CheckDomainIsBlocked(ctx context.Context, domainName string) (bool, error) {
	_, err := svc.nndnRepo.GetNNDN(ctx, domainName)
	if err != nil {
		if errors.Is(err, entities.ErrNNDNNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CheckDomainAvailability checks if a domain is available. A domain is availabel if
// * it is a valid domain name
// * it does not exist
// * it is not blocked
// * it is allowed in the current phase
func (svc *DomainCheckService) CheckDomainAvailability(ctx context.Context, domainName string, phase *entities.Phase) (bool, error) {
	if phase == nil {
		return false, ErrPhaseRequired
	}
	dom, err := entities.NewDomainName(domainName)
	if err != nil {
		return false, err
	}

	// Check if the domain exists
	exists, err := svc.CheckDomainExists(ctx, domainName)
	if err != nil {
		return false, err
	}
	if exists {
		return false, ErrDomainExists
	}

	// Check if the domain is blocked
	blocked, err := svc.CheckDomainIsBlocked(ctx, domainName)
	if err != nil {
		return false, err
	}
	if blocked {
		return false, ErrDomainBlocked
	}

	// Check if the domain is allowed in the current phase
	if !phase.Policy.LabelIsAllowed(dom.Label()) {
		return false, ErrLabelNotValidInPhase
	}

	// If all checks pass, the domain is available
	return true, nil
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
	} else { // if the phase is provided, get the phase by name
		phase, err = tld.FindPhaseByName(entities.ClIDType(q.PhaseName))
		if err != nil {
			return nil, err
		}
	}

	avail, err := svc.CheckDomainAvailability(ctx, q.DomainName.String(), phase)
	if err != nil && !errors.Is(err, ErrDomainExists) && !errors.Is(err, ErrDomainBlocked) {
		return nil, err
	}
	// Create the result object
	result := queries.NewDomainCheckQueryResult(q.DomainName)
	// set the availability and reason
	result.Available = avail
	if !avail {
		result.Reason = err.Error()
	}

	// So far so good, the domain doesn't exist and is not blocked
	// Return the result now if fees are not required
	if !q.IncludeFees {
		return result, nil
	}
	// If fees are required, prepare the result
	result.PricePoints = &queries.DomainPricePoints{}

	// GET THE PRICE and FEE objects for the phase and currency

	// Get the phase again preloading the price and fee objects
	phase, err = svc.phaseRepo.GetPhaseByTLDAndName(ctx, tld.Name.String(), phase.Name.String())
	if err != nil {
		return nil, err
	}

	// set the price for the currency.
	result.PricePoints.Price, _ = phase.GetPrice(q.Currency)

	// set the fees for the currency
	result.PricePoints.Fees = phase.GetFees(q.Currency)

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
