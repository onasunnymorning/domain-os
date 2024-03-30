package entities

import (
	s "strings"

	money "github.com/Rhymond/go-money"
)

// Price value object. Amounts are stored in the smallest unit of the currency (e.g. cents for USD)
type Price struct {
	Currency     string `json:"currency"  binding:"required" example:"USD"`
	Registration int64  `json:"registration"  binding:"required" example:"1000"`
	Renewal      int64  `json:"renewal"  binding:"required" example:"1000"`
	Transfer     int64  `json:"transfer"  binding:"required" example:"1000"`
	Restore      int64  `json:"restore"  binding:"required" example:"1000"`
	PhaseID      int64  `json:"phaseid"`
}

// Price factory. Validates the currency and returns a new Price object. Amounts are stored in the smallest unit of the currency (e.g. cents for USD)
func NewPrice(cur string, reg, ren, tr, res int64) (*Price, error) {
	// Validate currency
	currency := money.GetCurrency(s.ToUpper(cur))
	if currency == nil {
		return nil, ErrUnknownCurrency
	}

	return &Price{
		Currency:     currency.Code,
		Registration: reg,
		Renewal:      ren,
		Transfer:     tr,
		Restore:      res,
	}, nil
}
