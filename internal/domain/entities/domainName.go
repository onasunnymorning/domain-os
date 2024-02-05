package entities

import (
	"encoding/json"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/net/idna"
)

const (
	DOMAIN_MAX_LEN = 253
	DOMAIN_MIN_LEN = 1
)

var (
	ErrinvalIdDomainNameLength = errors.New("invalid domain name length. Domain name must be between 1 and 253 characters long")
	ErrInvalidDomainName       = errors.New("invalid domain name")
)

// A domainname is an alias for a string
type DomainName string

// NewDomainName returns a pointer to a DomainName struct or an error (ErrInvalidDomainName) if the domain name is invalid
// It normalizes the input string before validating it and Trims leading and trailing dots
// A single label is also a valid domain name
func NewDomainName(name string) (*DomainName, error) {
	n := NormalizeString(strings.ToLower(name))
	d := DomainName(strings.Trim(n, ".")) // trim leading and trailing dots
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

// Validate returns an error indicating if the domain name is valid or not
// A domain name is a FQDN (Fully Qualified Domain Name) and can contain letters, digits and hyphens
// A domain name can be between 1 and 253 characters long
// A domain consists of valid labels separated by dots
func (d *DomainName) Validate() error {
	if len(d.String()) > DOMAIN_MAX_LEN || len(d.String()) < DOMAIN_MIN_LEN {
		return ErrinvalIdDomainNameLength
	}

	// Verify that each label is valid
	for _, label := range d.GetLabels() {
		if err := label.Validate(); err != nil {
			return err
		}
	}
	return nil
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

// ToUnicode returns the Unicode representation of the domain name
func (d *DomainName) ToUnicode() (string, error) {
	return idna.ToUnicode(d.String())
}

// GetLabels returns a slice of Labels from the domain name
func (d *DomainName) GetLabels() []Label {
	labelStrings := strings.Split(d.String(), ".")
	l := make([]Label, len(labelStrings))
	for i, label := range labelStrings {
		l[i] = Label(label)
	}
	return l
}
