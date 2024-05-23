package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPrice(t *testing.T) {
	price, err := NewPrice("usd", 100, 100, 100, 100)

	assert.Nil(t, err)
	assert.Equal(t, "USD", price.Currency)
	assert.Equal(t, uint64(100), price.RegistrationAmount)
	assert.Equal(t, uint64(100), price.RenewalAmount)
	assert.Equal(t, uint64(100), price.TransferAmount)
	assert.Equal(t, uint64(100), price.RestoreAmount)
}

func TestNewPrice_InvalidCurrency(t *testing.T) {
	price, err := NewPrice("xxx", 100, 100, 100, 100)

	assert.Nil(t, price)
	assert.Equal(t, ErrUnknownCurrency, err)
}
func TestPrice_GetMoney_Registration(t *testing.T) {
	price := &Price{
		Currency:           "USD",
		RegistrationAmount: 100,
		RenewalAmount:      200,
		TransferAmount:     300,
		RestoreAmount:      400,
	}

	m, err := price.GetMoney("registration")

	assert.Nil(t, err)
	assert.Equal(t, int64(100), m.Amount())
	assert.Equal(t, "USD", m.Currency().Code)
}

func TestPrice_GetMoney_Renewal(t *testing.T) {
	price := &Price{
		Currency:           "USD",
		RegistrationAmount: 100,
		RenewalAmount:      200,
		TransferAmount:     300,
		RestoreAmount:      400,
	}

	m, err := price.GetMoney("renewal")

	assert.Nil(t, err)
	assert.Equal(t, int64(200), m.Amount())
	assert.Equal(t, "USD", m.Currency().Code)
}

func TestPrice_GetMoney_Transfer(t *testing.T) {
	price := &Price{
		Currency:           "USD",
		RegistrationAmount: 100,
		RenewalAmount:      200,
		TransferAmount:     300,
		RestoreAmount:      400,
	}

	m, err := price.GetMoney("transfer")

	assert.Nil(t, err)
	assert.Equal(t, int64(300), m.Amount())
	assert.Equal(t, "USD", m.Currency().Code)
}

func TestPrice_GetMoney_Restore(t *testing.T) {
	price := &Price{
		Currency:           "USD",
		RegistrationAmount: 100,
		RenewalAmount:      200,
		TransferAmount:     300,
		RestoreAmount:      400,
	}

	m, err := price.GetMoney("restore")

	assert.Nil(t, err)
	assert.Equal(t, int64(400), m.Amount())
	assert.Equal(t, "USD", m.Currency().Code)
}

func TestPrice_GetMoney_InvalidTransactionType(t *testing.T) {
	price := &Price{
		Currency:           "USD",
		RegistrationAmount: 100,
		RenewalAmount:      200,
		TransferAmount:     300,
		RestoreAmount:      400,
	}

	_, err := price.GetMoney("invalid")

	assert.Equal(t, ErrInvalidTransactionType, err)
}