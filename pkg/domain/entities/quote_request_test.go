package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestQuoteRequest_Validate(t *testing.T) {
	testcases := []struct {
		name     string
		request  QuoteRequest
		expected error
	}{
		{
			name: "valid request",
			request: QuoteRequest{
				DomainName:      "example.com",
				TransactionType: TransactionTypeRegistration,
				Currency:        "USD",
				Years:           1,
				ClID:            "testRegistrar1",
			},
			expected: nil,
		},
		{
			name: "invalid DomainName",
			request: QuoteRequest{
				DomainName:      "-example.com",
				TransactionType: TransactionTypeRegistration,
				Currency:        "USD",
				Years:           1,
				ClID:            "testRegistrar1",
			},
			expected: ErrInvalidLabelDash,
		},
		{
			name: "invalid TransactionType",
			request: QuoteRequest{
				DomainName:      "example.com",
				TransactionType: "mutation",
				Currency:        "USD",
				Years:           1,
				ClID:            "testRegistrar1",
			},
			expected: ErrInvalidTransactionTypeForQuote,
		},
		{
			name: "invalid Currency",
			request: QuoteRequest{
				DomainName:      "example.com",
				TransactionType: TransactionTypeRegistration,
				Currency:        "PPP",
				Years:           1,
				ClID:            "testRegistrar1",
			},
			expected: ErrUnknownCurrency,
		},
		{
			name: "invalid Years",
			request: QuoteRequest{
				DomainName:      "example.com",
				TransactionType: TransactionTypeRegistration,
				Currency:        "USD",
				Years:           11,
				ClID:            "testRegistrar1",
			},
			expected: ErrInvalidNumberOfYears,
		},
		{
			name: "invalid Clid",
			request: QuoteRequest{
				DomainName:      "example.com",
				TransactionType: TransactionTypeRegistration,
				Currency:        "USD",
				Years:           1,
				ClID:            "testRegistrar11234234",
			},
			expected: ErrInvalidClIDType,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.request.Validate()
			require.ErrorIs(t, err, tc.expected)
		})
	}
}
