package entities

import (
	"errors"
	s "strings"

	"github.com/Rhymond/go-money"
)

var (
	ErrInvalidPremiumClass = errors.New("invalid premium class")
)

// PremiumLabel represents a premium label entity
type PremiumLabel struct {
	Label              Label  `json:"label"`
	RegistrationAmount uint64 `json:"registrationAmount"`
	RenewalAmount      uint64 `json:"renewalAmount"`
	TransferAmount     uint64 `json:"transferAmount"`
	RestoreAmount      uint64 `json:"restoreAmount"`
	Currency           string `json:"currency"`
	Class              string `json:"class"`
}

// NewPremiumLabel creates a new PremiumLabel instance. It validates the currency, label and class (class string must be a valid label too).
func NewPremiumLabel(label Label, registrationAmount, renewalAmount, transferAmount, restoreAmount uint64, currency, class string) (*PremiumLabel, error) {
	validatedLabel := Label(label)
	if err := validatedLabel.Validate(); err != nil {
		return nil, err
	}
	validatedClass := Label(class)
	if err := validatedClass.Validate(); err != nil {
		return nil, errors.Join(ErrInvalidPremiumClass, err)
	}

	// Validate currency
	validatedCurrency := money.GetCurrency(s.ToUpper(currency))
	if validatedCurrency == nil {
		return nil, ErrUnknownCurrency
	}

	return &PremiumLabel{
		Label:              validatedLabel,
		RegistrationAmount: registrationAmount,
		RenewalAmount:      renewalAmount,
		TransferAmount:     transferAmount,
		RestoreAmount:      restoreAmount,
		Currency:           validatedCurrency.Code,
		Class:              validatedClass.String(),
	}, nil
}
