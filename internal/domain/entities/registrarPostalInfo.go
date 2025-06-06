package entities

import "errors"

var (
	ErrInvalidRegistrarPostalInfo = errors.New("invalid registrar postalinfo")
)

// RegistrarPostalInfo is a value object that represents a postal code as defined in RFC5733
type RegistrarPostalInfo struct {
	Type    PostalInfoEnumType `json:"Type" example:"loc" extensions:"x-order=0"`
	Address *Address           `json:"Address" extensions:"x-order=1"`
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
		return nil, errors.Join(ErrInvalidRegistrarPostalInfo, err)
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

// DeepCopy creates a new RegistrarPostalInfo with a copy of the original values
func (rp RegistrarPostalInfo) DeepCopy() RegistrarPostalInfo {
	// Shallow copy of rp
	copyRP := rp

	// Address is a pointer, so we need to allocate new memory if it's non-nil
	if rp.Address != nil {
		addrCopy := rp.Address.DeepCopy() // calls Address.DeepCopy()
		copyRP.Address = &addrCopy
	}

	return copyRP
}
