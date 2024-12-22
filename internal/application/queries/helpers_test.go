package queries

import (
	"strings"
	"testing"
	"time"

	"github.com/onasunnymorning/domain-os/internal/domain/entities"
	"github.com/stretchr/testify/assert"
)

func TestParseClID(t *testing.T) {
	tests := []struct {
		name    string
		clid    string
		want    entities.ClIDType
		wantErr bool
	}{
		{
			name:    "Empty ClID",
			clid:    "",
			want:    entities.ClIDType(""),
			wantErr: false,
		},
		{
			name:    "Valid ClID",
			clid:    "validClID",
			want:    entities.ClIDType("validClID"),
			wantErr: false,
		},
		{
			name:    "Invalid ClID",
			clid:    "invalidClIDBecauseItIsWaaaayToooooLooooong",
			want:    entities.ClIDType(""),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseClID(tt.clid)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
func TestParseDate(t *testing.T) {
	tests := []struct {
		name    string
		date    string
		want    time.Time
		wantErr bool
	}{
		{
			name:    "Empty Date",
			date:    "",
			want:    time.Now().UTC(),
			wantErr: false,
		},
		{
			name:    "Valid Date Only",
			date:    "2023-04-15",
			want:    time.Date(2023, 4, 15, 0, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "Valid RFC3339 Date",
			date:    "2023-04-15T01:00:00Z",
			want:    time.Date(2023, 4, 15, 1, 0, 0, 0, time.UTC),
			wantErr: false,
		},
		{
			name:    "Invalid Date",
			date:    "invalid-date",
			want:    time.Time{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTimeDefault(tt.date)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.date == "" {
					// Allow a margin of error for the current time
					assert.WithinDuration(t, tt.want, got, time.Second)
				} else {
					assert.Equal(t, tt.want, got)
				}
			}
		})
	}
}

// Helper function to check if the error message contains a specific substring
func containsErrorMessage(err error, msg string) bool {
	return err != nil && err.Error() != "" && strings.Contains(err.Error(), msg)
}
