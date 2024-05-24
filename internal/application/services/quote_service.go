package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/onasunnymorning/domain-os/internal/application/queries"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/onasunnymorning/domain-os/internal/domain/repositories"
)

var (
	ErrMissingFXRate = errors.New("no currency conversion rate available")
)

// QuoteService implements the QuoteService interface
type QuoteService struct {
	tldRepo          repositories.TLDRepository
	domainRepo       repositories.DomainRepository
	premiumLabelRepo repositories.PremiumLabelRepository
	fxRepo           repositories.FXRepository
}

// NewQuoteService returns a new QuoteService
func NewQuoteService(
	tr repositories.TLDRepository,
	dr repositories.DomainRepository,
	plr repositories.PremiumLabelRepository,
	fr repositories.FXRepository) *QuoteService {
	return &QuoteService{
		tldRepo:          tr,
		domainRepo:       dr,
		premiumLabelRepo: plr,
		fxRepo:           fr,
	}
}

// GetQuote returns a quote for a domain
func (s *QuoteService) GetQuote(ctx context.Context, q *queries.QuoteRequest) (*entities.Quote, error) {
	// Validate the request and
	if err := q.Validate(); err != nil {
		return nil, err
	}
	domainName, err := entities.NewDomainName(q.DomainName)
	if err != nil {
		return nil, err
	}
	// Get a fully preloaded TLD
	tld, err := s.tldRepo.GetByName(ctx, domainName.ParentDomain(), true)
	if err != nil {
		return nil, err
	}
	// If no phase name is provided, default to the "Currently Active GA Phase"
	var phase *entities.Phase
	if q.PhaseName == "" {
		fmt.Println("### Getting Current GA phase ###")
		phase, err = tld.GetCurrentGAPhase()
		if err != nil {
			return nil, err
		}
		fmt.Println("### Current GA phase: " + phase.Name.String() + " ###")
	} else {
		// Otherwise, use the specified phase
		fmt.Println("### Getting phase by NAME ###")
		phase, err = tld.FindPhaseByName(entities.ClIDType(q.PhaseName))
		if err != nil {
			return nil, err
		}
	}
	if phase == nil {
		fmt.Println("### NIL PHASE ###")
		return nil, entities.ErrPhaseNotFound
	}

	// Get the domain
	fmt.Println("### GETTING DOMAIN ###")
	domain, err := s.domainRepo.GetDomainByName(ctx, domainName.String(), false)
	fmt.Printf("### DOMAIN %v ###\n", domain)
	if err != nil {
		if !errors.Is(err, entities.ErrDomainNotFound) {
			// If there was an error other than domain not found, return it
			return nil, err
		}
		// If we don't have the domain, create a placeholder
		fmt.Println("### CREATING PLACEHOLDER DOMAIN ###")
		domain, err = entities.NewDomain("123_DOM-APEX", domainName.String(), q.ClID, "str0ngP@zz")
		if err != nil {
			return nil, err
		}
	}

	fmt.Printf("### DOML %v ###\n", domain)
	fmt.Printf("### PHASE %v ###\n", phase)
	// Get the PremiumLabels in all currencies
	var pe []*entities.PremiumLabel
	if phase.PremiumListName != nil {
		fmt.Println("#### LISTNAME" + *phase.PremiumListName)
		fmt.Println("#### domainName" + domainName.Label())
		pe, err := s.premiumLabelRepo.List(ctx, 25, "", *phase.PremiumListName, "", domainName.Label())
		if err != nil {
			return nil, err
		}
		fmt.Printf("### PE %v ###\n", pe)
	} else {
		pe = []*entities.PremiumLabel{}
		fmt.Println("### NO PREMIUM LABELS ###")
	}

	// Create a default FX in case we don't need FX
	fx := &entities.FX{
		BaseCurrency:   phase.Policy.BaseCurrency,
		TargetCurrency: phase.Policy.BaseCurrency,
		Rate:           1,
	}
	if q.Currency != phase.Policy.BaseCurrency {
		fx, err = s.fxRepo.GetByBaseAndTargetCurrency(ctx, phase.Policy.BaseCurrency, q.Currency)
		if err != nil {
			// If we don't have an FX rate, and we need it, return an error
			return nil, errors.Join(ErrMissingFXRate, err)
		}
	}

	// Instantiate a PriceEngine
	calc := entities.NewPriceEngine(*phase, *domain, *fx, pe)

	// Get/Return the quote
	return calc.GetQuote(*q.ToEntity())
}
