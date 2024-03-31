package entities

import (
	"testing"
	"time"
)

func TestTime_Round(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no rounding needed",
			input:    "2021-01-01T12:00:00.000001Z",
			expected: "2021-01-01T12:00:00.000001Z",
		},
		{
			name:     "Round down",
			input:    "2021-01-01T12:00:00.0000001Z",
			expected: "2021-01-01T12:00:00.000000Z",
		},
		{
			name:     "Round up",
			input:    "2021-01-01T12:00:00.0000009Z",
			expected: "2021-01-01T12:00:00.000001Z",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputTime, _ := time.Parse(time.RFC3339Nano, test.input)
			expectedTime, _ := time.Parse(time.RFC3339Nano, test.expected)

			result := RoundTime(inputTime)
			if !result.Equal(expectedTime) {
				t.Errorf("Expected time to be %v, but got %v", expectedTime, result)
			}
		})
	}
}

func TestTime_IsUTC(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "UTC",
			input:    "2021-01-01T12:00:00Z",
			expected: true,
		},
		{
			name:     "Not UTC",
			input:    "2021-01-01T12:00:00+01:00",
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			inputTime, _ := time.Parse(time.RFC3339, test.input)

			result := IsUTC(inputTime)
			if result != test.expected {
				t.Errorf("Expected time to be in UTC: %v, but got %v", test.expected, result)
			}
		})
	}
}
