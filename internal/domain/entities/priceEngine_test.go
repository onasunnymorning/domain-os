package entities

import (
	"testing"

	"github.com/Rhymond/go-money"
	"github.com/stretchr/testify/require"
)

func TestNewPriceEngine(t *testing.T) {
	phase := Phase{Name: "GA"}
	domain := Domain{Name: "example.com"}
	fx := FX{}
	pl := []*PremiumLabel{}

	pe := NewPriceEngine(phase, domain, fx, pl)
	require.NotNil(t, pe, "PriceEngine is nil")
}
func TestSetQuoteParams(t *testing.T) {
	phase := Phase{Name: "GA"}
	domain := Domain{Name: "example.com"}
	fx := FX{
		BaseCurrency:   "USD",
		TargetCurrency: "EUR",
		Rate:           0.8,
	}
	pl := []*PremiumLabel{}
	pe := NewPriceEngine(phase, domain, fx, pl)
	q := &Quote{}
	pe.Quote = q
	pe.setQuoteParams()
	require.Equal(t, fx.BaseCurrency, q.FXRate.BaseCurrency, "FXRate is not set correctly")
	require.Equal(t, fx.TargetCurrency, q.FXRate.TargetCurrency, "FXRate is not set correctly")
	require.Equal(t, domain.Name, q.DomainName, "DomainName is not set correctly")
	require.Equal(t, &phase, q.Phase, "Phase is not set correctly")
}
func TestAddPhaseFees(t *testing.T) {
	priceEngine := NewPriceEngine(Phase{Name: "GA", Policy: PhasePolicy{BaseCurrency: "USD"}}, Domain{Name: "example.com"}, FX{}, []*PremiumLabel{})

	// Testcase: no Phase fees
	err := priceEngine.addPhaseFees()
	require.NoError(t, err, "Error adding Phase fees")
	require.Equal(t, 0, len(priceEngine.Quote.Fees), "Phase fees should be empty")

	// Testcase: Phase fees in target currency
	priceEngine.Phase.Fees = []Fee{
		{
			Name:     "Sunrise Fee",
			Amount:   100,
			Currency: "EUR",
		},
		{
			Name:     "Verification Fee",
			Amount:   1000,
			Currency: "EUR",
		},
	}

	priceEngine.QuoteRequest.Currency = "EUR"
	priceEngine.Quote.Price = money.New(0, "EUR")
	err = priceEngine.addPhaseFees()
	require.NoError(t, err, "Error adding Phase fees")
	require.Equal(t, 2, len(priceEngine.Quote.Fees), "Phase fees should be 2")
	for _, fee := range priceEngine.Quote.Fees {
		require.Equal(t, "EUR", fee.Currency, "Fee currency is not correct")
	}
	require.Equal(t, int64(1100), priceEngine.Quote.Price.Amount(), "Price is not correct")
	require.Equal(t, "EUR", priceEngine.Quote.Price.Currency().Code, "Price currency is not correct")

	// Testcase Phase fees in base currency
	priceEngine = NewPriceEngine(Phase{Name: "GA", Policy: PhasePolicy{BaseCurrency: "USD"}}, Domain{Name: "example.com"}, FX{}, []*PremiumLabel{})

	priceEngine.Phase.Fees = []Fee{
		{
			Name:     "Sunrise Fee",
			Amount:   100,
			Currency: "USD",
		},
		{
			Name:     "Verification Fee",
			Amount:   1000,
			Currency: "USD",
		},
	}

	priceEngine.QuoteRequest.Currency = "EUR"
	priceEngine.Quote.Price = money.New(0, "EUR")
	priceEngine.FXRate = FX{
		BaseCurrency:   "USD",
		TargetCurrency: "EUR",
		Rate:           0.8,
	}
	priceEngine.Quote.FXRate = &priceEngine.FXRate
	err = priceEngine.addPhaseFees()
	require.NoError(t, err, "Error adding Phase fees")
	require.Equal(t, 2, len(priceEngine.Quote.Fees), "Phase fees should be 2")
	for _, fee := range priceEngine.Quote.Fees {
		require.Equal(t, "USD", fee.Currency, "Fee currency is not correct")
	}
	require.Equal(t, int64(880), priceEngine.Quote.Price.Amount(), "Price is not correct")
	require.Equal(t, "EUR", priceEngine.Quote.Price.Currency().Code, "Price currency is not correct")

}

