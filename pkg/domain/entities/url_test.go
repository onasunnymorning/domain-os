package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUrl(t *testing.T) {
	url := URL("https://apex.domains/whois")
	tests := []struct {
		url      string
		expected *URL
		err      error
	}{
		{
			url:      "example.com",
			expected: nil,
			err:      ErrInvalidURL,
		},
		{
			url:      "https://apex.domains/whois",
			expected: &url,
			err:      nil,
		},
	}

	for _, test := range tests {
		t.Run(test.url, func(t *testing.T) {
			u, err := NewURL(test.url)
			require.Equal(t, test.err, err)
			if err == nil {
				require.Equal(t, test.expected, u)
			}
		})

	}
}

func TestURL_String(t *testing.T) {
	url := URL("https://apex.domains/whois")
	require.Equal(t, "https://apex.domains/whois", url.String(), "URL string mismatch")
}

func TestURL_Validate(t *testing.T) {
	url := URL("https://apex.domains/whois")
	require.NoError(t, url.Validate(), "URL validation failed")
}

func TestURL_ValidateInvalid(t *testing.T) {
	url := URL("example.com")
	require.EqualError(t, url.Validate(), ErrInvalidURL.Error(), "URL validation failed")
}

func TestURL_ValidateInvalidDomain(t *testing.T) {
	url := URL("https://--apex.domains/whois")
	require.EqualError(t, url.Validate(), ErrInvalidLabelDash.Error(), "URL validation failed")
}

// A URL with no "scheme://" has fewer than three slash-separated fields, and
// reading the third to get its domain part panics. gonet.IsURL accepts several
// such spellings, so the panic was reachable from any deposit, EPP command or
// import that carried one: one registrar in a real .co escrow deposit wrote its
// whois URL as a bare host with a path, and that one value ended a validation
// of 12 million objects with "validation pipeline panicked".
func TestURL_ValidateSchemelessURLIsInvalidNotFatal(t *testing.T) {
	for _, u := range []string{
		"www.example.com/whois", // the .co registrar's spelling: one slash, two fields
		"example.com/path",
		"example.com/a/b", // three fields, and the third is a path segment
		"mailto:someone@example.com",
	} {
		t.Run(u, func(t *testing.T) {
			url := URL(u)
			require.NotPanics(t, func() { _ = url.Validate() })
			require.ErrorIs(t, url.Validate(), ErrInvalidURL)
		})
	}
}

// The shapes that did work must keep working: a URL with a scheme is still
// judged by its domain part and nothing else.
func TestURL_ValidateKeepsJudgingTheDomainPart(t *testing.T) {
	valid := []string{"https://apex.domains/whois", "https://apex.domains", "HTTP://APEX.DOMAINS/whois"}
	for _, s := range valid {
		u := URL(s)
		require.NoError(t, u.Validate(), s)
	}
	bad := URL("https://--apex.domains/whois")
	require.ErrorIs(t, bad.Validate(), ErrInvalidLabelDash)
}
