package entities

import (
	s "strings"

	"errors"
	"github.com/Rhymond/go-money"
)

var (
	ErrInvalidFee = errors.New("invalid fee")
)

// Fee value object. Amounts are stored in the smallest unit of the currency (e.g. cents for USD)
type Fee struct {
	Currency   string   `json:"currency" binding:"required" example:"USD"`
	Name       ClIDType `json:"name" binding:"required" example:"sunrise_fee"`
	Amount     uint64   `json:"amount" binding:"required" example:"10000"`
	Refundable *bool    `json:"refundable" binding:"required" example:"false"`
	PhaseID    int64    `json:"-"`
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
