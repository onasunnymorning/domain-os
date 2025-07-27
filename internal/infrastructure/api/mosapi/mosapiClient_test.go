package mosapi

import (
	"testing"
)

func TestEntityTypeString(t *testing.T) {
	tests := []struct {
		name     string
		input    EntityType
		expected string
	}{
		{
			name:     "Registry",
			input:    EntityRegistry,
			expected: "ry",
		},
		{
			name:     "Registrar",
			input:    EntityRegistrar,
			expected: "rr",
		},
		{
			name:     "Unknown",
			input:    "foo",
			expected: "unknown",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.input.String()
			if result != tc.expected {
				t.Errorf("String() = %q, want %q", result, tc.expected)
			}
		})
	}
}
