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
	RyID          ClIDType        `json:"ryID"`
	PremiumLabels []*PremiumLabel `json:"premiumLabels"`
	CreatedAt     string          `json:"createdAt"`
	UpdatedAt     string          `json:"updatedAt"`
}

// NewPremiumList creates a new PremiumList instance
func NewPremiumList(name, ryid string) (*PremiumList, error) {
	validatedName := Label(name)
	if err := validatedName.Validate(); err != nil {
		return nil, errors.Join(ErrInvalidPremiumListName, err)
	}
	validatedRyID, err := NewClIDType(ryid)
	if err != nil {
		return nil, err
	}
	return &PremiumList{
		Name:          string(validatedName),
		RyID:          validatedRyID,
		PremiumLabels: []*PremiumLabel{},
	}, nil
}
