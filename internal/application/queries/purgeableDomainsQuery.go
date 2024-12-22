package queries

import (
	"errors"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// ExpiringDomainsQuery represents a query to get a list of expiring domains.
type PurgeableDomainsQuery struct {
	Before time.Time
	ClID   entities.ClIDType
	TLD    entities.DomainName
}

// NewExpiringDomainsQuery creates a new instance of ExpiringDomainsQuery. It will return an error if the ClID or date are invalid. It expects date to be in dd-mm-yyyy format. Both date and clid can be empty strings ("").
func NewPurgeableDomainsQuery(clid, date, tld string) (*PurgeableDomainsQuery, error) {
	validatedDate, err := parseTimeDefault(date)
	if err != nil {
		return nil, errors.Join(ErrInvalidTimeFormat, err)
	}
	validatedClID, err := parseClID(clid)
	if err != nil {
		return nil, err
	}
	validatedTLD, err := parseTld(tld)
	if err != nil {
		return nil, err
	}
	return &PurgeableDomainsQuery{
		Before: validatedDate,
		ClID:   validatedClID,
		TLD:    *validatedTLD,
	}, nil
}
