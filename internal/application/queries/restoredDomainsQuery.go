package queries

import "github.com/onasunnymorning/domain-os/internal/domain/entities"

// RestoredDomainsQuery represents a query to get a list of restored domains.
type RestoredDomainsQuery struct {
	ClID entities.ClIDType
	TLD  entities.DomainName
}

// NewRestoredDomainsQuery creates a new instance of RestoredDomainsQuery after validating the provided
// client ID (is valid clidType) and top-level domain (domainname). It returns a pointer to the RestoredDomainsQuery and
// an error if the validation fails.
func NewRestoredDomainsQuery(clid, tld string) (*RestoredDomainsQuery, error) {
	validatedClID, err := parseClID(clid)
	if err != nil {
		return nil, err
	}
	validatedTLD, err := parseTld(tld)
	if err != nil {
		return nil, err
	}
	return &RestoredDomainsQuery{
		ClID: validatedClID,
		TLD:  *validatedTLD,
	}, nil
}
