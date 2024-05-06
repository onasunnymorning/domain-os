package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPremiumList(t *testing.T) {
	tests := []struct {
		name     string
		listName string
		ryid     string
		wantErr  error
	}{
		{
			name:     "Valid premium list name",
			listName: "example",
			ryid:     "ry-id",
			wantErr:  nil,
		},
		{
			name:     "Invalid premium list name",
			listName: "invalid_name!?",
			ryid:     "ry-id",
			wantErr:  ErrInvalidPremiumListName,
		},
		{
			name:     "Empty premium list name",
			listName: "",
			ryid:     "ry-id",
			wantErr:  ErrInvalidPremiumListName,
		},
		{
			name:     "Empty ryid ",
			listName: "mypremiumlist",
			ryid:     "",
			wantErr:  ErrInvalidClIDType,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewPremiumList(tc.listName, tc.ryid)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
