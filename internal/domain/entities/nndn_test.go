package entities

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/idna"
)

func TestNewNNDN(t *testing.T) {
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

	// test cases
	tests := []struct {
		name   string
		domain string
		err    error
	}{
		{
			name:   "Invalid ASCII domain",
			domain: "invalid_domain!?.com",
			err:    ErrInvalidDomainName,
		},
		{
			name:   "Too long domain",
			domain: "thisisaverylongdomainnamethatexceedsthemaximumallowedlengthforadomainname.com",
			err:    ErrInvalidDomainName,
		},
		{
			name:   "Empty domain",
			domain: "",
			err:    ErrInvalidDomainName,
		},
		{
			name:   "Valid IDN TLD",
			domain: "xn--somevalididn.normal.tld",
			err:    nil,
		},
		{
			name:   "Valid SLD with IDN TLD",
			domain: "label.xn--somevalididn",
			err:    nil,
		},
		{
			name:   "Multiple IDN labels",
			domain: "label.xn--somevalididn.xn--somevalididn",
			err:    nil,
		},
		{
			name:   "Complex SLD structure",
			domain: "my.domain.in.an.sld",
			err:    nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewNNDN(tc.domain)
			if tc.err != nil {
				require.ErrorIs(t, err, tc.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
