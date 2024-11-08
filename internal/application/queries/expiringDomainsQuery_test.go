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
func TestNewExpiringDomainsQuery(t *testing.T) {
	testCases := []struct {
		name       string
		clid       string
		date       string
		tld        string
		expectErr  bool
		errMessage string
	}{
		{
			name:      "Valid inputs",
			clid:      "validClID",
			date:      "",
			tld:       "com",
			expectErr: false,
		},
		{
			name:      "Empty clid and date",
			clid:      "",
			date:      "",
			tld:       "net",
			expectErr: false,
		},
		{
			name:       "Invalid date format",
			clid:       "validClID",
			date:       "01-01-2023", // Invalid format
			tld:        "org",
			expectErr:  true,
			errMessage: "invalid time format",
		},
		{
			name:       "Invalid clid",
			clid:       "thisistooooolooooooong", // Invalid format
			date:       "2023-01-01",
			tld:        "io",
			expectErr:  true,
			errMessage: "invalid clIDType",
		},
		{
			name:       "Invalid tld",
			clid:       "validClID",
			date:       "2023-01-01",
			tld:        "-aaaaaa", // Cannot start with a hyphen
			expectErr:  true,
			errMessage: "invalid label: each label cannot start or end with a hyphen",
		},
		{
			name:      "empty tld",
			clid:      "validClID",
			date:      "",
			tld:       "", // Is valid
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			query, err := NewExpiringDomainsQuery(tc.clid, tc.date, tc.tld)

			if tc.expectErr {
				if err == nil {
					t.Errorf("Expected error but got nil")
				} else if !containsErrorMessage(err, tc.errMessage) {
					t.Errorf("Expected error message to contain '%s', got '%s'", tc.errMessage, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Did not expect error but got '%s'", err.Error())
				}
				if query == nil {
					t.Error("Expected non-nil query, got nil")
				} else {
					// Additional checks can be added here
					// For example, validate the parsed date
					if tc.date != "" {
						expectedDate, _ := time.Parse(time.DateOnly, tc.date)
						if !query.Before.Equal(expectedDate) {
							t.Errorf("Expected date '%v', got '%v'", expectedDate, query.Before)
						}
					}
				}
			}
		})
	}
}

// Helper function to check if the error message contains a specific substring
func containsErrorMessage(err error, msg string) bool {
	return err != nil && err.Error() != "" && strings.Contains(err.Error(), msg)
}
