package entities

import (
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"github.com/Rhymond/go-money"
)

var (
	ErrFXConversion = errors.New("currency conversion error")
)

// FX represents a foreign exchange rate
type FX struct {
	Date time.Time
	From string
	To   string
	Rate float64
}

// Convert converts an amount from the 'from' currency to the 'to' currency using the FX rate
func (fx *FX) Convert(source *money.Money) (*money.Money, error) {
	if source.Currency().Code != fx.From {
		return nil, errors.Join(ErrFXConversion, fmt.Errorf("input from currency (%s) does not match FX from currency (%s) ", source.Currency().Code, fx.From))
	}
	// rate := money.NewFromFloat(fx.Rate, fx.From)                             // this ensures the amount is multiplied as an int64 to avoid floating point errors. The rate might get rounded if the rate has many decimal places
	// destination := money.New(rate.Multiply(source.Amount()).Amount(), fx.To) // this returns a new Money object with the result of the conversion
	// return destination, nil

	s := big.NewFloat(source.AsMajorUnits())
	r := big.NewFloat(fx.Rate)
	var d big.Float
	d.Mul(s, r)
	f, err := strconv.ParseFloat(d.String(), 64)
	if err != nil {
		return nil, err
	}
	destination := money.NewFromFloat(f, fx.To)
	return destination, nil
}
