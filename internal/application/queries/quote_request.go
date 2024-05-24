package queries

import (
	"errors"
	"slices"

	"github.com/Rhymond/go-money"
	"github.com/onasunnymorning/domain-os/internal/domain/entities"
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
	if _, err := entities.NewDomainName(qr.DomainName); err != nil {
		return errors.Join(entities.ErrInvalidQuoteRequest, err)
	}
	if !slices.Contains(entities.ValidTransactionTypes, qr.TransactionType) {
		return errors.Join(entities.ErrInvalidQuoteRequest, entities.ErrInvalidTransactionType)
	}
	cur := money.GetCurrency(qr.Currency)
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
