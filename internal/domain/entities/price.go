package entities

import (
	s "strings"

	"errors"

	money "github.com/Rhymond/go-money"
)

var (
	ErrInvalidPrice = errors.New("invalid price")
)

// Price value object. Amounts are stored in the smallest unit of the currency (e.g. cents for USD)
type Price struct {
	Currency           string `json:"currency"  binding:"required" example:"USD"`
	RegistrationAmount uint64 `json:"registrationAmount"  binding:"required" example:"1000"`
	RenewalAmount      uint64 `json:"renewalAmount"  binding:"required" example:"1000"`
	TransferAmount     uint64 `json:"transferAmount"  binding:"required" example:"1000"`
	RestoreAmount      uint64 `json:"restoreAmount"  binding:"required" example:"1000"`
	PhaseID            int64  `json:"phaseid"`
}

// Price factory. Validates the currency and returns a new Price object. Amounts are stored in the smallest unit of the currency (e.g. cents for USD)
func NewPrice(cur string, reg, ren, tr, res uint64) (*Price, error) {
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

// GetMoney returns a money.Money object for the given transaction type
func (p *Price) GetMoney(transactionType TransactionType) (*money.Money, error) {
	var amount uint64
	switch transactionType {
	case TransactionTypeRegistration:
		amount = p.RegistrationAmount
	case TransactionTypeRenewal:
		amount = p.RenewalAmount
	case TransactionTypeAutoRenewal:
		amount = p.RenewalAmount
	case TransactionTypeTransfer:
		amount = p.TransferAmount
	case TransactionTypeRestore:
		amount = p.RestoreAmount
	default:
		return nil, ErrInvalidTransactionTypeForQuote
	}

	return money.New(int64(amount), p.Currency), nil
}
