package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPremiumList(t *testing.T) {
	tests := []struct {
		name     string
		listName string
		wantErr  error
	}{
		{
			name:     "Valid premium list name",
			listName: "example",
			wantErr:  nil,
		},
		{
			name:     "Invalid premium list name",
			listName: "invalid_name!?",
			wantErr:  ErrInvalidPremiumListName,
		},
		{
			name:     "Empty premium list name",
			listName: "",
			wantErr:  ErrInvalidPremiumListName,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPremiumList(tc.listName)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
