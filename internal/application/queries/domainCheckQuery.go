package queries

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// DomainCheckQuery represents a query to check the availability of a domain name.
type DomainCheckQuery struct {
	DomainName  entities.DomainName
	IncludeFees bool
	PhaseName   string
	Currency    string
}

// NewDomainCheckQuery creates a new instance of DomainCheckQuery. If the domain name is invalid, an error is returned so we can fail fast.
func NewDomainCheckQuery(domainName string, includeFees bool) (*DomainCheckQuery, error) {
	validatedDomainName, err := entities.NewDomainName(domainName)
	if err != nil {
		return nil, err
	}
	return &DomainCheckQuery{
		DomainName:  *validatedDomainName,
		IncludeFees: includeFees,
	}, nil
}

// DomainCheckResult represents the result of a domain check query.
type DomainCheckResult struct {
	DomainName  entities.DomainName
	TimeStamp   time.Time
	Available   bool
	PricePoints DomainPricePoints
}

// DomainPricePoints represents the all the price points for a domain.
type DomainPricePoints struct {
	Price        *entities.Price
	Fees         []entities.Fee
	PremiumPrice *entities.PremiumLabel
}

// NewDomainCheckQueryResult creates a new instance of DomainCheckQueryResult.
func NewDomainCheckQueryResult(domainName entities.DomainName) *DomainCheckResult {
	return &DomainCheckResult{
		DomainName: domainName,
		TimeStamp:  time.Now().UTC(),
	}
}
