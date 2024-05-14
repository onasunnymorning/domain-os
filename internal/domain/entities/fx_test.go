package entities

import (
	"fmt"
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
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.fx.Convert(tc.from)
			require.ErrorIs(t, err, tc.err)
			if err == nil {
				equal, err := tc.to.Equals(result)
				require.NoError(t, err)
				fmt.Println(result.Display())
				fmt.Println(tc.to.Display())
				require.True(t, equal)
			}
		})
	}
}
