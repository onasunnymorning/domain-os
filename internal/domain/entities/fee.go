package entities

import (
	s "strings"

	"github.com/Rhymond/go-money"
)

// Fee value object. Amounts are stored in the smallest unit of the currency (e.g. cents for USD)
type Fee struct {
	Currency   string `json:"currency" binding:"required" example:"USD"`
	Name       string `json:"name" binding:"required" example:"sunrise fee"`
	Amount     int64  `json:"amount" binding:"required" example:"10000"`
	Refundable *bool  `json:"refundable" binding:"required" example:"false"`
	PhaseID    int64  `json:"phaseid"`
}

// Fee factory. Validates the currency and returns a new Fee object. Amounts are stored in the smallest unit of the currency (e.g. cents for USD). Then name is normalized.
func NewFee(cur, name string, amount int64, refundable *bool) (*Fee, error) {
	// Validate currency
	currency := money.GetCurrency(s.ToUpper(cur))
	if currency == nil {
		return nil, ErrUnknownCurrency
	}
	return &Fee{
		Currency:   currency.Code,
		Name:       NormalizeString(name),
		Amount:     amount,
		Refundable: refundable,
	}, nil
}
