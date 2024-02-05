package entities

import (
	"regexp"

	"github.com/pkg/errors"
)

// <simpleType name="e164StringType">
//       <restriction base="token">
//         <pattern value="(\+[0-9]{1,3}\.[0-9]{1,14})?"/>
//         <maxLength value="17"/>
//       </restriction>
// </simpleType>

const (
	E164_REGEX = `^\+[0-9]{1,3}\.[0-9]{1,14}$`
)

var (
	ErrInvalidE164Type = errors.New("invalid e164Type")
)

// E164Type is a type for E164 phone number
type E164Type string

// NewE164Type creates a new instance of E164Type
func NewE164Type(phoneNumber string) (*E164Type, error) {
	e := E164Type(NormalizeString(phoneNumber))
	if err := e.Validate(); err == nil {
		return &e, nil
	}
	return nil, ErrInvalidE164Type
}

// IvValid checks if E164Type is valid
func (e E164Type) Validate() error {
	// Is not a required field, so can be empty string
	if e == "" {
		return nil
	}
	// If not empty mus match the pattern
	r := regexp.MustCompile(E164_REGEX)
	if !r.MatchString(string(e)) {
		return ErrInvalidE164Type
	}
	return nil
}

// String returns the string value of the E164Type
func (e E164Type) String() string {
	return string(e)
}
