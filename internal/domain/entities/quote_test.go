package entities

import (
	"testing"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/stretchr/testify/require"
)

func TestNewQuote(t *testing.T) {
	quote := NewQuote()

	// Verify that the TimeStamp is set to the current time
	require.WithinDuration(t, time.Now().UTC(), quote.TimeStamp, time.Second, "TimeStamp is not set correctly")

	// Verify that the Price is nil
	require.Nil(t, quote.Price, "Price is not nil")

	// Verify that the Fees slice is empty
	require.Empty(t, quote.Fees, "Fees is not empty")

	// Verify that the FXRate is nil
	require.Nil(t, quote.FXRate, "FXRate is not nil")

	// Verify that the DomainName is empty
	require.Empty(t, quote.DomainName, "DomainName is not empty")

	// Verify that the TransactionType is empty
	require.Empty(t, quote.TransactionType, "TransactionType is not empty")

	// Verify that the Phase is nil
	require.Nil(t, quote.Phase, "Phase is not nil")

	// Verify that the Clid is empty
	require.Empty(t, quote.Clid, "Clid is not empty")
}
func TestNewQuoteFromQuoteRequest(t *testing.T) {
	qr := QuoteRequest{
		DomainName:      "example.com",
		TransactionType: "purchase",
		Years:           1,
		Currency:        "USD",
		ClID:            "123456789",
	}

	quote, err := NewQuoteFromQuoteRequest(qr)

	require.NoError(t, err, "Error should be nil")

	require.Equal(t, DomainName("example.com"), quote.DomainName, "DomainName is not set correctly")
	require.Equal(t, "purchase", quote.TransactionType, "TransactionType is not set correctly")
	require.Equal(t, 1, quote.Years, "Years is not set correctly")
	require.Equal(t, "standard", quote.Class, "Class is not set correctly")
	require.Equal(t, money.New(0, "USD"), quote.Price, "Price is not set correctly")
	require.Equal(t, ClIDType("123456789"), quote.Clid, "Clid is not set correctly")
}
func TestAddFee(t *testing.T) {
	// Create a new quote
	quote := NewQuote()

	// Create a fee
	fee := &Fee{
		Amount:   100,
		Currency: "USD",
	}

	// Add the fee to the quote
	err := quote.AddFeeAndUpdatePrice(fee, false)

	// Verify that the fee is added successfully
	require.NoError(t, err, "Error should be nil")
	require.Len(t, quote.Fees, 1, "Fees slice should have one element")

	// Verify that the total price is updated correctly
	expectedPrice := money.New(100, "USD")
	require.Equal(t, expectedPrice, quote.Price, "Price is not updated correctly")
}
