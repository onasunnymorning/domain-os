package entities

import (
	"testing"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/stretchr/testify/require"
)

func TestNewQuote(t *testing.T) {
	quote := NewQuote("USD")

	// Verify that the TimeStamp is set to the current time
	require.WithinDuration(t, time.Now().UTC(), quote.TimeStamp, time.Second, "TimeStamp is not set correctly")

	// Verify that the Currency is set correctly
	require.Equal(t, "USD", quote.Price.Currency().Code, "Currency is not set correctly")
}
func TestNewQuoteFromQuoteRequest(t *testing.T) {
	qr := QuoteRequest{
		DomainName:      "example.com",
		TransactionType: "registration",
		Years:           1,
		Currency:        "USD",
		ClID:            "123456789",
	}

	quote, err := NewQuoteFromQuoteRequest(qr)

	require.NoError(t, err, "Error should be nil")

	require.Equal(t, DomainName("example.com"), quote.DomainName, "DomainName is not set correctly")
	require.Equal(t, "registration", quote.TransactionType, "TransactionType is not set correctly")
	require.Equal(t, 1, quote.Years, "Years is not set correctly")
	require.Equal(t, "standard", quote.Class, "Class is not set correctly")
	require.Equal(t, money.New(0, "USD"), quote.Price, "Price is not set correctly")
	require.Equal(t, ClIDType("123456789"), quote.Clid, "Clid is not set correctly")
}
func TestAddFee(t *testing.T) {
	// Create a new quote
	quote := NewQuote("USD")

	// Create a fee
	fee := &Fee{
		Amount:   10000,
		Currency: "USD",
	}

	// Add the fee to the quote
	err := quote.AddFeeAndUpdatePrice(fee, false)

	// Verify that the fee is added successfully
	require.NoError(t, err, "Error should be nil")
	require.Len(t, quote.Fees, 1, "Fees slice should have one element")

	// Verify that the total price is updated correctly
	expectedPrice := money.New(10000, "USD")
	require.Equal(t, expectedPrice, quote.Price, "Price is not updated correctly")
}
