package entities

import (
	"errors"
	"slices"
	"strings"

	"github.com/Rhymond/go-money"
)

var (
	ErrInvalidQuoteRequest  = errors.New("invalid quote request")
	ErrInvalidNumberOfYears = errors.New("invalid number of years")
)

// QuoteRequest represents a query to get a quote for a domain name.
type QuoteRequest struct {
	DomainName      string `json:"DomainName" binding:"required"`
	TransactionType string `json:"TransactionType" binding:"required"`
	Currency        string `json:"Currency" binding:"required"`
	Years           int    `json:"Years" binding:"required"`
	ClID            string `json:"ClID" binding:"required"`
	PhaseName       string `json:"PhaseName"` // Phase name - if empty the current GA phase is assumed
}

// Validate validates the QuoteRequest.
func (qr *QuoteRequest) Validate() error {
	if _, err := NewDomainName(qr.DomainName); err != nil {
		return errors.Join(ErrInvalidQuoteRequest, err)
	}
	if !slices.Contains(ValidTransactionTypes, qr.TransactionType) {
		return errors.Join(ErrInvalidQuoteRequest, ErrInvalidTransactionType)
	}
	cur := money.GetCurrency(strings.ToUpper(qr.Currency))
	if cur == nil {
		return errors.Join(ErrInvalidQuoteRequest, ErrUnknownCurrency)
	}
	if qr.Years < 1 || qr.Years > MaxHorizon {
		return errors.Join(ErrInvalidQuoteRequest, ErrInvalidNumberOfYears)
	}
	if _, err := NewClIDType(qr.ClID); err != nil {
		return errors.Join(ErrInvalidQuoteRequest, err)
	}
	return nil
}
