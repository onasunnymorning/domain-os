package entities

import (
	"errors"
	"slices"
	"time"
)

const (
	GFConditionTransfer = "transfer"
	GFConditionDelete   = "delete"
	GFConditionDate     = "date"
)

var (
	// ErrInvalidExpiryCondition is an error that is returned when the expiry condition is invalid
	ErrInvalidGFExpiryCondition = errors.New("invalid expiry condition - must be transfer, delete or date")
	ValidGFExpiryConditions     = []string{GFConditionTransfer, GFConditionDelete, GFConditionDate}
)

// DomainGrandFathering is a struct that represents the grandfathering conditions of a domain
type DomainGrandFathering struct {
	GFAmount          uint64     `json:"Amount"`
	GFCurrency        string     `json:"Currency"`
	GFExpiryCondition string     `json:"ExpiryCondition"` // transfer (GF will be void on transfer), delete (GF will be void only when the domain is deleted), expiry_date (will exipre on a specific date GFEpiryDate)
	GFVoidDate        *time.Time `json:"VoidDate"`        // if nil it will never expire
}

// NewDomainGrandFathering is a constructor for DomainGrandFathering
func NewDomainGrandFathering(amount uint64, currency string, expiryCondition string, expiryDate *time.Time) (*DomainGrandFathering, error) {
	if !slices.Contains(ValidGFExpiryConditions, expiryCondition) {
		return nil, ErrInvalidGFExpiryCondition
	}
	return &DomainGrandFathering{
		GFAmount:          amount,
		GFCurrency:        currency,
		GFExpiryCondition: expiryCondition,
		GFVoidDate:        expiryDate,
	}, nil
}
