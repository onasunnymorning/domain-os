package entities

import (
	"errors"
)

const (
	POSTAL_LINE_TYPE_MIN_LENGTH = 1
	POSTAL_LINE_TYPE_MAX_LENGTH = 255
)

// <simpleType name="postalLineType">
//        <restriction base="normalizedString">
//          <minLength value="1"/>
//          <maxLength value="255"/>
//        </restriction>
// </simpleType>

// <simpleType name="optPostalLineType">
//        <restriction base="normalizedString">
//          <maxLength value="255"/>
//        </restriction>
// </simpleType>

var (
	ErrInvalidPostalLineType    = errors.New("invalid postalLineType")
	ErrInvalidOptPostalLineType = errors.New("invalid optPostalLineType")
)

// PostalLineType is a string value that represents a postal info line as defined in RFC5733 https://datatracker.ietf.org/doc/html/rfc5733#:~:text=simpleType%3E%0A%0A%20%20%20%20%3CsimpleType%20name%3D%22-,postalLineType,-%22%3E%0A%20%20%20%20%20%20%20%3Crestriction%20base%3D%22normalizedString
type PostalLineType string

// Validate checks if the PostalInfoLineType is within the minimum and maximum length
func (p *PostalLineType) Validate() error {
	if len(*p) < POSTAL_LINE_TYPE_MIN_LENGTH || len(*p) > POSTAL_LINE_TYPE_MAX_LENGTH {
		return ErrInvalidPostalLineType
	}
	return nil
}

// NewPostalLineType creates a new PostalLineType
func NewPostalLineType(postalLine string) (*PostalLineType, error) {
	p := PostalLineType(NormalizeString(postalLine))
	if err := p.Validate(); err != nil {
		return nil, err
	}
	return &p, nil
}

// String returns the string value of the PostalLineType
func (p *PostalLineType) String() string {
	return string(*p)
}

// OptPostalLineType is a string value that represents an optional postal info line as defined in RFC5733 https://datatracker.ietf.org/doc/html/rfc5733#:~:text=simpleType%3E%0A%0A%20%20%20%20%3CsimpleType%20name%3D%22-,optPostalLineType,-%22%3E%0A%20%20%20%20%20%20%20%3Crestriction%20base%3D%22normalizedString
type OptPostalLineType string

// IsValid checks if the OptionalPostalInfoLineType does not exceed the maximum length
func (p *OptPostalLineType) IsValid() error {
	if len(*p) <= POSTAL_LINE_TYPE_MAX_LENGTH {
		return nil
	}
	return ErrInvalidOptPostalLineType
}

// NewOptPostalLineType creates a new OptPostalLineType
func NewOptPostalLineType(postalLine string) (*OptPostalLineType, error) {
	p := OptPostalLineType(postalLine)
	if err := p.IsValid(); err != nil {
		return nil, err
	}
	return &p, nil
}

// String returns the string value of the OptPostalLineType
func (p *OptPostalLineType) String() string {
	return string(*p)
}
