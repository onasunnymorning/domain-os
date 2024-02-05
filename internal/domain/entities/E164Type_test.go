package entities

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestE164Type_NewE164Type(t *testing.T) {
	tests := []struct {
		name        string
		phoneNumber string
		want        E164Type
		wantErr     error
	}{
		{
			name:        "valid phone number",
			phoneNumber: "+1.12345678901234",
			want:        E164Type("+1.12345678901234"),
			wantErr:     nil,
		},
		{
			name:        "empty phone number",
			phoneNumber: "",
			want:        E164Type(""),
			wantErr:     nil,
		},
		{
			name:        "missing + sign",
			phoneNumber: "1.12345678901234",
			want:        E164Type(""),
			wantErr:     ErrInvalidE164Type,
		},
		{
			name:        "too many country code digits",
			phoneNumber: "+1234.123456789",
			want:        E164Type(""),
			wantErr:     ErrInvalidE164Type,
		},
		{
			name:        "too many phone number digits",
			phoneNumber: "+123.123456789123456789123456789",
			want:        E164Type(""),
			wantErr:     ErrInvalidE164Type,
		},
	}

	for _, tt := range tests {
		actual, err := NewE164Type(tt.phoneNumber)
		require.Equal(t, tt.wantErr, err, fmt.Sprintf("Error mismatch for test '%s'", tt.name))
		if tt.wantErr == nil {
			require.Equal(t, tt.want, *actual, fmt.Sprintf("Value mismatch for test '%s'", tt.name))
		}
	}
}

func TestE164Type_String(t *testing.T) {
	tests := []struct {
		name string
		e    E164Type
		want string
	}{
		{
			name: "valid phone number",
			e:    E164Type("+1.12345678901234"),
			want: "+1.12345678901234",
		},
		{
			name: "empty phone number",
			e:    E164Type(""),
			want: "",
		},
	}

	for _, tt := range tests {
		require.Equal(t, tt.want, tt.e.String(), fmt.Sprintf("Value mismatch for test '%s'", tt.name))
	}
}
