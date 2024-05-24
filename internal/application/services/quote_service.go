package services

import (
	"context"
	"errors"
	"strings"

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
		phase, err = tld.GetCurrentGAPhase()
		if err != nil {
			return nil, err
		}
	} else {
		// Otherwise, use the specified phase
		phase, err = tld.FindPhaseByName(entities.ClIDType(q.PhaseName))
		if err != nil {
			return nil, err
		}
	}
	if phase == nil {
		return nil, entities.ErrPhaseNotFound
	}

	domain, err := s.domainRepo.GetDomainByName(ctx, domainName.String(), false)
	// Get the domain
	if err != nil {
		if !errors.Is(err, entities.ErrDomainNotFound) {
			// If there was an error other than domain not found, return it
			return nil, err
		}
		// If we don't have the domain, create a placeholder
		domain, err = entities.NewDomain("123_DOM-APEX", domainName.String(), q.ClID, "str0ngP@zz")
		if err != nil {
			return nil, err
		}
	}

	// Get the PremiumLabels in all currencies
	pe := []*entities.PremiumLabel{}
	if phase.PremiumListName != nil {
		pe, err = s.premiumLabelRepo.List(ctx, 25, "", *phase.PremiumListName, "", domainName.Label())
		if err != nil {
			return nil, err
		}
	}

	// Create a default FX in case we don't need FX
	fx := &entities.FX{
		BaseCurrency:   phase.Policy.BaseCurrency,
		TargetCurrency: phase.Policy.BaseCurrency,
		Rate:           1,
	}
	if q.Currency != phase.Policy.BaseCurrency {
		fx, err = s.fxRepo.GetByBaseAndTargetCurrency(ctx, phase.Policy.BaseCurrency, strings.ToUpper(q.Currency))
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
