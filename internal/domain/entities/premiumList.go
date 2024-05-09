package entities

import (
	"errors"
	"time"
)

var (
	ErrInvalidPremiumListName = errors.New("invalid premium list name")
	ErrPremiumListNotFound    = errors.New("premium list not found")
)

// PremiumList represents a premium list entity
type PremiumList struct {
	Name          string          `json:"Name"`
	RyID          ClIDType        `json:"RyID"`
	PremiumLabels []*PremiumLabel `json:"PremiumLabels,omitempty"`
	CreatedAt     time.Time       `json:"CreatedAt"`
	UpdatedAt     time.Time       `json:"UpdatedAt"`
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
		CreatedAt:     RoundTime(time.Now().UTC()),
		UpdatedAt:     RoundTime(time.Now().UTC()),
	}, nil
}
