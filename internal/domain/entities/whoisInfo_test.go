package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewWhoisInfo(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected *WhoisInfo
		err      error
	}{
		{
			name:     "example.com",
			url:      "example.com",
			expected: nil,
			err:      ErrInvalidURL,
		},
		{
			name:     "-invalidname",
			url:      "https://apex.domains/whois",
			expected: nil,
			err:      ErrInvalidLabelDash,
		},
		{
			name:     "validname",
			url:      "-invalidUrl",
			expected: nil,
			err:      ErrInvalidURL,
		},
		{
			name: "whois.apex.domains",
			url:  "https://apex.domains/whois",
			expected: &WhoisInfo{
				Name: "whois.apex.domains",
				URL:  "https://apex.domains/whois",
			},
			err: nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			w, err := NewWhoisInfo(test.name, test.url)
			require.Equal(t, test.err, err)
			if err == nil {
				require.Equal(t, test.expected, w)
			}
		})
	}
}

func TestWhoisInfo_Validate(t *testing.T) {
	w := WhoisInfo{
		Name: "whois.apex.domains",
		URL:  "https://apex.domains/whois",
	}
	require.NoError(t, w.Validate(), "WhoisInfo validation failed")
}

func TestWhoisInfo_ValidateInvalidName(t *testing.T) {
	w := WhoisInfo{
		Name: "-invalidname",
		URL:  "https://apex.domains/whois",
	}
	require.EqualError(t, w.Validate(), ErrInvalidLabelDash.Error(), "WhoisInfo validation failed")
}

func TestWhoisInfo_ValidateInvalidURL(t *testing.T) {
	w := WhoisInfo{
		Name: "whois.apex.domains",
		URL:  "https://--apex.domains/whois",
	}
	require.EqualError(t, w.Validate(), ErrInvalidLabelDash.Error(), "WhoisInfo validation failed")
}
