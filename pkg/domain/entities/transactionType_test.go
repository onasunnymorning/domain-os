package entities

import (
	"testing"
)

func TestTransactionType_String(t *testing.T) {
	tests := []struct {
		name string
		tt   TransactionType
		want string
	}{
		{"registration", TransactionTypeRegistration, "registration"},
		{"renewal", TransactionTypeRenewal, "renewal"},
		{"transfer", TransactionTypeTransfer, "transfer"},
		{"restore", TransactionTypeRestore, "restore"},
		{"delete", TransactionTypeDelete, "delete"},
		{"info", TransactionTypeInfo, "info"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.tt.String()
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
