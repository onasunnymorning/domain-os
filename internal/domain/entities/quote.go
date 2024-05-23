package entities

import (
	"time"

	"github.com/Rhymond/go-money"
)

// Quote represents a quote for a specific transaction on the system.
type Quote struct {
	TimeStamp       time.Time
	Price           *money.Money
	Fees            []*Fee
	FXRate          *FX
	DomainName      DomainName
	Years           int
	TransactionType string
	Phase           *Phase
	Clid            ClIDType
	Class           string
}

// NewQuote creates a new Quote.
func NewQuote() *Quote {
	return &Quote{
		TimeStamp: time.Now().UTC(),
	}
}
