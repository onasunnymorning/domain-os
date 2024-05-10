package queries

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// DomainCheckQuery represents a query to check the availability of a domain name.
type DomainCheckQuery struct {
	DomainName  entities.DomainName // Fail fast, if the domain name is invalid
	IncludeFees bool                // Include fees in the result e.g. fee extension
	PhaseName   string              // Phase name - if empty the current GA phase is assumed
	Currency    string              // Currency to use for the price check
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
	TimeStamp   time.Time
	DomainName  entities.DomainName
	Available   bool
	Reason      string
	PhaseName   string
	PricePoints *DomainPricePoints `json:",omitempty"` // don't include if nil
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
