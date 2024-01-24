package entities

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
)

const (
	DOMAIN_MAX_LEN = 253
	DOMAIN_MIN_LEN = 1
)

var (
	ErrInvalidDomainName = errors.New("invalid domain name")
)

// A domainname is an alias for a string
type DomainName string

// NewDomainName returns a pointer to a DomainName struct or an error (ErrInvalidDomainName) if the domain name is invalid
// It normalizes the input string before validating it and Trims leading and trailing dots
// A single label is also a valid domain name
func NewDomainName(name string) (*DomainName, error) {
	n := NormalizeString(name)
	d := DomainName(strings.Trim(strings.ToLower(n), "."))
	if !d.IsValid() {
		return nil, ErrInvalidDomainName
	}
	return &d, nil
}

// IsValid returns a boolean indicating if the domain name is valid or not
// A domain name is a FQDN (Fully Qualified Domain Name) and can contain letters, digits and hyphens
// A domain name can be between 1 and 253 characters long
// A domain consists of valid labels separated by dots
func (d *DomainName) IsValid() bool {
	if len(d.String()) > DOMAIN_MAX_LEN || len(d.String()) < DOMAIN_MIN_LEN {
		return false
	}
	labels := strings.Split(d.String(), ".")
	for _, label := range labels {
		if !IsValidLabel(label) {
			return false
		}
	}
	return true
}

// Returns the parent domain of the domain name
func (d *DomainName) ParentDomain() string {
	labels := strings.Split(string(*d), ".")
	return strings.Join(labels[1:], ".")
}

// Returns the domain name as a string
func (d *DomainName) String() string {
	return string(*d)
}

// UnmarshalJSON implements json.Unmarshaler interface for DomainName
func (d *DomainName) UnmarshalJSON(bytes []byte) error {
	var name string
	err := json.Unmarshal(bytes, &name)
	if err != nil {
		return err
	}
	*d = DomainName(name)
	return nil
}
