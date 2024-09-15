package queries

import (
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
func TestNewExpiringDomainsQuery(t *testing.T) {
	tests := []struct {
		name    string
		clid    string
		date    string
		want    *ExpiringDomainsQuery
		wantErr bool
	}{
		{
			name:    "Empty ClID and Date",
			clid:    "",
			date:    "",
			want:    &ExpiringDomainsQuery{Before: time.Now().UTC(), ClID: entities.ClIDType("")},
			wantErr: false,
		},
		{
			name:    "Valid ClID and Date",
			clid:    "validClID",
			date:    "2023-04-15",
			want:    &ExpiringDomainsQuery{Before: time.Date(2023, 4, 15, 0, 0, 0, 0, time.UTC), ClID: entities.ClIDType("validClID")},
			wantErr: false,
		},
		{
			name:    "Valid ClID and DateTime",
			clid:    "validClID",
			date:    "2023-04-15T01:00:00Z",
			want:    &ExpiringDomainsQuery{Before: time.Date(2023, 4, 15, 1, 0, 0, 0, time.UTC), ClID: entities.ClIDType("validClID")},
			wantErr: false,
		},
		{
			name:    "Invalid Date",
			clid:    "validClID",
			date:    "invalid-date",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "Invalid ClID",
			clid:    "invalidClIDBecauseItIsWaaaayToooooLooooong",
			date:    "2023-04-15",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewExpiringDomainsQuery(tt.clid, tt.date)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.date == "" {
					// Allow a margin of error for the current time
					assert.WithinDuration(t, tt.want.Before, got.Before, time.Second)
				} else {
					assert.Equal(t, tt.want.Before, got.Before)
				}
				assert.Equal(t, tt.want.ClID, got.ClID)
			}
		})
	}
}
