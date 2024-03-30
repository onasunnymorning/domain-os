package entities

import (
	"errors"
	"time"
)

var (
	ErrInvalidPhaseName    = errors.New("invalid phase name")
	ErrInvalidPhaseType    = errors.New("invalid phase type")
	ErrDuplicatePriceEntry = errors.New("Price entry for this currency already exists")
	ErrDuplicateFeeEntry   = errors.New("Fee entry with this name and currency already exists")
	ErrEndDateBeforeStart  = errors.New("end date is before start date")
	ErrEndDateInPast       = errors.New("end date is in the past")
	ErrEndDateAlreadySet   = errors.New("end date already set")
)

const (
	PhaseTypeGA     PhaseType = "GA"
	PhaseTypeLaunch PhaseType = "Launch"
)

// PhasetType is a custom type for representing the type of a phase.
type PhaseType string

// TLD Phase entity
type Phase struct {
	ID              int64      `json:"id"`
	Name            ClIDType   `json:"name"`
	Type            PhaseType  `json:"type"`
	Starts          time.Time  `json:"starts"`
	Ends            *time.Time `json:"ends"`
	Prices          []Price    `json:"prices"`
	Fees            []Fee      `json:"fees"`
	PremiumListName string     `json:"premiumListName"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
	TLDName         string     `json:"tldName"`
	PhasePolicy
}

// Phase factory. Phase name is of type ClIDType and phaseType is a string (GA or Launch)
func NewPhase(name, phaseType string, start time.Time) (*Phase, error) {
	// Validate the phase type
	if phaseType != string(PhaseTypeGA) && phaseType != string(PhaseTypeLaunch) {
		return nil, ErrInvalidPhaseType
	}
	// TODO: Validate phase name
	validatedName, err := NewClIDType(name)
	if err != nil {
		return nil, errors.Join(ErrInvalidPhaseName, err)
	}
	start = start.UTC()
	new_phase := &Phase{
		Name:        validatedName,
		Type:        PhaseType(phaseType),
		Starts:      start,
		PhasePolicy: NewPhasePolicy(),
	}
	return new_phase, nil
}

// Add a fee to the phase
func (p *Phase) AddFee(f Fee) (int, error) {
	err := p.checkFeeExists(f)
	if err != nil {
		return 0, err
	}
	newIndex := len(p.Fees)
	p.Fees = append(p.Fees, f)
	return newIndex, nil
}

// There can be multiple fees for a phase but not with the same name (name = reason)
func (p *Phase) checkFeeExists(pr Fee) error {
	for i := 0; i < len(p.Fees); i++ {
		if p.Fees[i].Currency == pr.Currency && p.Fees[i].Name == pr.Name {
			return ErrDuplicateFeeEntry
		}
	}
	return nil
}

// Add a price to the phase
func (p *Phase) AddPrice(pr Price) (int, error) {
	err := p.checkPriceExists(pr)
	if err != nil {
		return 0, err
	}
	newIndex := len(p.Prices)
	p.Prices = append(p.Prices, pr)
	return newIndex, nil
}

// Only one pricepoint per currency in any given phase
func (p *Phase) checkPriceExists(pr Price) error {
	for i := 0; i < len(p.Prices); i++ {
		if p.Prices[i].Currency == pr.Currency {
			return ErrDuplicatePriceEntry
		}
	}
	return nil
}

// Set an enddate to a phase
func (p *Phase) SetEnd(e time.Time) error {
	end := e.UTC()
	if p.Ends != nil {
		// TODO: Updating the End date of a phase is tricky because we need to check the new end date will not cause an overlap with another phase
		return ErrEndDateAlreadySet
	}
	if end.Before(p.Starts) {
		return ErrEndDateBeforeStart
	}
	if end.Before(time.Now().UTC()) {
		return ErrEndDateInPast
	}
	p.Ends = &end
	return nil
}
