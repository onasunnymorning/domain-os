package entities

import (
	"testing"
	"time"

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
