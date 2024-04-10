package entities

import (
	"testing"

	assert "github.com/stretchr/testify/assert"
)

func TestNewFee(t *testing.T) {
	fee, err := NewFee("usd", "sunrise fee", 10000, nil)

	assert.Nil(t, err)
	assert.Equal(t, "USD", fee.Currency)
	assert.Equal(t, "sunrise fee", fee.Name.String())
	assert.Equal(t, int64(10000), fee.Amount)
	assert.Nil(t, fee.Refundable)
}

func TestNewFee_InvalidCurrency(t *testing.T) {
	fee, err := NewFee("xxx", "sunrise fee", 10000, nil)

	assert.Nil(t, fee)
	assert.Equal(t, ErrUnknownCurrency, err)
}

func TestNewFee_InvalidName(t *testing.T) {
	fee, err := NewFee("usd", "sunrïse fee", 10000, nil)

	assert.Nil(t, fee)
	assert.Equal(t, ErrInvalidClIDType, err)
}
