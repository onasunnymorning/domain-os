package entities

import (
	"time"

	"github.com/Rhymond/go-money"
)

// Quote represents a quote for a specific transaction on the system.
type Quote struct {
	TimeStamp       time.Time
	DomainName      DomainName
	TransactionType string
	Clid            ClIDType
	Years           int
	Price           *money.Money
	Class           string
	Fees            []*Fee
	FXRate          *FX
	Phase           *Phase // `json:"-"`
}

// NewQuote creates a new Quote.
func NewQuote(currency string) *Quote {
	return &Quote{
		TimeStamp: time.Now().UTC(),
		Price:     money.New(0, currency),
	}
}

// NewQuoteFromQuoteRequest creates a new Quote from a QuoteRequest.
func NewQuoteFromQuoteRequest(qr QuoteRequest) (*Quote, error) {
	if err := qr.Validate(); err != nil {
		return nil, err
	}
	q := NewQuote(qr.Currency)
	q.DomainName = DomainName(qr.DomainName)
	q.TransactionType = qr.TransactionType
	q.Years = qr.Years
	q.Class = "standard"
	q.Clid = ClIDType(qr.ClID)
	return q, nil
}

// AddFeeAndUpdatePrice adds a fee to the quote and update the total price
func (q *Quote) AddFeeAndUpdatePrice(fee *Fee, yearlyFee bool) error {
	q.Fees = append(q.Fees, fee)
	feeMoney := money.New(int64(fee.Amount), fee.Currency)
	// if the currency matches no need to convert the currency
	if feeMoney.Currency() == q.Price.Currency() {
		// Multiply the fee by the number of years if it is a yearly fee
		if yearlyFee {
			feeMoney = feeMoney.Multiply(int64(q.Years))
		}
		// Add the fee in matching currency to the total price
		var err error
		q.Price, err = q.Price.Add(feeMoney)
		if err != nil {
			return err
		}
		// If it is a yearly fee, add the fee to the fees slice as many times as the number of years
		if yearlyFee {
			for i := 1; i < q.Years; i++ {
				q.Fees = append(q.Fees, fee)
			}
		}
		return nil
	}
	// convert the currency
	convertedFeeMoney, err := q.FXRate.Convert(feeMoney)
	if err != nil {
		return err
	}
	// Multiply the fee by the number of years if it is a yearly fee
	if yearlyFee {
		convertedFeeMoney = convertedFeeMoney.Multiply(int64(q.Years))
	}
	// Add the fee in matching currency to the total price
	q.Price, _ = q.Price.Add(convertedFeeMoney)
	// If it is a yearly fee, add the fee to the fees slice as many times as the number of years
	if yearlyFee {
		for i := 1; i < q.Years; i++ {
			q.Fees = append(q.Fees, fee)
		}
	}
	return nil
}