func TestAddGrandFatheringFees(t *testing.T) {
	testCases := []struct {
		name          string
		domain        Domain
		phase         Phase
		fx            FX
		pl            []*PremiumLabel
		quoteRequest  QuoteRequest
		expectedPrice int64
	}{
		{
			name: "No GrandFathering Fees",
			domain: Domain{
				Name: "example.com",
			},
			phase: Phase{
				Name: "GA",
			},
			fx: FX{},
			pl: []*PremiumLabel{},
			quoteRequest: QuoteRequest{
				DomainName:      "example.com",
				Years:           1,
				ClID:            "123456789",
				TransactionType: TransactionTypeRenewal,
				Currency:        "EUR",
			},
			expectedPrice: 0,
		},
		{
			name: "GF in target currency but quote for registration",
			domain: Domain{
				Name: "example.com",
				GrandFathering: DomainGrandFathering{
					GFAmount:          100,
					GFCurrency:        "EUR",
					GFExpiryCondition: "transfer",
				},
			},
			phase: Phase{
				Name: "GA",
			},
			fx: FX{},
			pl: []*PremiumLabel{},
			quoteRequest: QuoteRequest{
				DomainName:      "example.com",
				Years:           1,
				ClID:            "123456789",
				TransactionType: TransactionTypeRegistration,
				Currency:        "EUR",
			},
			expectedPrice: 0,
		},
		{
			name: "GF in target currency and renewal",
			domain: Domain{
				Name: "example.com",
				GrandFathering: DomainGrandFathering{
					GFAmount:          100,
					GFCurrency:        "EUR",
					GFExpiryCondition: "transfer",
				},
			},
			phase: Phase{
				Name: "GA",
			},
			fx: FX{},
			pl: []*PremiumLabel{},
			quoteRequest: QuoteRequest{
				DomainName:      "example.com",
				Years:           1,
				ClID:            "123456789",
				TransactionType: TransactionTypeRenewal,
				Currency:        "EUR",
			},
			expectedPrice: 100,
		},
		{
			name: "GF in base currency and renewal",
			domain: Domain{
				Name: "example.com",
				GrandFathering: DomainGrandFathering{
					GFAmount:          100,
					GFCurrency:        "USD",
					GFExpiryCondition: "transfer",
				},
			},
			phase: Phase{
				Name: "GA",
			},
			fx: FX{
				BaseCurrency:   "USD",
				TargetCurrency: "EUR",
				Rate:           0.8,
			},
			pl: []*PremiumLabel{},
			quoteRequest: QuoteRequest{
				DomainName:      "example.com",
				Years:           1,
				ClID:            "123456789",
				TransactionType: TransactionTypeRenewal,
				Currency:        "EUR",
			},
			expectedPrice: 80,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			priceEngine := NewPriceEngine(tc.phase, tc.domain, tc.fx, tc.pl)
			priceEngine.QuoteRequest = tc.quoteRequest
			var err error
			priceEngine.Quote, err = NewQuoteFromQuoteRequest(tc.quoteRequest)
			priceEngine.setQuoteParams()
			require.NoError(t, err, "Error creating Quote")
			priceEngine.addGrandFatheringFees()
			require.Equal(t, tc.expectedPrice, priceEngine.Quote.Price.Amount(), "Price is not correct")
			if tc.domain.IsGrandFathered() && tc.quoteRequest.TransactionType == TransactionTypeRenewal {
				require.Equal(t, 1, len(priceEngine.Quote.Fees), "Fees should be 1")
			} else {
				require.Equal(t, 0, len(priceEngine.Quote.Fees), "Fees should be 0")
			}
		})
	}
}

func TestAddPremiumFees(t *testing.T) {
	testCases := []struct {
		name          string
		domain        Domain
		phase         Phase
		fx            FX
		pl            []*PremiumLabel
		quoteRequest  QuoteRequest
		expectedPrice int64
	}{
		{
			name: "No Premium Fees",
			domain: Domain{
				Name: "example.com",
			},
			phase: Phase{
				Name: "GA",
			},
			fx: FX{},
			pl: []*PremiumLabel{},
			quoteRequest: QuoteRequest{
				DomainName:      "example.com",
				Years:           1,
				ClID:            "123456789",
				TransactionType: TransactionTypeRenewal,
				Currency:        "EUR",
			},
			expectedPrice: 0,
		},
		{
			name: "Premiums in Target currency",
			domain: Domain{
				Name: "example.com",
			},
			phase: Phase{
				Name: "GA",
			},
			fx: FX{},
			pl: []*PremiumLabel{
				{
					Label:              "example",
					RegistrationAmount: 100,
					RenewalAmount:      100,
					RestoreAmount:      100,
					TransferAmount:     100,
					Currency:           "EUR",
					Class:              "lowPremium",
				},
			},
			quoteRequest: QuoteRequest{
				DomainName:      "example.com",
				Years:           1,
				ClID:            "123456789",
				TransactionType: TransactionTypeRenewal,
				Currency:        "EUR",
			},
			expectedPrice: 100,
		},
		{
			name: "Premiums in Base currency",
			domain: Domain{
				Name: "example.com",
			},
			phase: Phase{
				Name: "GA",
				Policy: PhasePolicy{
					BaseCurrency: "USD",
				},
			},
			fx: FX{
				BaseCurrency:   "USD",
				TargetCurrency: "EUR",
				Rate:           0.8,
			},
			pl: []*PremiumLabel{
				{
					Label:              "example",
					RegistrationAmount: 100,
					RenewalAmount:      100,
					RestoreAmount:      100,
					TransferAmount:     100,
					Currency:           "USD",
					Class:              "lowPremium",
				},
			},
			quoteRequest: QuoteRequest{
				DomainName:      "example.com",
				Years:           1,
				ClID:            "123456789",
				TransactionType: TransactionTypeRenewal,
				Currency:        "EUR",
			},
			expectedPrice: 80,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			priceEngine := NewPriceEngine(tc.phase, tc.domain, tc.fx, tc.pl)
			priceEngine.QuoteRequest = tc.quoteRequest
			var err error
			priceEngine.Quote, err = NewQuoteFromQuoteRequest(tc.quoteRequest)
			priceEngine.setQuoteParams()
			require.NoError(t, err, "Error creating Quote")
			priceEngine.addPremiumFees()
			require.Equal(t, tc.expectedPrice, priceEngine.Quote.Price.Amount(), "Price is not correct")
			if len(tc.pl) > 0 {
				require.Equal(t, priceEngine.Quote.Class, tc.pl[0].Class, "Class is not correct")
				require.Equal(t, 1, len(priceEngine.Quote.Fees), "Fees should be 1")
			}
		})
	}
}
