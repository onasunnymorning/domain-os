package queries

import (
	"errors"
	"testing"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
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
				TransactionType: entities.TransactionTypeRegistration,
				Currency:        "USD",
				Years:           1,
				ClID:            "testRegistrar1",
			},
			expected: nil,
		},
		{
			name: "invalid DomainName",
			request: QuoteRequest{
				DomainName:      "exa--mple.com",
				TransactionType: entities.TransactionTypeRegistration,
				Currency:        "USD",
				Years:           1,
				ClID:            "testRegistrar1",
			},
			expected: entities.ErrInvalidLabelDoubleDash,
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
			expected: entities.ErrInvalidTransactionType,
		},
		{
			name: "invalid Currency",
			request: QuoteRequest{
				DomainName:      "example.com",
				TransactionType: entities.TransactionTypeRegistration,
				Currency:        "PPP",
				Years:           1,
				ClID:            "testRegistrar1",
			},
			expected: entities.ErrUnknownCurrency,
		},
		{
			name: "invalid Years",
			request: QuoteRequest{
				DomainName:      "example.com",
				TransactionType: entities.TransactionTypeRegistration,
				Currency:        "USD",
				Years:           11,
				ClID:            "testRegistrar1",
			},
			expected: entities.ErrInvalidNumberOfYears,
		},
		{
			name: "invalid Clid",
			request: QuoteRequest{
				DomainName:      "example.com",
				TransactionType: entities.TransactionTypeRegistration,
				Currency:        "USD",
				Years:           1,
				ClID:            "testRegistrar11234234",
			},
			expected: entities.ErrInvalidClIDType,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.request.Validate()
			if !errors.Is(err, tc.expected) {
				t.Errorf("expected error %v, got %v", tc.expected, err)
			}
		})
	}
}

func TestQuoteRequest_ToEntity(t *testing.T) {
	request := QuoteRequest{
		DomainName:      "example.com",
		TransactionType: entities.TransactionTypeRegistration,
		Currency:        "USD",
		Years:           1,
		ClID:            "testRegistrar1",
	}

	entity := request.ToEntity()
	if entity.DomainName != request.DomainName {
		t.Errorf("expected %s, got %s", request.DomainName, entity.DomainName)
	}
	if entity.TransactionType != request.TransactionType {
		t.Errorf("expected %s, got %s", request.TransactionType, entity.TransactionType)
	}
	if entity.Currency != request.Currency {
		t.Errorf("expected %s, got %s", request.Currency, entity.Currency)
	}
	if entity.Years != request.Years {
		t.Errorf("expected %d, got %d", request.Years, entity.Years)
	}
	if entity.ClID != request.ClID {
		t.Errorf("expected %s, got %s", request.ClID, entity.ClID)
	}
}
