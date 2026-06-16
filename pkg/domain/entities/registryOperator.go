package entities

import (
	"net/mail"
	"strings"
	"time"

	"errors"
)

var (
	ErrInvalidRegistryOperator  = errors.New("invalid registry operator")
	ErrRegistryOperatorNotFound = errors.New("registry operator not found")
)

// RegistryOperator represents a registry Operator that manages one or more TLDs.
type RegistryOperator struct {
	RyID         ClIDType
	Name         string
	URL          URL
	Email        string
	Voice        E164Type
	Fax          E164Type
	CreatedAt    time.Time
	UpdatedAt    time.Time
	TLDs         []*TLD
	PremiumLists []*PremiumList
}

// NewRegistryOperator creates a new instance of RegistryOperator
func NewRegistryOperator(ryID, name, email string) (*RegistryOperator, error) {
	validatedRyID, err := NewClIDType(ryID)
	if err != nil {
		return nil, errors.Join(ErrInvalidRegistryOperator, err)
	}
	// Validate the email
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, errors.Join(ErrInvalidEmail, err)
	}

	return &RegistryOperator{
		RyID:      validatedRyID,
		Name:      NormalizeString(name),
		Email:     strings.ToLower(email),
		CreatedAt: RoundTime(time.Now().UTC()),
		UpdatedAt: RoundTime(time.Now().UTC()),
	}, nil
}
