package entities

import (
	"errors"
	"testing"
)

func TestDomainLifeCycleEvent_GenerateSKU(t *testing.T) {
	tests := []struct {
		name          string
		event         DomainLifeCycleEvent
		expectedSKU   string
		expectedError error
	}{
		{
			name: "Empty TldName returns ErrEmptyTldName",
			event: DomainLifeCycleEvent{
				TldName:         "",
				TransactionType: TransactionTypeRegistration,
				DomainYears:     1,
			},
			expectedSKU:   "",
			expectedError: ErrEmptyTldName,
		},
		{
			name: "Empty TransactionType returns ErrEmptyTransactionType",
			event: DomainLifeCycleEvent{
				TldName:         "com",
				TransactionType: TransactionType(""),
				DomainYears:     1,
			},
			expectedSKU:   "",
			expectedError: ErrEmptyTransactionType,
		},
		{
			name: "Valid input sets SKU correctly",
			event: DomainLifeCycleEvent{
				TldName:         "com",
				TransactionType: TransactionTypeRegistration,
				DomainYears:     2,
			},
			expectedSKU:   "COM-REGISTRATION-2Y",
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.GenerateSKU()
			if !errors.Is(err, tt.expectedError) {
				t.Errorf("expected error %v, got %v", tt.expectedError, err)
			}
			if tt.expectedSKU != tt.event.SKU {
				t.Errorf("expected SKU %s, got %s", tt.expectedSKU, tt.event.SKU)
			}
		})
	}
}

func TestNewDomainLifeCycleEvent(t *testing.T) {
	tests := []struct {
		name        string
		clientID    string
		resellerID  string
		tldName     string
		domainName  string
		domainYears int
		transaction TransactionType
		wantErr     error
	}{
		{
			name:       "Empty clientID returns ErrEmptyClientID",
			clientID:   "",
			domainName: "example.com",
			wantErr:    ErrEmptyClientID,
		},
		{
			name:       "Empty clientID returns ErrEmptyClientID",
			clientID:   "myclientid",
			domainName: "example.com",
			wantErr:    ErrEmptyTldName,
		},
		{
			name:       "Empty domainName returns ErrEmptyDomainName",
			clientID:   "1234",
			domainName: "",
			wantErr:    ErrEmptyDomainName,
		},
		{
			name:        "Valid input returns no error",
			clientID:    "1234",
			resellerID:  "5678",
			tldName:     "com",
			domainName:  "example.com",
			domainYears: 2,
			transaction: TransactionTypeRegistration,
			wantErr:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dle, err := NewDomainLifeCycleEvent(
				tt.clientID,
				tt.resellerID,
				tt.tldName,
				tt.domainName,
				tt.domainYears,
				tt.transaction,
			)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("expected error %v, got %v", tt.wantErr, err)
			}
			if err == nil && dle == nil {
				t.Errorf("expected non-nil DomainLifeCycleEvent, got nil")
			}
			if dle != nil && dle.TimeStamp.IsZero() {
				t.Errorf("expected TimeStamp to be set, got zero value")
			}
		})
	}
}
