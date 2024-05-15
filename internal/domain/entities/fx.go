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
	Date           time.Time
	BaseCurrency   string
	TargetCurrency string
	Rate           float64
}

// Convert converts an amount from the 'from' currency to the 'to' currency using the FX rate
func (fx *FX) Convert(source *money.Money) (*money.Money, error) {
	if source.Currency().Code != fx.BaseCurrency {
		return nil, errors.Join(ErrFXConversion, fmt.Errorf("input from currency (%s) does not match FX from currency (%s) ", source.Currency().Code, fx.BaseCurrency))
	}
	s := big.NewFloat(source.AsMajorUnits())     // Convert our source amount to a big.Float
	r := big.NewFloat(fx.Rate)                   // Convert our rate to a big.Float
	var d big.Float                              // Declare a big.Float to hold the result
	d.Mul(s, r)                                  // Multiply the source amount by the rate
	fmt.Printf("%f x %f = %f\n", s, r, &d)       // Print the calculation
	f, err := strconv.ParseFloat(d.String(), 64) // Convert the result to a float64
	if err != nil {
		return nil, err
	}
	destination := money.NewFromFloat(f, fx.TargetCurrency) // Create a new money.Money instance with the result
	// If the amounts are so small that they round to zero, return a minimum amount of 0.01
	if destination.Amount() == 0 && source.Amount() > 0 {
		return money.NewFromFloat(0.01, fx.TargetCurrency), nil
	}
	return destination, nil
}
