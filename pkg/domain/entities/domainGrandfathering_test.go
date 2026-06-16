package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewDomainGrandFathering(t *testing.T) {
	amount := uint64(100)
	currency := "USD"
	expiryCondition := "transfer"
	expiryDate := time.Now().AddDate(1, 0, 0)

	grandFathering, err := NewDomainGrandFathering(amount, currency, expiryCondition, &expiryDate)
	require.NoError(t, err, "Failed to create DomainGrandFathering")

	require.Equal(t, amount, grandFathering.GFAmount, "Amount mismatch")
	require.Equal(t, currency, grandFathering.GFCurrency, "Currency mismatch")
	require.Equal(t, expiryCondition, grandFathering.GFExpiryCondition, "ExpiryCondition mismatch")
	require.Equal(t, &expiryDate, grandFathering.GFVoidDate, "ExpiryDate mismatch")
}

func TestNewDomainGrandFathering_InvalidExpiryCondition(t *testing.T) {
	amount := uint64(100)
	currency := "USD"
	expiryCondition := "invalid_condition"
	expiryDate := time.Now().AddDate(1, 0, 0)

	grandFathering, err := NewDomainGrandFathering(amount, currency, expiryCondition, &expiryDate)
	require.Error(t, err, "Expected an error for invalid expiry condition")
	require.Nil(t, grandFathering, "Expected grandFathering to be nil for invalid expiry condition")
	require.ErrorIs(t, err, ErrInvalidGFExpiryCondition, "Expected ErrInvalidGFExpiryCondition error")
}
