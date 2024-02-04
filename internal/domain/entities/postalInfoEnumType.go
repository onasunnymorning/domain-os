package entities

import (
	"strings"

	"github.com/pkg/errors"
)

const (
	PostalInfoEnumTypeINT = "int"
	PostalInfoEnumTypeLOC = "loc"
)

var (
	ErrInvalidPostalInfoEnumType = errors.New("invalid postalInfoEnumType")
)

// <simpleType name="postalInfoEnumType">
//       <restriction base="token">
//         <enumeration value="loc"/>
//         <enumeration value="int"/>
//       </restriction>
// </simpleType>

// PostalInofEnumType is a string value that represents a postal code as defined in RFC5733https://www.rfc-editor.org/rfc/rfc5733.html#:~:text=complexType%3E%0A%0A%20%20%20%20%3CsimpleType%20name%3D%22-,postalInfoEnumType,-%22%3E%0A%20%20%20%20%20%20%3Crestriction%20base%3D%22token
type PostalInfoEnumType string

// NewPostalInfoEnumType
func NewPostalInfoEnumType(t string) (*PostalInfoEnumType, error) {
	s := strings.ToLower(NormalizeString(t))
	enum := PostalInfoEnumType(s)
	if err := enum.Validate(); err != nil {
		return nil, err
	}
	return &enum, nil
}

// Validate checks if the value is valid
func (t PostalInfoEnumType) Validate() error {
	switch t {
	case PostalInfoEnumTypeINT:
		return nil
	case PostalInfoEnumTypeLOC:
		return nil
	default:
		return ErrInvalidPostalInfoEnumType
	}
}
