package entities

import (
	"errors"
)

const (
	CLID_MIN_LENGTH = 3
	CLID_MAX_LENGTH = 16
)

var (
	ErrInvalidClIDType = errors.New("invalid clIDType")
)

// ClIDType represents the client identifier as used throughout the EPP protocol.
// <simpleType name="clIDType">
//
//	<restriction base="token">
//	  <minLength value="3"/>
//	  <maxLength value="16"/>
//	</restriction>
//
// </simpleType>
type ClIDType string

// NewClIDType creates a new instance of ClIDType. It checks if it is a valid clIDType and if only ASCII characters are used
func NewClIDType(clID string) (ClIDType, error) {
	c := ClIDType(NormalizeString(clID))
	if err := c.Validate(); err != nil {
		return ClIDType(""), ErrInvalidClIDType
	}
	return c, nil
}

// Validate checks if the ClIDType is valid
// It is valid when the length is between 3 and 16 characters
// and only ASCII characters are used
func (c *ClIDType) Validate() error {
	if len(c.String()) < CLID_MIN_LENGTH || len(c.String()) > CLID_MAX_LENGTH {
		return ErrInvalidClIDType
	}
	if !IsASCII(c.String()) {
		return ErrInvalidClIDType
	}
	return nil
}

// String implements the Stringer interface
func (c *ClIDType) String() string {
	return string(*c)
}
