package entities

import (
	"strings"

	"github.com/pkg/errors"
)

var (
	ErrInvalidContactPostalInfo = errors.New("invalid contact postalinfo")
)

// ContactPostalInfo is a value object that represents a postal code as defined in RFC5733
type ContactPostalInfo struct {
	Type    PostalInfoEnumType `json:"type" example:"loc" extensions:"x-order=0"`
	Name    PostalLineType     `json:"name" example:"Gerardo Aguantis" extensions:"x-order=1"`
	Org     OptPostalLineType  `json:"org" example:"Agua Britanica" extensions:"x-order=2"`
	Address *Address           `json:"address" extensions:"x-order=3"`
}

// NewContactPostalInfo creates a new ContactPostalInfo
func NewContactPostalInfo(t, name string, address *Address) (*ContactPostalInfo, error) {
	piType, err := NewPostalInfoEnumType(t)
	if err != nil {
		return nil, err
	}
	if err := address.Validate(*piType); err != nil {
		return nil, err
	}
	a := &ContactPostalInfo{
		Type:    PostalInfoEnumType(strings.ToLower(NormalizeString(t))),
		Name:    PostalLineType(NormalizeString(name)),
		Address: address,
	}
	if !a.IsValid() {
		return nil, ErrInvalidContactPostalInfo
	}
	return a, nil
}

// IsValid checks if the value is valid
func (t *ContactPostalInfo) IsValid() bool {
	if err := t.Type.Validate(); err != nil {
		return false
	}
	if err := t.Name.Validate(); err != nil {
		return false
	}
	if err := t.Org.IsValid(); err != nil {
		return false
	}
	if t.Address == nil {
		return false
	} else {
		if err := t.Address.Validate(t.Type); err != nil {
			return false
		}
	}
	if t.Type == PostalInfoEnumTypeINT {
		if !IsASCII(t.Name.String()) {
			return false
		}
		if !IsASCII(t.Org.String()) {
			return false
		}
	}
	return true
}
