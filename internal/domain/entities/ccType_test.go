package entities

import (
	"testing"

	require "github.com/stretchr/testify/require"
)

func TestCCType_IsValid(t *testing.T) {
	tests := []struct {
		cc       string
		expected string
		err      error
	}{
		{"AR", "AR", nil},
		{"ar", "AR", nil},
		{"aR", "AR", nil},
		{"Ar", "AR", nil},
		{"a", "", ErrInvalidCountryCode},
		{"", "", ErrInvalidCountryCode},
		{"USA", "", ErrInvalidCountryCode},
		{"PP", "", ErrInvalidCountryCode},
	}
	for _, test := range tests {
		t.Run(test.cc, func(t *testing.T) {
			cc, err := NewCCType(test.cc)
			if err != nil {
				require.Equal(t, test.err, err)
			} else {
				require.Equal(t, test.expected, cc.String())
			}
		})
	}

}
