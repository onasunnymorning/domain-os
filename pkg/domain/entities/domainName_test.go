package entities

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDomainName(t *testing.T) {
	tests := []struct {
		testname      string
		name          string
		expected      string
		expectedError error
	}{
		{"example.com", "example.com", "example.com", nil},
		{"EXAMPLE.COM", "EXAMPLE.COM", "example.com", nil},
		{"example.com.", "example.com.", "example.com", nil},
		{"example", "example", "example", nil},
		{"example..com", "example..com", "example..com", ErrInvalidLabelLength},
		{"example_com", "example_com", "example_com", ErrLabelContainsInvalidCharacter},
		{"example$com", "example$com", "example$com", ErrLabelContainsInvalidCharacter},
		{"example!com", "example!com", "example!com", ErrLabelContainsInvalidCharacter},
		{"example.com!", "example.com!", "example.com!", ErrLabelContainsInvalidCharacter},
		{"example.com ", "example.com ", "example.com", nil},
		{" example.com", " example.com", "example.com", nil},
		{".example.com", ".example.com", "example.com", nil},
		{"empty", "", "", ErrinvalIdDomainNameLength},
		{"dot only", ".", "", ErrinvalIdDomainNameLength}, // dots will be trimmed
		{"one character", "a", "a", nil},
		{"domain name too long", "tooooooooooooooolooooooooooooooooooongdooooooooooooooooomainnaaaaaaaaaaaame.tooooooooooooooolooooooooooooooooooongdooooooooooooooooomainnaaaaaaaaaaaame.tooooooooooooooolooooooooooooooooooongdooooooooooooooooomainnaaaaaaaaaaaame.tooooooooooooooolooooooooooooooooooongdooooooooooooooooomainnaaaaaaaaaaaame.", "", ErrinvalIdDomainNameLength},
	}

	for _, test := range tests {
		t.Run(test.testname, func(t *testing.T) {
			d, err := NewDomainName(test.name)
			require.Equal(t, test.expectedError, err, "error mismatch")
			if err == nil {
				assert.Equal(t, test.expected, d.String(), "domain name mismatch")
			}
		})
	}
}
func TestDomainName_ParentDomain(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected string
	}{
		{"example.com", "example.com", "com"},
		{"sub.example.com", "sub.example.com", "example.com"},
		{"www.sub.example.com", "www.sub.example.com", "sub.example.com"},
		{"example", "example", ""},
		{"", "", ""},
	}

	for _, test := range tests {
		d := DomainName(test.domain)
		parentDomain := d.ParentDomain()
		if parentDomain != test.expected {
			t.Errorf("Expected parent domain to be %s, but got %s for domain %s", test.expected, parentDomain, test.domain)
		}
	}
}

func TestDomainName_Label(t *testing.T) {
	tests := []struct {
		name     string
		domain   string
		expected string
	}{
		{"example.com", "example.com", "example"},
		{"sub.example.com", "sub.example.com", "sub"},
		{"www.sub.example.com", "www.sub.example.com", "www"},
		{"example", "example", "example"},
		{"", "", ""},
	}

	for _, test := range tests {
		d := DomainName(test.domain)
		label := d.Label()
		if label != test.expected {
			t.Errorf("Expected parent domain to be %s, but got %s for domain %s", test.expected, label, test.domain)
		}
	}
}

func TestUnmarshallJson(t *testing.T) {
	// Test UnmarshalJSON method
	bytes := []byte(`"example.com"`)
	var name DomainName
	err := json.Unmarshal(bytes, &name)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if string(name) != "example.com" {
		t.Errorf("unexpected result, got %v, want %v", string(name), "example.com")
	}

}

func TestDomainNameUnmarshalJSONError(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected DomainName
	}{
		{
			name:  "invalid input",
			input: `123`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result DomainName
			err := json.Unmarshal([]byte(tt.input), &result)

			if tt.expected == "" {
				assert.Error(t, err)
				assert.Equal(t, DomainName(""), result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestDomainName_IsIDN(t *testing.T) {
	tests := []struct {
		name         string
		domainName   string
		expectedErr  error
		expectedBool bool
	}{
		{
			name:         "non-IDN",
			domainName:   "geoff.apex.domains",
			expectedErr:  nil,
			expectedBool: false,
		},
		{
			name:         "IDN",
			domainName:   "xn--c1yn36f.com",
			expectedErr:  nil,
			expectedBool: true,
		},
		{
			name:         "Error",
			domainName:   "xn--1.com",
			expectedErr:  ErrInvalidDomainName,
			expectedBool: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DomainName(tt.domainName)

			b, err := d.IsIDN()
			assert.Equal(t, tt.expectedErr, err)
			assert.Equal(t, b, tt.expectedBool)
		})
	}

}
