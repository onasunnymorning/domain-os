package entities

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewPostalInfoEnumType(t *testing.T) {
	tests := []struct {
		input    string
		expected PostalInfoEnumType
		err      error
	}{
		{
			input:    "loc",
			expected: PostalInfoEnumType("loc"),
			err:      nil,
		},
		{
			input:    "int",
			expected: PostalInfoEnumType("int"),
			err:      nil,
		},
		{
			input:    "Int",
			expected: PostalInfoEnumType("int"),
			err:      nil,
		},
		{
			input:    "invalid",
			expected: "",
			err:      ErrInvalidPostalInfoEnumType,
		},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			actual, err := NewPostalInfoEnumType(test.input)
			require.Equal(t, test.err, err)
			if test.expected != "" {
				require.Equal(t, test.expected, *actual)
			}
		})
	}
}
