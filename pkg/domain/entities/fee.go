package entities

import (
	s "strings"

	"errors"

	"github.com/Rhymond/go-money"
)

var (
	ErrInvalidFee = errors.New("invalid fee")
)

// Fee represents a fee structure with details about the currency, name, amount,
// refundability, and associated phase ID.
// Fees currenlty apply to ALL transactions (also see price engine) we will make this configurable see https://github.com/onasunnymorning/domain-os/issues/227
type Fee struct {
	// Currency is the ISO 4217 currency code (e.g. USD)
	Currency string `json:"currency" binding:"required" example:"USD"`
	// Name is the name of the fee, for convenience, it is adherent to the ClIDType
	Name ClIDType `json:"name" binding:"required" example:"sunrise_fee"`
	// Amount is the amount of the fee in the smallest unit of the currency (e.g. cents for USD)
	Amount uint64 `json:"amount" binding:"required" example:"10000"`
	// Refundable is a pointer to a bool to allow for nil values, which are interpreted as false.
	// Its intent it to flag if the fees are subject to refunds if the transaction is reversed within the grace period.
	Refundable *bool `json:"refundable" binding:"required" example:"false"`
	// PhaseID is the ID of the phase this fee is associated with
	PhaseID int64 `json:"-"`
}

// Fee factory. Validates the currency and returns a new Fee object. Amounts are stored in the smallest unit of the currency (e.g. cents for USD). Then name is normalized.
func NewFee(cur, name string, amount uint64, refundable *bool) (*Fee, error) {
	// Validate currency
	currency := money.GetCurrency(s.ToUpper(cur))
	if currency == nil {
		return nil, ErrUnknownCurrency
	}
	validatedName, err := NewClIDType(name)
	if err != nil {
		return nil, err
	}
	return &Fee{
		Currency:   currency.Code,
		Name:       ClIDType(validatedName),
		Amount:     amount,
		Refundable: refundable,
	}, nil
}

// GetMoney returns a money.Money object for that fee
func (f *Fee) GetMoney() *money.Money {
	return money.New(int64(f.Amount), f.Currency)
}
