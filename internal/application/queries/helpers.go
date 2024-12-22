package queries

import (
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

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
