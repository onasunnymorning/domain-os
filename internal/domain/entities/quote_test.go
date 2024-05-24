package entities

import (
	"testing"
	"time"

	"github.com/Rhymond/go-money"
	"github.com/stretchr/testify/require"
)

func TestNewQuote(t *testing.T) {
	quote := NewQuote("USD")

	// Verify that the TimeStamp is set to the current time
	require.WithinDuration(t, time.Now().UTC(), quote.TimeStamp, time.Second, "TimeStamp is not set correctly")

	// Verify that the Currency is set correctly
	require.Equal(t, "USD", quote.Price.Currency().Code, "Currency is not set correctly")
}
func TestNewQuoteFromQuoteRequest(t *testing.T) {
	qr := QuoteRequest{
		DomainName:      "example.com",
		TransactionType: "registration",
		Years:           1,
		Currency:        "USD",
		ClID:            "123456789",
	}

	quote, err := NewQuoteFromQuoteRequest(qr)

	require.NoError(t, err, "Error should be nil")

	require.Equal(t, DomainName("example.com"), quote.DomainName, "DomainName is not set correctly")
	require.Equal(t, "registration", quote.TransactionType, "TransactionType is not set correctly")
	require.Equal(t, 1, quote.Years, "Years is not set correctly")
	require.Equal(t, "standard", quote.Class, "Class is not set correctly")
	require.Equal(t, money.New(0, "USD"), quote.Price, "Price is not set correctly")
	require.Equal(t, ClIDType("123456789"), quote.Clid, "Clid is not set correctly")
}

func TestNewQuoteFromQuoteRequest_ValidationFail(t *testing.T) {
	qr := QuoteRequest{
		DomainName:      "exa--mple.com",
		TransactionType: "registration",
		Years:           1,
		Currency:        "USD",
		ClID:            "123456789",
	}

	quote, err := NewQuoteFromQuoteRequest(qr)

	require.Error(t, err)
	require.Nil(t, quote)
}
func TestAddFee(t *testing.T) {
	testcases := []struct {
		name      string
		quote     *Quote
		fee       *Fee
		yearlyFee bool
	}{
		{
			name: "Both USD",
			quote: &Quote{
				Price: money.New(0, "USD"),
			},
			fee: &Fee{
				Amount:   10000,
				Currency: "USD",
			},
			yearlyFee: false,
		},
		{
			name: "Both USD 2 years not yearly",
			quote: &Quote{
				Price: money.New(0, "USD"),
				Years: 2,
			},
			fee: &Fee{
				Amount:   10000,
				Currency: "USD",
			},
			yearlyFee: false,
		},
		{
			name: "Both USD 2 years yearly",
			quote: &Quote{
				Price: money.New(0, "USD"),
				Years: 2,
			},
			fee: &Fee{
				Amount:   10000,
				Currency: "USD",
			},
			yearlyFee: true,
		},
		{
			name: "Convert multi year",
			quote: &Quote{
				Price: money.New(0, "EUR"),
				Years: 2,
				FXRate: &FX{
					BaseCurrency:   "USD",
					TargetCurrency: "EUR",
					Rate:           0.8,
				},
			},
			fee: &Fee{
				Amount:   10000,
				Currency: "USD",
			},
			yearlyFee: true,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.quote.AddFeeAndUpdatePrice(tc.fee, tc.yearlyFee)
			require.NoError(t, err, "Error should be nil")
			if tc.yearlyFee {
				require.Len(t, tc.quote.Fees, tc.quote.Years)
				if tc.quote.FXRate == nil {
					require.Equal(t, money.New(int64(tc.fee.Amount*uint64(tc.quote.Years)), tc.quote.Price.Currency().Code), tc.quote.Price, "Price is not updated correctly")
				} else {
					require.NotEqual(t, money.New(int64(tc.fee.Amount*uint64(tc.quote.Years)), tc.quote.Price.Currency().Code), tc.quote.Price, "Price is not updated correctly")
				}
			} else {
				require.Len(t, tc.quote.Fees, 1)
				if tc.quote.FXRate == nil {
					require.Equal(t, money.New(int64(tc.fee.Amount), tc.quote.Price.Currency().Code), tc.quote.Price, "Price is not updated correctly")
				} else {
					require.NotEqual(t, money.New(int64(tc.fee.Amount), tc.quote.Price.Currency().Code), tc.quote.Price, "Price is not updated correctly")
				}
			}
		})

	}
}
