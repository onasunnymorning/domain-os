package entities

import (
	"strings"

	"github.com/biter777/countries"
	"github.com/pkg/errors"
)

// <simpleType name="ccType">
//       <restriction base="token">
//         <length value="2"/>
//       </restriction>
// </simpleType>

const (
	CCTYPE_LENGTH = 2
)

var (
	ErrInvalidCountryCode = errors.New("invalid country code")
)

// CCType is a string value that represents a country code as defined in RFC5733 https://www.rfc-editor.org/rfc/rfc5733.html#:~:text=types.%0A%20%20%20%2D%2D%3E%0A%20%20%20%20%3CsimpleType%20name%3D%22-,ccType,-%22%3E%0A%20%20%20%20%20%20%3Crestriction%20base%3D%22token
type CCType string

// Validate checks if the CCType is the correct length
func (c *CCType) Validate() error {
	if len(*c) != CCTYPE_LENGTH {
		return ErrInvalidCountryCode
	}
	if !countries.ByName(string(*c)).IsValid() {
		return ErrInvalidCountryCode
	}
	return nil
}

// NewCCType creates a new CCType
func NewCCType(cc string) (CCType, error) {
	c := CCType(strings.ToUpper(NormalizeString(cc)))
	if err := c.Validate(); err != nil {
		return CCType(""), ErrInvalidCountryCode
	}
	return c, nil
}

// String implements the Stringer interface
func (c CCType) String() string {
	return string(c)
}
