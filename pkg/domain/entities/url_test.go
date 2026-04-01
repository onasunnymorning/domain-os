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
