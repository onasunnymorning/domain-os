package queries

import (
	"errors"
	"slices"
	"strings"

	"github.com/Rhymond/go-money"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
)

// QuoteRequest represents a query to get a quote for a domain name.
type QuoteRequest struct {
	DomainName      string `json:"DomainName" binding:"required" example:"get.busy"`
	TransactionType string `json:"TransactionType" binding:"required" example:"registration"`
	Currency        string `json:"Currency" binding:"required"  example:"USD"`
	Years           int    `json:"Years" binding:"required" example:"2"`
	ClID            string `json:"ClID" binding:"required"  example:"1290-RiskNames"`
	PhaseName       string `json:"PhaseName" example:"sunrise"` // Phase name - if empty the current GA phase is assumed
}

// Validate validates the QuoteRequest.
func (qr *QuoteRequest) Validate() error {
	if _, err := entities.NewDomainName(qr.DomainName); err != nil {
		return errors.Join(entities.ErrInvalidQuoteRequest, err)
	}
	if !slices.Contains(entities.ValidTransactionTypes, qr.TransactionType) {
		return errors.Join(entities.ErrInvalidQuoteRequest, entities.ErrInvalidTransactionType)
	}
	cur := money.GetCurrency(strings.ToUpper(qr.Currency))
	if cur == nil {
		return errors.Join(entities.ErrInvalidQuoteRequest, entities.ErrUnknownCurrency)
	}
	if qr.Years < 1 || qr.Years > entities.MaxHorizon {
		return errors.Join(entities.ErrInvalidQuoteRequest, entities.ErrInvalidNumberOfYears)
	}
	if _, err := entities.NewClIDType(qr.ClID); err != nil {
		return errors.Join(entities.ErrInvalidQuoteRequest, err)
	}
	return nil
}

// ToEntity converts a QuoteRequest to a QuoteRequest entity.
func (qr *QuoteRequest) ToEntity() *entities.QuoteRequest {
	return &entities.QuoteRequest{
		DomainName:      qr.DomainName,
		TransactionType: qr.TransactionType,
		Currency:        qr.Currency,
		Years:           qr.Years,
		ClID:            qr.ClID,
		PhaseName:       qr.PhaseName,
	}
}
