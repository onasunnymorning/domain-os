package entities

import (
	"testing"
)

func TestFindInvalidLabelCharacters(t *testing.T) {
	tests := []struct {
		label           string
		expectedInvalid string
	}{
		{"abc123", ""},
		{"aBc123", ""},
		{"abc-123", ""},
		{"abc_123", "_"},
		{"abc$123", "$"},
		{"abc123!", "!"},
		{"abc123ñ", "ñ"},
		{"abc123ñ@", "ñ"},
		{"abc@123", "@"},
		{"ABC 123", " "},
		{"abc123-", ""},
		{"abc123--", ""},
		{"abc123--def", ""},
		{"", ""},
		{"-abc", ""},
		{"abc-", ""},
		{"abc--def", ""},
		{"xn--abc", ""},
		{"xn--ümlaut", "ü"},
	}

	for _, test := range tests {
		result := findInvalidLabelCharacters(test.label)
		if result != test.expectedInvalid {
			t.Errorf("Expected invalid character to be %s, but got %s for input %s", test.expectedInvalid, result, test.label)
		}
	}
}

func TestIsValidLabel(t *testing.T) {
	tests := []struct {
		label    string
		expected bool
	}{
		{"abc123", true},
		{"abc-123", true},
		{"abc_123", false},
		{"abc$123", false},
		{"abc123!", false},
		{"abc123ñ", false},
		{"abc123ñ@", false},
		{"abc@123", false},
		{"abc 123", false},
		{"abc123-", false},
		{"abc123--", false},
		{"abc123--def", false},
		{"", false},
		{"-abc", false},
		{"abc-", false},
		{"abc--def", false},
		{"xn--abc", false},
		{"xn--cario-rta", true},
		{"xn--ümlaut", false},
	}

	for _, test := range tests {
		result := IsValidLabel(test.label)
		if result != test.expected {
			t.Errorf("Expected IsValidLabel(%s) to be %v, but got %v", test.label, test.expected, result)
		}
	}
}
