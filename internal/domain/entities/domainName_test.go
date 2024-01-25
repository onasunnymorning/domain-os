package entities

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDomainName(t *testing.T) {
	tests := []struct {
		name          string
		expected      string
		expectedError error
	}{
		{"example.com", "example.com", nil},
		{"EXAMPLE.COM", "example.com", nil},
		{"example.com.", "example.com", nil},
		{"example", "example", nil},
		{"example..com", "example..com", ErrInvalidDomainName},
		{"example_com", "example_com", ErrInvalidDomainName},
		{"example$com", "example$com", ErrInvalidDomainName},
		{"example!com", "example!com", ErrInvalidDomainName},
		{"example.com!", "example.com!", ErrInvalidDomainName},
		{"example.com ", "example.com", nil},
		{" example.com", "example.com", nil},
		{".example.com", "example.com", nil},
		{"", "", ErrInvalidDomainName},
		{".", "", ErrInvalidDomainName},
		{"a", "a", nil},
		{"tooooooooooooooolooooooooooooooooooongdooooooooooooooooomainnaaaaaaaaaaaame.tooooooooooooooolooooooooooooooooooongdooooooooooooooooomainnaaaaaaaaaaaame.tooooooooooooooolooooooooooooooooooongdooooooooooooooooomainnaaaaaaaaaaaame.tooooooooooooooolooooooooooooooooooongdooooooooooooooooomainnaaaaaaaaaaaame.", "", ErrInvalidDomainName},
	}

	for _, test := range tests {
		d, err := NewDomainName(test.name)
		if err != test.expectedError {
			t.Errorf("Expected error to be %v, but got %v for input %s", test.expectedError, err, test.name)
		}

		if err == nil && test.expected != d.String() {
			t.Errorf("Expected domain name to be %s, but got %s for input %s", strings.ToLower(test.name), d.String(), test.name)
		}

		if d != nil {
			tld := d.GetTLD()
			require.NotEmptyf(t, tld, "TLD must not be empty")
		}
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
