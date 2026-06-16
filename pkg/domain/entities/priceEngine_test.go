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

func TestAddPhasePrice(t *testing.T) {
	testcases := []struct {
		name          string
		domain        Domain
		phase         Phase
		fx            FX
		pl            []*PremiumLabel
		quoteRequest  QuoteRequest
		expectedPrice int64
	}{
		{
			name: "No Phase Price",
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
			name: "Phase Price in target currency",
			domain: Domain{
				Name: "example.com",
			},
			phase: Phase{
				Name: "GA",
				Prices: []Price{
					{
						RegistrationAmount: 100,
						RenewalAmount:      100,
						Currency:           "EUR",
					},
				},
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
			name: "Phase Price in base currency",
			domain: Domain{
				Name: "example.com",
			},
			phase: Phase{
				Name: "GA",
				Policy: PhasePolicy{
					BaseCurrency: "USD",
				},
				Prices: []Price{
					{
						RegistrationAmount: 100,
						RenewalAmount:      100,
						Currency:           "USD",
					},
				},
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
		{
			name: "Phase Price in an odd currency",
			domain: Domain{
				Name: "example.com",
			},
			phase: Phase{
				Name: "GA",
				Policy: PhasePolicy{
					BaseCurrency: "USD",
				},
				Prices: []Price{
					{
						RegistrationAmount: 100,
						RenewalAmount:      100,
						Currency:           "PEN",
					},
				},
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
			expectedPrice: 0,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			priceEngine := NewPriceEngine(tc.phase, tc.domain, tc.fx, tc.pl)
			priceEngine.QuoteRequest = tc.quoteRequest
			var err error
			priceEngine.Quote, err = NewQuoteFromQuoteRequest(tc.quoteRequest)
			priceEngine.setQuoteParams()
			require.NoError(t, err, "Error creating Quote")
			priceEngine.addPhasePrice()
			require.Equal(t, tc.expectedPrice, priceEngine.Quote.Price.Amount(), "Price is not correct")
		})
	}
}

func TestGetQuote(t *testing.T) {
	tr := true
	testcases := []struct {
		name          string
		domain        Domain
		phase         Phase
		fx            FX
		pl            []*PremiumLabel
		quoteRequest  QuoteRequest
		expectedQuote Quote
		expectedError error
	}{
		{
			name: "No Price",
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
				TransactionType: TransactionTypeRegistration,
				Currency:        "EUR",
			},
			expectedQuote: Quote{
				DomainName: "example.com",
				Years:      1,
				Clid:       ClIDType("123456789"),
				Price:      money.New(0, "EUR"),
				Phase:      &Phase{Name: "GA"},
				Fees:       nil,
				FXRate:     &FX{},
				Class:      "standard",
			},
			expectedError: nil,
		},
		{
			name: "invalid phase name",
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
				TransactionType: TransactionTypeRegistration,
				Currency:        "EUR",
				PhaseName:       "bullocks",
			},
			expectedQuote: Quote{
				DomainName: "example.com",
				Years:      1,
				Clid:       ClIDType("123456789"),
				Price:      money.New(0, "EUR"),
				Phase:      &Phase{Name: "GA"},
				Fees:       nil,
				FXRate:     &FX{},
				Class:      "standard",
			},
			expectedError: ErrInvalidPhaseName,
		},
		{
			name: "invalid domain name - quote request",
			domain: Domain{
				Name: "example.com",
			},
			phase: Phase{
				Name: "GA",
			},
			fx: FX{},
			pl: []*PremiumLabel{},
			quoteRequest: QuoteRequest{
				DomainName:      "-example.com",
				Years:           1,
				ClID:            "123456789",
				TransactionType: TransactionTypeRegistration,
				Currency:        "EUR",
			},
			expectedQuote: Quote{
				DomainName: "example.com",
				Years:      1,
				Clid:       ClIDType("123456789"),
				Price:      money.New(0, "EUR"),
				Phase:      &Phase{Name: "GA"},
				Fees:       nil,
				FXRate:     &FX{},
				Class:      "standard",
			},
			expectedError: ErrInvalidQuoteRequest,
		},
		{
			name: "just fees",
			domain: Domain{
				Name: "example.com",
			},
			phase: Phase{
				Name: "GA",
				Fees: []Fee{
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
				},
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
			expectedQuote: Quote{
				DomainName: "example.com",
				Years:      1,
				Clid:       ClIDType("123456789"),
				Price:      money.New(1100, "EUR"),
				Phase:      &Phase{Name: "GA"},
				Fees: []*Fee{
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
				},
				FXRate: &FX{},
				Class:  "standard",
			},
			expectedError: nil,
		},
		{
			name: "fees with grandfathering",
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
				Fees: []Fee{
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
				},
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
			expectedQuote: Quote{
				DomainName: "example.com",
				Years:      1,
				Clid:       ClIDType("123456789"),
				Price:      money.New(1200, "EUR"),
				Phase:      &Phase{Name: "GA"},
				Fees: []*Fee{
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
					{
						Name:       "grandfathered renewal fee",
						Amount:     100,
						Currency:   "EUR",
						Refundable: &tr,
					},
				},
				FXRate: &FX{},
				Class:  "standard",
			},
			expectedError: nil,
		},
		{
			name: "fees with premiums",
			domain: Domain{
				Name: "example.com",
			},
			phase: Phase{
				Name: "GA",
				Fees: []Fee{
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
				},
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
				TransactionType: TransactionTypeRegistration,
				Currency:        "EUR",
			},
			expectedQuote: Quote{
				DomainName: "example.com",
				Years:      1,
				Clid:       ClIDType("123456789"),
				Price:      money.New(1200, "EUR"),
				Phase:      &Phase{Name: "GA"},
				Fees: []*Fee{
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
					{
						Name:       "registration fee",
						Amount:     100,
						Currency:   "EUR",
						Refundable: &tr,
					},
				},
				FXRate: &FX{},
				Class:  "lowPremium",
			},
			expectedError: nil,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			priceEngine := NewPriceEngine(tc.phase, tc.domain, tc.fx, tc.pl)
			quote, err := priceEngine.GetQuote(tc.quoteRequest)
			require.ErrorIs(t, err, tc.expectedError, "Error is not correct")
			if tc.expectedError == nil {
				require.Equal(t, tc.expectedQuote.DomainName, quote.DomainName, "DomainName is not correct")
				require.Equal(t, tc.expectedQuote.Years, quote.Years, "Years are not correct")
				require.Equal(t, tc.expectedQuote.Clid, quote.Clid, "Clid is not correct")
				require.Equal(t, tc.expectedQuote.Price, quote.Price, "Price is not correct")
				require.Equal(t, &tc.phase, quote.Phase, "Phase is not correct")
				require.Equal(t, len(tc.expectedQuote.Fees), len(quote.Fees), "Fees are not correct")
				require.Equal(t, tc.expectedQuote.FXRate, quote.FXRate, "FxRate is not correct")
				require.Equal(t, tc.expectedQuote.Class, quote.Class, "Class is not correct")
			}
		})
	}

}
