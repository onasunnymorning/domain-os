package entities

import (
	"testing"

	require "github.com/stretchr/testify/require"
)

func TestPCType_IsValid(t *testing.T) {
	tests := []struct {
		name string
		pc   string
		err  error
	}{
		{"valid", "12345", nil},
		{"too long", "12345678901234567890", ErrInvalidPCType},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			p, err := NewPCType(test.pc)
			require.Equal(t, test.err, err)
			if err == nil {
				require.Nil(t, p.Validate())
			}
		})
	}
}

func TestPCType_String(t *testing.T) {
	tests := []struct {
		pc       string
		expected string
		err      error
	}{
		{"12345", "12345", nil},
		{"12345678901234567890", "", ErrInvalidPCType},
	}
	for _, test := range tests {
		actual, err := NewPCType(test.pc)
		require.Equal(t, test.err, err, "Error mismatch")
		if err == nil {
			require.Equal(t, test.expected, actual.String(), "PCType mismatch")
		}
	}
}
