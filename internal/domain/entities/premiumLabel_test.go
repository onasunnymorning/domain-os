package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPremiumLabel(t *testing.T) {
	labelString := "example"
	registrationAmount := uint64(100)
	renewalAmount := uint64(50)
	transferAmount := uint64(20)
	restoreAmount := uint64(30)
	currency := "USD"
	class := "premium"
	listName := "example-list"

	pl, err := NewPremiumLabel(labelString, registrationAmount, renewalAmount, transferAmount, restoreAmount, currency, class, listName)
	require.NoError(t, err, "Failed to create PremiumLabel")

	require.Equal(t, labelString, pl.Label.String(), "Label mismatch")
	require.Equal(t, registrationAmount, pl.RegistrationAmount, "RegistrationAmount mismatch")
	require.Equal(t, renewalAmount, pl.RenewalAmount, "RenewalAmount mismatch")
	require.Equal(t, transferAmount, pl.TransferAmount, "TransferAmount mismatch")
	require.Equal(t, restoreAmount, pl.RestoreAmount, "RestoreAmount mismatch")
	require.Equal(t, currency, pl.Currency, "Currency mismatch")
	require.Equal(t, class, pl.Class, "Class mismatch")
	require.Equal(t, listName, pl.PremiumListName, "ListName mismatch")
}

func TestNewPremiumLabel_InvalidLabel(t *testing.T) {
	labelString := "inva--lid"
	registrationAmount := uint64(100)
	renewalAmount := uint64(50)
	transferAmount := uint64(20)
	restoreAmount := uint64(30)
	currency := "USD"
	class := "premium"
	listName := "example-list"

	_, err := NewPremiumLabel(labelString, registrationAmount, renewalAmount, transferAmount, restoreAmount, currency, class, listName)
	require.Error(t, err, "Expected error for invalid label")
	require.Contains(t, err.Error(), "invalid label", "Error message mismatch")
}

func TestNewPremiumLabel_InvalidClass(t *testing.T) {
	labelString := "example"
	registrationAmount := uint64(100)
	renewalAmount := uint64(50)
	transferAmount := uint64(20)
	restoreAmount := uint64(30)
	currency := "USD"
	class := "thisisnotavalidclidtype"
	listName := "example-list"

	_, err := NewPremiumLabel(labelString, registrationAmount, renewalAmount, transferAmount, restoreAmount, currency, class, listName)
	require.Error(t, err, "Expected error for invalid class")
	require.Contains(t, err.Error(), "invalid premium class", "Error message mismatch")
}

func TestNewPremiumLabel_UnknownCurrency(t *testing.T) {
	labelString := "example"
	registrationAmount := uint64(100)
	renewalAmount := uint64(50)
	transferAmount := uint64(20)
	restoreAmount := uint64(30)
	currency := "XYZ"
	class := "premium"
	listName := "example-list"

	_, err := NewPremiumLabel(labelString, registrationAmount, renewalAmount, transferAmount, restoreAmount, currency, class, listName)
	require.Error(t, err, "Expected error for unknown currency")
	require.Contains(t, err.Error(), "unknown currency", "Error message mismatch")
}
func TestPremiumLabel_GetMoney_Registration(t *testing.T) {
	pl := &PremiumLabel{
		RegistrationAmount: 100,
		Currency:           "USD",
	}

	m, err := pl.GetMoney("registration")
	require.NoError(t, err, "Failed to get money for registration")
	require.Equal(t, int64(100), m.Amount(), "Amount mismatch")
	require.Equal(t, "USD", m.Currency().Code, "Currency mismatch")
}

func TestPremiumLabel_GetMoney_Renewal(t *testing.T) {
	pl := &PremiumLabel{
		RenewalAmount: 50,
		Currency:      "USD",
	}

	m, err := pl.GetMoney("renewal")
	require.NoError(t, err, "Failed to get money for renewal")
	require.Equal(t, int64(50), m.Amount(), "Amount mismatch")
	require.Equal(t, "USD", m.Currency().Code, "Currency mismatch")
}

func TestPremiumLabel_GetMoney_Transfer(t *testing.T) {
	pl := &PremiumLabel{
		TransferAmount: 20,
		Currency:       "USD",
	}

	m, err := pl.GetMoney("transfer")
	require.NoError(t, err, "Failed to get money for transfer")
	require.Equal(t, int64(20), m.Amount(), "Amount mismatch")
	require.Equal(t, "USD", m.Currency().Code, "Currency mismatch")
}

func TestPremiumLabel_GetMoney_Restore(t *testing.T) {
	pl := &PremiumLabel{
		RestoreAmount: 30,
		Currency:      "USD",
	}

	m, err := pl.GetMoney("restore")
	require.NoError(t, err, "Failed to get money for restore")
	require.Equal(t, int64(30), m.Amount(), "Amount mismatch")
	require.Equal(t, "USD", m.Currency().Code, "Currency mismatch")
}

func TestPremiumLabel_GetMoney_InvalidTransactionType(t *testing.T) {
	pl := &PremiumLabel{
		Currency: "USD",
	}

	_, err := pl.GetMoney("invalid")
	require.Error(t, err, "Expected error for invalid transaction type")
	require.Contains(t, err.Error(), "invalid transaction type", "Error message mismatch")
}