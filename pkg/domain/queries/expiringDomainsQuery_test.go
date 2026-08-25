package queries

import (
	"testing"
	"time"
)

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
