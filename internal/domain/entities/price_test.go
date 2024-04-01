package entities

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPrice(t *testing.T) {
	price, err := NewPrice("usd", 100, 100, 100, 100)

	assert.Nil(t, err)
	assert.Equal(t, "USD", price.Currency)
	assert.Equal(t, int64(100), price.RegistrationAmount)
	assert.Equal(t, int64(100), price.RenewalAmount)
	assert.Equal(t, int64(100), price.TransferAmount)
	assert.Equal(t, int64(100), price.RestoreAmount)
}

func TestNewPrice_InvalidCurrency(t *testing.T) {
	price, err := NewPrice("xxx", 100, 100, 100, 100)

	assert.Nil(t, price)
	assert.Equal(t, ErrUnknownCurrency, err)
}
