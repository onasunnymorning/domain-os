package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPremiumLabel(t *testing.T) {
	label := Label("example")
	registrationAmount := uint64(100)
	renewalAmount := uint64(50)
	transferAmount := uint64(20)
	restoreAmount := uint64(30)
	currency := "USD"
	class := "premium"

	pl, err := NewPremiumLabel(label, registrationAmount, renewalAmount, transferAmount, restoreAmount, currency, class)
	require.NoError(t, err, "Failed to create PremiumLabel")

	require.Equal(t, label, pl.Label, "Label mismatch")
	require.Equal(t, registrationAmount, pl.RegistrationAmount, "RegistrationAmount mismatch")
	require.Equal(t, renewalAmount, pl.RenewalAmount, "RenewalAmount mismatch")
	require.Equal(t, transferAmount, pl.TransferAmount, "TransferAmount mismatch")
	require.Equal(t, restoreAmount, pl.RestoreAmount, "RestoreAmount mismatch")
	require.Equal(t, currency, pl.Currency, "Currency mismatch")
	require.Equal(t, class, pl.Class, "Class mismatch")
}

func TestNewPremiumLabel_InvalidLabel(t *testing.T) {
	label := Label("inva--lid")
	registrationAmount := uint64(100)
	renewalAmount := uint64(50)
	transferAmount := uint64(20)
	restoreAmount := uint64(30)
	currency := "USD"
	class := "premium"

	_, err := NewPremiumLabel(label, registrationAmount, renewalAmount, transferAmount, restoreAmount, currency, class)
	require.Error(t, err, "Expected error for invalid label")
	require.Contains(t, err.Error(), "invalid label", "Error message mismatch")
}

func TestNewPremiumLabel_InvalidClass(t *testing.T) {
	label := Label("example")
	registrationAmount := uint64(100)
	renewalAmount := uint64(50)
	transferAmount := uint64(20)
	restoreAmount := uint64(30)
	currency := "USD"
	class := "inva--lid"

	_, err := NewPremiumLabel(label, registrationAmount, renewalAmount, transferAmount, restoreAmount, currency, class)
	require.Error(t, err, "Expected error for invalid class")
	require.Contains(t, err.Error(), "invalid premium class", "Error message mismatch")
}

func TestNewPremiumLabel_UnknownCurrency(t *testing.T) {
	label := Label("example")
	registrationAmount := uint64(100)
	renewalAmount := uint64(50)
	transferAmount := uint64(20)
	restoreAmount := uint64(30)
	currency := "XYZ"
	class := "premium"

	_, err := NewPremiumLabel(label, registrationAmount, renewalAmount, transferAmount, restoreAmount, currency, class)
	require.Error(t, err, "Expected error for unknown currency")
	require.Contains(t, err.Error(), "unknown currency", "Error message mismatch")
}
