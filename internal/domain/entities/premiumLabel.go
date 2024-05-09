package entities

import (
	"errors"
	s "strings"

	"github.com/Rhymond/go-money"
)

var (
	ErrInvalidPremiumClass  = errors.New("invalid premium class")
	ErrPremiumLabelNotFound = errors.New("premium label not found")
)

// PremiumLabel represents a premium label entity
type PremiumLabel struct {
	Label              Label
	PremiumListName    string
	RegistrationAmount uint64
	RenewalAmount      uint64
	TransferAmount     uint64
	RestoreAmount      uint64
	Currency           string
	Class              string
}

// NewPremiumLabel creates a new PremiumLabel instance. It validates the currency, label and class (class string must be a valid clIDType).
func NewPremiumLabel(label string, registrationAmount, renewalAmount, transferAmount, restoreAmount uint64, currency, class, listName string) (*PremiumLabel, error) {
	validatedLabel := Label(label)
	if err := validatedLabel.Validate(); err != nil {
		return nil, err
	}
	validatedClass, err := NewClIDType(class)
	if err != nil {
		return nil, ErrInvalidPremiumClass
	}

	// Validate currency
	validatedCurrency := money.GetCurrency(s.ToUpper(currency))
	if validatedCurrency == nil {
		return nil, ErrUnknownCurrency
	}

	return &PremiumLabel{
		Label:              validatedLabel,
		PremiumListName:    listName,
		RegistrationAmount: registrationAmount,
		RenewalAmount:      renewalAmount,
		TransferAmount:     transferAmount,
		RestoreAmount:      restoreAmount,
		Currency:           validatedCurrency.Code,
		Class:              validatedClass.String(),
	}, nil
}
