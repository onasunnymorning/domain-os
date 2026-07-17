package queries

import (
	"errors"
	"time"

	"github.com/onasunnymorning/domain-os/pkg/domain/entities"
)

// PurgeableDomainsQuery represents a query to get a list of purgeable domains:
// domains that are pendingDelete and whose purge date falls on or before the
// cutoff time (Before).
type PurgeableDomainsQuery struct {
	// Before is the cutoff: domains with purge_date <= Before match.
	Before time.Time
	ClID   entities.ClIDType
	TLD    entities.DomainName
	// PageSize caps the number of results returned per page. Zero means the
	// caller's default applies.
	PageSize int
}

// NewPurgeableDomainsQuery creates a new instance of PurgeableDomainsQuery. It will return an error if the ClID or date are invalid. It expects date to be in RFC3339 or yyyy-mm-dd format. Date, clid and tld can be empty strings (""); an empty date defaults to the current time.
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
