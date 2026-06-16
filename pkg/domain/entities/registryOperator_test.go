package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRegistryOperator(t *testing.T) {
	testcases := []struct {
		name    string
		ryID    string
		email   string
		wantErr error
	}{
		{
			name:    "valid registry operator",
			ryID:    "ry-operator",
			email:   "my@me.com",
			wantErr: nil,
		},
		{
			name:    "invalid email",
			ryID:    "ry-operator",
			email:   "invalid-email",
			wantErr: ErrInvalidEmail,
		},
		{
			name:    "invalid ryID operator",
			ryID:    "invalid-ryIDinvalid-ryIDinvalid-ryIDinvalid-ryID",
			email:   "my@me.com",
			wantErr: ErrInvalidClIDType,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewRegistryOperator(tc.ryID, "name", tc.email)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}

}
