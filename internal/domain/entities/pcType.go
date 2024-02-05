package entities

import "github.com/pkg/errors"

const (
	PCTYPE_MAX_LENGTH = 16
)

var (
	ErrInvalidPCType = errors.New("invalid pcType")
)

// <simpleType name="pcType">
//       <restriction base="token">
//         <maxLength value="16"/>
//       </restriction>
// </simpleType>/

// PCType is a string value that represents a postal code as defined in RFC5733 https://datatracker.ietf.org/doc/html/rfc5733#:~:text=simpleType%3E%0A%0A%20%20%20%20%3CsimpleType%20name%3D%22-,pcType,-%22%3E%0A%20%20%20%20%20%20%3Crestriction%20base%3D%22token
type PCType string

// Validate checks if the PCType does not exceed the maximum length
func (p *PCType) Validate() error {
	if len(*p) <= PCTYPE_MAX_LENGTH {
		return nil
	}
	return ErrInvalidPCType
}

// NewPCType creates a new PCType
func NewPCType(pc string) (*PCType, error) {
	p := PCType(NormalizeString(pc))
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// String returns the string value of the PCType
func (p *PCType) String() string {
	return string(*p)
}
