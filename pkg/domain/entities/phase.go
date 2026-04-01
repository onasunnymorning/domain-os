package entities

import (
	"errors"
	"strings"
	"time"

	"github.com/Rhymond/go-money"
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

// Phase entity Ref:https://centralnic.atlassian.net/wiki/spaces/REG/pages/5023629498/DOS+-+TLD+Phases
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

// IsCurrentlyActive checks if the phase is currently active.
// A phase is active during the interval [Starts, Ends) - inclusive start, exclusive end.
// This means:
//   - A request at exactly Starts time belongs to this phase
//   - A request at exactly Ends time belongs to the NEXT phase
//
// This allows consecutive phases to touch at boundaries without gaps or overlaps.
// Example: Phase1 [2025-10-01 00:00:00, 2025-10-03 00:00:00) followed by
//
//	Phase2 [2025-10-03 00:00:00, 2025-10-05 00:00:00) creates perfect continuity.
func (p *Phase) IsCurrentlyActive() bool {
	now := time.Now().UTC()
	// Active if: now >= Starts AND (no end OR now < Ends)
	return !now.Before(p.Starts) && (p.Ends == nil || now.Before(*p.Ends))
}

// OverlapsWith checks if the phase overlaps with another phase.
// This is intended to be used for GA phases; launch phases may overlap.
//
// With [inclusive start, exclusive end) semantics, phases can touch at boundaries without overlap.
// Example: Phase1 [2025-10-01, 2025-10-03) and Phase2 [2025-10-03, 2025-10-05) do NOT overlap.
//
// Two phases overlap if there exists any point in time where both would be active.
// Using interval notation [A, B) and [C, D):
//   - They overlap if: A < D AND C < B
//   - They touch (no overlap) if: B == C OR D == A
func (p *Phase) OverlapsWith(other *Phase) bool {
	// if both phases have no end date, they overlap (both extend indefinitely)
	if p.Ends == nil && other.Ends == nil {
		return true
	}

	// if this phase has no end date: [Starts, ∞)
	if p.Ends == nil {
		// Overlaps if other starts before this phase ends (which is never)
		// So only overlaps if other starts at or after this phase starts
		return !other.Starts.Before(p.Starts)
	}

	// if the other phase has no end date: [Starts, ∞)
	if other.Ends == nil {
		// Overlaps if this starts before other phase ends (which is never)
		// So only overlaps if this starts at or after other phase starts
		return !p.Starts.Before(other.Starts)
	}

	// if both phases have an end date: [A, B) and [C, D)
	// They overlap if: A < D AND C < B
	// This means there's at least one moment where both phases are active
	return p.Starts.Before(*other.Ends) && other.Starts.Before(*p.Ends)
}

// IsContinuousWith checks if this phase is continuous with another phase.
// Returns true if one phase ends exactly when the other starts (no gap, no overlap).
// With [inclusive start, exclusive end) semantics, phases touch when:
//   - This phase ends at the exact moment the other starts, OR
//   - The other phase ends at the exact moment this starts
//
// Example: Phase1 [2025-10-01, 2025-10-03) is continuous with Phase2 [2025-10-03, 2025-10-05)
func (p *Phase) IsContinuousWith(other *Phase) bool {
	// Can't be continuous if either phase has no end date
	if p.Ends == nil || other.Ends == nil {
		return false
	}

	// Check if this ends when other starts, or vice versa
	return p.Ends.Equal(other.Starts) || other.Ends.Equal(p.Starts)
}

// SuggestNextPhaseStart suggests when the next phase should start to maintain continuity.
// Returns the exact timestamp where this phase ends, which is when the next phase should start.
// Returns zero time if this phase has no end date (can't suggest next start for indefinite phases).
//
// Example: If this phase ends at 2025-10-03 00:00:00 UTC, the next phase should start
// at exactly 2025-10-03 00:00:00 UTC to maintain continuity with no gaps or overlaps.
func (p *Phase) SuggestNextPhaseStart() time.Time {
	if p.Ends == nil {
		return time.Time{} // Can't suggest if no end date
	}
	// With [inclusive start, exclusive end) semantics, next phase starts exactly when this ends
	return *p.Ends
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

// GetTransactionPriceAsMoney retrieves the monetary value for a specific transaction type
// in the specified target currency. If no price is available for the target currency,
// it attempts to retrieve the price in the base currency and then convert it.
// Returns a money.Money value or an error if retrieval, conversion, or lookup fails.
func (p *Phase) GetTransactionPriceAsMoney(targetCurrency string, transactionType TransactionType, fx FX) (*money.Money, error) {
	price, err := p.GetPrice(targetCurrency)
	if err != nil {
		if !errors.Is(err, ErrPriceNotFound) {
			// if we get an error that is not ErrPriceNotFound, we return it
			return nil, err
		}
		// If the price is not found, we try to get the price in the base currency
		price, err = p.GetPrice(fx.BaseCurrency)
		if err != nil {
			return nil, err
		}
		pm, err := price.GetMoney(transactionType)
		if err != nil {
			return nil, err
		}
		// We return it converted to the target currency
		return fx.Convert(pm)
	}
	// if we had a price in the target currency, we return it
	return price.GetMoney(transactionType)
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

// CanUpdate checks if the phase can be updated. A phase can be updated if it has not ended yet.
func (p *Phase) CanUpdate() (bool, error) {
	if p.Ends == nil {
		return true, nil
	}
	if p.Ends.Before(time.Now().UTC()) {
		return false, ErrUpdateHistoricPhase
	}
	return true, nil
}
