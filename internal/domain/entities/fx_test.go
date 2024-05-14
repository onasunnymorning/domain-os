package entities

import (
	"testing"

	"github.com/Rhymond/go-money"
	"github.com/stretchr/testify/require"
)

func TestFX_Convert(t *testing.T) {
	testcases := []struct {
		name string
		fx   *FX
		from *money.Money
		to   *money.Money
		err  error
	}{
		{
			name: "USD to EUR",
			fx: &FX{
				From: "USD",
				To:   "EUR",
				Rate: 0.88,
			},
			from: money.New(10000, "USD"),
			to:   money.New(8800, "EUR"),
			err:  nil,
		},
		{
			name: "USD to EUR large",
			fx: &FX{
				From: "USD",
				To:   "EUR",
				Rate: 0.92884123,
			},
			from: money.New(100000000, "USD"),
			to:   money.New(92884123, "EUR"),
			err:  nil,
		},
		{
			name: "USD to EUR small",
			fx: &FX{
				From: "USD",
				To:   "EUR",
				Rate: 0.92884123,
			},
			from: money.New(2, "USD"),
			to:   money.New(1, "EUR"),
			err:  nil,
		},
		{
			name: "USD to EUR very small with correction",
			fx: &FX{
				From: "USD",
				To:   "EUR",
				Rate: 0.92884123,
			},
			from: money.New(1, "USD"),
			to:   money.New(1, "EUR"),
			err:  nil,
		},
		{
			name: "currency mismatch",
			fx: &FX{
				From: "USD",
				To:   "EUR",
				Rate: 0.92884123,
			},
			from: money.New(100, "PEN"),
			to:   money.New(100, "EUR"),
			err:  ErrFXConversion,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.fx.Convert(tc.from)
			require.ErrorIs(t, err, tc.err)
			if err == nil {
				equal, err := tc.to.Equals(result)
				require.NoError(t, err)
				if !equal {
					t.Logf("expected: %s, got: %s", tc.to.Display(), result.Display())
				}
				require.True(t, equal)
			}
		})
	}
}
