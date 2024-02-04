package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLabel_FindInvalidLabelCharacters(t *testing.T) {
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
		l := Label(test.label)
		result := l.findInvalidLabelCharacters()
		if result != test.expectedInvalid {
			t.Errorf("Expected findInvalidLabelCharacters(%s) to be %s, but got %s", test.label, test.expectedInvalid, result)
		}
	}
}

func TestLabel_IsValidLabel(t *testing.T) {
	tests := []struct {
		label    string
		expected error
	}{
		{"abc123", nil},
		{"abc-123", nil},
		{"abc_123", ErrLabelContainsInvalidCharacter},
		{"abc$123", ErrLabelContainsInvalidCharacter},
		{"abc123!", ErrLabelContainsInvalidCharacter},
		{"abc123ñ", ErrLabelContainsInvalidCharacter},
		{"abc123ñ@", ErrLabelContainsInvalidCharacter},
		{"abc@123", ErrLabelContainsInvalidCharacter},
		{"abc 123", ErrLabelContainsInvalidCharacter},
		{"abc123-", ErrInvalidLabelDash},
		{"abc123--", ErrInvalidLabelDash},
		{"abc123--def", ErrInvalidLabelDoubleDash},
		{"", ErrInvalidLabelLength},
		{"-abc", ErrInvalidLabelDash},
		{"abc-", ErrInvalidLabelDash},
		{"abc--def", ErrInvalidLabelDoubleDash},
		{"xn--abc", ErrInvalidLabelIDN},
		{"xn--cario-rta", nil},
		{"xn--ümlaut", ErrInvalidLabelIDN},
	}

	for _, test := range tests {
		l := Label(test.label)
		result := l.Validate()
		require.Equal(t, test.expected, result, "Expected Validate(%s) to be %s, but got %s", test.label, test.expected, result)
	}
}

func TestLabel_ToUnicode(t *testing.T) {
	tests := []struct {
		testname string
		label    string
		expected string
	}{
		{"non idn label", "abc123", "abc123"},
		{"cariño", "xn--cario-rta", "cariño"},
	}

	for _, test := range tests {
		l := Label(test.label)
		result, err := l.ToUnicode()
		require.Nil(t, err, "Expected ToUnicode(%s) to be nil, but got %s", test.label, err)
		require.Equal(t, test.expected, result, "Expected ToUnicode(%s) to be %s, but got %s", test.label, test.expected, result)
	}
}
