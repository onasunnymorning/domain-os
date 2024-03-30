package entities

import (
	s "strings"

	money "github.com/Rhymond/go-money"
)

// Price value object. Amounts are stored in the smallest unit of the currency (e.g. cents for USD)
type Price struct {
	Currency           string `json:"currency"  binding:"required" example:"USD"`
	RegistrationAmount int64  `json:"registrationAmount"  binding:"required" example:"1000"`
	RenewalAmount      int64  `json:"renewalAmount"  binding:"required" example:"1000"`
	TransferAmount     int64  `json:"transferAmount"  binding:"required" example:"1000"`
	RestoreAmount      int64  `json:"restoreAmount"  binding:"required" example:"1000"`
	PhaseID            int64  `json:"phaseid"`
}

// Price factory. Validates the currency and returns a new Price object. Amounts are stored in the smallest unit of the currency (e.g. cents for USD)
func NewPrice(cur string, reg, ren, tr, res int64) (*Price, error) {
	// Validate currency
	currency := money.GetCurrency(s.ToUpper(cur))
	if currency == nil {
		return nil, ErrUnknownCurrency
	}

	return &Price{
		Currency:           currency.Code,
		RegistrationAmount: reg,
		RenewalAmount:      ren,
		TransferAmount:     tr,
		RestoreAmount:      res,
	}, nil
}
