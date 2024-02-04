package entities

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestClIDType_String(t *testing.T) {
	tests := []struct {
		name     string
		clIDType ClIDType
		want     string
	}{
		{
			name:     "test case 1",
			clIDType: "example",
			want:     "example",
		},
		{
			name:     "test case 2",
			clIDType: "test",
			want:     "test",
		},
		// Add more test cases as needed
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.clIDType.String()
			require.Equal(t, test.want, got)
		})
	}
}

func TestNewClIDType(t *testing.T) {
	tests := []struct {
		name    string
		clID    string
		want    ClIDType
		wantErr error
	}{
		{
			name:    "Vali",
			clID:    "Example clid",
			want:    "Example clid",
			wantErr: nil,
		},
		{
			name:    "too short",
			clID:    "te",
			want:    ClIDType(""),
			wantErr: ErrInvalidClIDType,
		},
		{
			name:    "too long",
			clID:    "this is tooooooooooo looooooooooong",
			want:    ClIDType(""),
			wantErr: ErrInvalidClIDType,
		},
		{
			name:    "Nonn ACII",
			clID:    "ïnørrçt",
			want:    ClIDType(""),
			wantErr: ErrInvalidClIDType,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := NewClIDType(test.clID)
			require.Equal(t, test.want, got, fmt.Sprintf("ClIDType mismatch for test: '%s'", test.name))
			require.Equal(t, test.wantErr, err, fmt.Sprintf("Error mismatch for test: '%s'", test.name))
		})
	}
}
