package entities

import (
	"strings"

	"errors"

	"github.com/biter777/countries"
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

// alpha2Codes are the ISO 3166-1 alpha-2 codes, and XK, which Kosovo uses.
// They are listed rather than looked up by name: the library's name lookup
// also takes "XX", for none.
var alpha2Codes = func() map[string]bool {
	set := map[string]bool{}
	for _, c := range countries.All() {
		set[c.Alpha2()] = true
	}
	return set
}()

// Validate checks that the CCType is an ISO 3166-1 alpha-2 code, in either
// case.
func (c *CCType) Validate() error {
	if len(*c) != CCTYPE_LENGTH || !alpha2Codes[strings.ToUpper(string(*c))] {
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
