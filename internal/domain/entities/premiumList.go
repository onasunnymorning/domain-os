package entities

import (
	"errors"
)

var (
	ErrInvalidPremiumListName = errors.New("invalid premium list name")
)

// PremiumList represents a premium list entity
type PremiumList struct {
	Name          string          `json:"name"`
	PremiumLabels []*PremiumLabel `json:"premiumLabels"`
	CreatedAt     string          `json:"createdAt"`
	UpdatedAt     string          `json:"updatedAt"`
}

// NewPremiumList creates a new PremiumList instance
func NewPremiumList(name string) (*PremiumList, error) {
	validatedName := Label(name)
	if err := validatedName.Validate(); err != nil {
		return nil, errors.Join(ErrInvalidPremiumListName, err)
	}
	return &PremiumList{
		Name:          string(validatedName),
		PremiumLabels: []*PremiumLabel{},
	}, nil
}
