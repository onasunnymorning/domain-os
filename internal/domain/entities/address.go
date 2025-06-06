package entities

import "errors"

// Inspiration for this entity comes from: https://www.rfc-editor.org/rfc/rfc9022.html#name-rderegistrarregistrar-eleme
// Matching the RFC is important for interoperability with other systems such as EPP, RDDS, RDAP, Escrow, etc.
// While there will be relatively little Registrar object, the Contact object is very similar so can use the same structure and benefits.

var (
	ErrInvalidCity              = errors.New("invalid city")
	ErrInvalidPostalCode        = errors.New("invalid postal code")
	ErrInvalidStreet            = errors.New("invalid street")
	ErrInvalidStreetCount       = errors.New("invalid street count")
	ErrInvalidStateProvince     = errors.New("invalid state/province")
	ErrInvalidASCIIInIntAddress = errors.New("invalid address: non-ASCII in INT object")
)

// Addr value object used in Contact and Registrar
type Address struct {
	Street1       OptPostalLineType `json:"Street1" example:"Boulnes 2545" extensions:"x-order=0"`
	Street2       OptPostalLineType `json:"Street2" example:"Piso8" extensions:"x-order=1"`
	Street3       OptPostalLineType `json:"Street3" example:"Portero" extensions:"x-order=2"`
	City          PostalLineType    `json:"City" binding:"required" example:"Buenos Aires" extensions:"x-order=3"`
	StateProvince OptPostalLineType `json:"SP" example:"Palermo SOHO" extensions:"x-order=4"`
	PostalCode    PCType            `json:"PC" example:"EN234Z" extensions:"x-order=5"`
	CountryCode   CCType            `json:"CC" binding:"required" example:"AR" extensions:"x-order=6"`
}

// NewAddress creates a new Address
func NewAddress(City, CountryCode string) (*Address, error) {
	cc, err := NewCCType(CountryCode)
	if err != nil {
		return nil, err
	}
	c, err := NewPostalLineType(City)
	if err != nil {
		return nil, err
	}
	return &Address{
		City:        *c,
		CountryCode: cc,
	}, nil
}

// Validate
func (a *Address) Validate(t PostalInfoEnumType) error {
	if err := a.City.Validate(); err != nil {
		return err
	}
	if err := a.CountryCode.Validate(); err != nil {
		return ErrInvalidCountryCode
	}
	if err := a.PostalCode.Validate(); err != nil {
		return err
	}
	if err := a.Street1.IsValid(); err != nil {
		return err
	}
	if err := a.Street2.IsValid(); err != nil {
		return err
	}
	if err := a.Street3.IsValid(); err != nil {
		return err
	}
	if err := a.StateProvince.IsValid(); err != nil {
		return err
	}
	if t == PostalInfoEnumTypeINT {
		_, err := a.IsASCII()
		if err != nil {
			return err
		}
	}
	return nil
}

// IsASCII checks if all the fields in the address are valid ASCII
func (a *Address) IsASCII() (bool, error) {
	if a.Street1.String() != "" && !IsASCII(a.Street1.String()) {
		return false, ErrInvalidASCIIInIntAddress
	}
	if a.Street2.String() != "" && !IsASCII(a.Street2.String()) {
		return false, ErrInvalidASCIIInIntAddress
	}
	if a.Street3.String() != "" && !IsASCII(a.Street3.String()) {
		return false, ErrInvalidASCIIInIntAddress
	}
	if !IsASCII(a.City.String()) {
		return false, ErrInvalidASCIIInIntAddress
	}
	if a.StateProvince.String() != "" && !IsASCII(a.StateProvince.String()) {
		return false, ErrInvalidASCIIInIntAddress
	}
	if a.PostalCode.String() != "" && !IsASCII(a.PostalCode.String()) {
		return false, ErrInvalidASCIIInIntAddress
	}
	if !IsASCII(a.CountryCode.String()) {
		return false, ErrInvalidASCIIInIntAddress
	}
	return true, nil
}

// Address is effectively all value types (string aliases), so a simple copy suffices.
// However, we’ll give it a method for consistency and future-proofing:
func (a Address) DeepCopy() Address {
	// Since everything inside Address is just a string or string alias,
	// a shallow copy is enough; there's nothing that can share memory inside.
	return a
}
