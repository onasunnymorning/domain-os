package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/idna"
)

func TestNewNNDN(t *testing.T) {
	// Positive test case
	t.Run("Valid NNDN Creation", func(t *testing.T) {
		const (
			asciiName   = "example.com"
			unicodeName = "example.com"
			tldName     = "com"
		)

		nndn, err := NewNNDN(asciiName)
		require.NoError(t, err, "Failed to create NNDN")

		require.Equal(t, asciiName, nndn.Name.String(), "ASCII Name mismatch")
		uNameStr, _ := idna.ToUnicode(nndn.Name.String())
		require.Equal(t, unicodeName, uNameStr, "Unicode Name mismatch")
		require.Equal(t, tldName, nndn.TLDName.String(), "TLD Name mismatch")
		require.True(t, nndn.CreatedAt.Before(time.Now()), "CreatedAt should be before current time")
		require.True(t, nndn.UpdatedAt.Before(time.Now()), "UpdatedAt should be before current time")
	})

	// Negative test cases
	tests := []struct {
		name   string
		domain string
	}{
		{
			name:   "Invalid ASCII domain",
			domain: "invalid_domain!?.com",
		},
		{
			name:   "Too long domain",
			domain: "thisisaverylongdomainnamethatexceedsthemaximumallowedlengthforadomainname.com",
		},
		{
			name:   "Empty domain",
			domain: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewNNDN(tc.domain)
			require.Error(t, err, "Expected an error for "+tc.name)
		})
	}
}
