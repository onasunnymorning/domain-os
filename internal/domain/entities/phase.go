package entities

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrInvalidPhase        = errors.New("invalid phase")
	ErrInvalidPhaseName    = errors.New("invalid phase name")
	ErrInvalidPhaseType    = errors.New("invalid phase type")
	ErrDuplicatePriceEntry = errors.New("Price entry for this currency already exists")
	ErrDuplicateFeeEntry   = errors.New("Fee entry with this name and currency already exists")
	ErrEndDateBeforeStart  = errors.New("end date is before start date")
	ErrEndDateInPast       = errors.New("end date is in the past")
	ErrPriceNotFound       = errors.New("price not found")
)

const (
	PhaseTypeGA     PhaseType = "GA"
	PhaseTypeLaunch PhaseType = "Launch"
)

// PhasetType is a custom type for representing the type of a phase.
type PhaseType string

// Phase entity
type Phase struct {
	ID              int64       `json:"id"`
	Name            ClIDType    `json:"name"`
	Type            PhaseType   `json:"type"`
	Starts          time.Time   `json:"starts"`
	Ends            *time.Time  `json:"ends"`
	Prices          []Price     `json:"prices"`
	Fees            []Fee       `json:"fees"`
	PremiumListName *string     `json:"premiumListName"`
	CreatedAt       time.Time   `json:"createdAt"`
	UpdatedAt       time.Time   `json:"updatedAt"`
	TLDName         DomainName  `json:"tldName"`
	Policy          PhasePolicy `json:"policy"`
}

// Phase factory. Phase name is of type ClIDType and phaseType is a string (GA or Launch)
func NewPhase(name, phaseType string, start time.Time) (*Phase, error) {
	// Validate the phase type
	if phaseType != string(PhaseTypeGA) && phaseType != string(PhaseTypeLaunch) {
		return nil, ErrInvalidPhaseType
	}
	// Validate phase name
	validatedName, err := NewClIDType(name)
	if err != nil {
		return nil, errors.Join(ErrInvalidPhaseName, err)
	}
	// Check if the start date is in UTC
	if !IsUTC(start) {
		return nil, ErrTimeStampNotUTC
	}
	new_phase := &Phase{
		Name:   validatedName,
		Type:   PhaseType(phaseType),
		Starts: start,
		Policy: NewPhasePolicy(),
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

// DeleteFee deletes a fee from the phase. We always store currency Codes in uppercase, but this function will also accept lowercase currency codes.
func (p *Phase) DeleteFee(name, currency string) error {
	// If the phase has ended, we should not update it, there is also no need to remove any fees as they are historical
	if p.Ends != nil && p.Ends.Before(time.Now().UTC()) {
		return ErrUpdateHistoricPhase
	}
	for i := 0; i < len(p.Fees); i++ {
		if p.Fees[i].Currency == strings.ToUpper(currency) && p.Fees[i].Name.String() == name {
			p.Fees = append(p.Fees[:i], p.Fees[i+1:]...)
			return nil
		}
	}
	return nil // Fee not found, not an error, be idempotent
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

// DeletePrice deletes a price from the phase. We always store currency Codes in uppercase, but this function will also accept lowercase currency codes.
func (p *Phase) DeletePrice(currency string) error {
	// If the phase has ended, we should not update it, there is also no need to remove any prices as they are historical
	if p.Ends != nil && p.Ends.Before(time.Now().UTC()) {
		return ErrUpdateHistoricPhase
	}
	for i := 0; i < len(p.Prices); i++ {
		if p.Prices[i].Currency == strings.ToUpper(currency) {
			p.Prices = append(p.Prices[:i], p.Prices[i+1:]...)
			return nil
		}
	}
	return nil // Price not found, not an error, be idempotent
}

// SetEnd Sets an enddate to a phase. The enddate must be in the future and after the start date. Returns an error if the enddate is in the past or before the start date.
func (p *Phase) SetEnd(endDate time.Time) error {
	// Check if the end date is in UTC
	if !IsUTC(endDate) {
		return ErrTimeStampNotUTC
	}
	if endDate.Before(p.Starts) {
		return ErrEndDateBeforeStart
	}
	if endDate.Before(time.Now().UTC()) {
		return ErrEndDateInPast
	}
	p.Ends = &endDate
	return nil
}

// IsCurrentlyActive checks if the phase is currently active. A phase is active if the current time is between the start and end date. Or if the end date is nil, the phase is active if the current time is after the start date.
func (p *Phase) IsCurrentlyActive() bool {
	now := time.Now().UTC()
	return p.Starts.Before(now) && (p.Ends == nil || p.Ends.After(now))
}

// OverlapsWith checks if the phase overlaps with the phase that is passed in as an argument. This is intended to be used for GA phases, launch phases may overlap.
func (p *Phase) OverlapsWith(other *Phase) bool {
	// if both phases no end date, they overlap
	if p.Ends == nil && other.Ends == nil {
		return true
	}

	// if this phase has no end date
	if p.Ends == nil {
		// if the other phase starts after this phase, they overlap
		if other.Starts.After(p.Starts) {
			return true
		}
		// if the other phase's end date is not before this phase's start date, they overlap
		if !other.Ends.Before(p.Starts) {
			return true
		}
	}

	// if the other phase has no end date
	if other.Ends == nil {
		// if this phase starts after the other phase, they overlap
		if p.Starts.After(other.Starts) {
			return true
		}
		// if this phase's end date is not before the other phase's start date, they overlap
		if !p.Ends.Before(other.Starts) {
			return true
		}
	}

	// if both phases have an end date
	if p.Ends != nil && other.Ends != nil {
		// if this phase starts first
		if p.Starts.Before(other.Starts) {
			// Then it has to end before the other phase starts, or it overlaps
			if !p.Ends.Before(other.Starts) {
				return true
			}
		}
		// if the other phase starts first
		if other.Starts.Before(p.Starts) {
			// Then it has to end before this phase starts, or it overlaps
			if !other.Ends.Before(p.Starts) {
				return true
			}
		}
	}

	// if none of these conditions are met, the phases do not overlap
	return false

}

// GetPrice returns the price for a given currency
func (p *Phase) GetPrice(currency string) (*Price, error) {
	for i := 0; i < len(p.Prices); i++ {
		if p.Prices[i].Currency == strings.ToUpper(currency) {
			return &p.Prices[i], nil
		}
	}
	return nil, ErrPriceNotFound
}

// GetFees returns the fees for a given currency
func (p *Phase) GetFees(currency string) []Fee {
	var fees []Fee
	for i := 0; i < len(p.Fees); i++ {
		if p.Fees[i].Currency == strings.ToUpper(currency) {
			fees = append(fees, p.Fees[i])
		}
	}
	return fees
}
