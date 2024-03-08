package entities

import "github.com/pkg/errors"

var (
	ErrInvalidRegistrarPostalInfo = errors.New("invalid registrar postalinfo")
)

// RegistrarPostalInfo is a value object that represents a postal code as defined in RFC5733
type RegistrarPostalInfo struct {
	Type    PostalInfoEnumType `json:"type" example:"loc" extensions:"x-order=0"`
	Address *Address           `json:"address" extensions:"x-order=1"`
}

// NewRegistrarPostalInfo creates a new RegistrarPostalInfo
func NewRegistrarPostalInfo(t string, Address *Address) (*RegistrarPostalInfo, error) {
	pit, err := NewPostalInfoEnumType(t)
	if err != nil {
		return nil, err
	}
	a := &RegistrarPostalInfo{
		Type:    *pit,
		Address: Address,
	}
	if err := a.IsValid(); err != nil {
		return nil, ErrInvalidRegistrarPostalInfo
	}
	return a, nil
}

// IsValid checks if the value is valid
func (t *RegistrarPostalInfo) IsValid() error {
	if err := t.Type.Validate(); err != nil {
		return err
	}
	if t.Address == nil {
		return ErrInvalidRegistrarPostalInfo
	} else {
		if err := t.Address.Validate(t.Type); err != nil {
			return err
		}
	}
	return nil
}
