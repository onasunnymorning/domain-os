package entities

import (
	"testing"

	require "github.com/stretchr/testify/require"
)

func TestPostalLineType_IsValid(t *testing.T) {
	tests := []struct {
		pl       string
		expected string
		err      error
	}{
		{"", "", ErrInvalidPostalLineType},
		{"12345", "12345", nil},
		{"12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890", "", ErrInvalidPostalLineType},
	}
	for _, test := range tests {
		t.Run(test.pl, func(t *testing.T) {
			p, err := NewPostalLineType(test.pl)
			require.Equal(t, test.err, err)
			if err == nil {
				require.Nil(t, p.Validate())
			}
		},
		)

	}
}

func TestOptPostalLineType_IsValid(t *testing.T) {
	tests := []struct {
		pl       string
		expected string
		err      error
	}{
		{"", "", nil},
		{"12345", "12345", nil},
		{"12345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890", "", ErrInvalidOptPostalLineType},
	}
	for _, test := range tests {
		t.Run(test.pl, func(t *testing.T) {
			p, err := NewOptPostalLineType(test.pl)
			require.Equal(t, test.err, err)
			if err == nil {
				require.Nil(t, p.IsValid())
			}
		},
		)
	}
}

func TestPostalLineType_String(t *testing.T) {
	pi, _ := NewPostalLineType("12345")

	if pi.String() != "12345" {
		t.Errorf("PostalLineType string mismatch")
	}
}

func TestOptPostalLineType_String(t *testing.T) {
	pi, _ := NewOptPostalLineType("12345")

	if pi.String() != "12345" {
		t.Errorf("OptPostalLineType string mismatch")
	}
}
