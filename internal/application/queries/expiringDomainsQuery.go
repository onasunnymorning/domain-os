package queries

import (
	"errors"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

var (
	ErrInvalidTimeFormat = errors.New("invalid time format, expected yyyy-mm-dd or RFC3339")
)

// ExpiringDomainsQuery represents a query to get a list of expiring domains.
type ExpiringDomainsQuery struct {
	Before time.Time
	ClID   entities.ClIDType
	TLD    entities.DomainName
}

// NewExpiringDomainsQuery creates a new instance of ExpiringDomainsQuery. It will return an error if the ClID or date are invalid. It expects date to be in dd-mm-yyyy format. Both date and clid can be empty strings ("").
func NewExpiringDomainsQuery(clid, date, tld string) (*ExpiringDomainsQuery, error) {
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
	return &ExpiringDomainsQuery{
		Before: validatedDate,
		ClID:   validatedClID,
		TLD:    *validatedTLD,
	}, nil
}

// parseTimeDefault parses a date string in RFC3339 or yyyy-mm-dd format and returns a time.Time object. In case the date is empty, it will return the current time.
func parseTimeDefault(date string) (time.Time, error) {
	if date == "" {
		return time.Now().UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, date)
	if err != nil {
		t, err = time.Parse(time.DateOnly, date)
		if err != nil {
			return time.Time{}, err
		}
	}
	return t, nil
}

// parseClID validates the ClID and returns it as a ClIDType. It will return an empty ClIDType if the ClID is empty.
func parseClID(clid string) (entities.ClIDType, error) {
	if clid == "" {
		var ClID entities.ClIDType
		return ClID, nil
	}
	return entities.NewClIDType(clid)
}

// parseTld validates the TLD and returns it as a DomainName. It will return an empty DomainName if the TLD is empty.
func parseTld(tld string) (*entities.DomainName, error) {
	if tld == "" {
		var TLD entities.DomainName
		return &TLD, nil
	}
	return entities.NewDomainName(tld)
}
