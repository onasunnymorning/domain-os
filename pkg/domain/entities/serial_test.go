package entities

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSerialLessThan(t *testing.T) {
	tests := []struct {
		name   string
		s1, s2 uint32
		want   bool
	}{
		// Normal ordering
		{"1 < 2", 1, 2, true},
		{"100 < 200", 100, 200, true},
		{"0 < 1", 0, 1, true},

		// Reverse
		{"2 not < 1", 2, 1, false},
		{"200 not < 100", 200, 100, false},

		// Equal
		{"42 not < 42", 42, 42, false},
		{"0 not < 0", 0, 0, false},

		// Wraparound: s1 near max, s2 near 0 → s2 is "ahead"
		{"0xFFFFFFFF < 0 (wraps)", math.MaxUint32, 0, true},
		{"0xFFFFFFFF < 1 (wraps)", math.MaxUint32, 1, true},
		{"0xFFFFFFFE < 1 (wraps)", math.MaxUint32 - 1, 1, true},
		{"0xFFFFFFF0 < 0x10 (wraps)", uint32(0xFFFFFFF0), uint32(0x10), true},

		// Wraparound: s1 near 0, s2 near max → s1 is "ahead"
		{"0 not < 0xFFFFFFFF (wraps back)", 0, math.MaxUint32, false},
		{"1 not < 0xFFFFFFFF (wraps back)", 1, math.MaxUint32, false},

		// Large gap but less than 2^31 → still valid comparison
		{"0 < 2^31 - 1", 0, (1 << 31) - 1, true},

		// Exactly 2^31 apart — undefined, should return false
		{"0 not < 2^31 (undefined)", 0, 1 << 31, false},
		{"1 not < 2^31+1 (undefined)", 1, uint32((1 << 31) + 1), false},

		// Realistic DNS serials (YYYYMMDDnn format)
		{"2024010101 < 2024010102", 2024010101, 2024010102, true},
		{"2024010102 not < 2024010101", 2024010102, 2024010101, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SerialLessThan(tt.s1, tt.s2)
			assert.Equal(t, tt.want, got,
				"SerialLessThan(%d, %d) = %v, want %v", tt.s1, tt.s2, got, tt.want)
		})
	}
}

func TestSerialEqual(t *testing.T) {
	tests := []struct {
		s1, s2 uint32
		want   bool
	}{
		{0, 0, true},
		{42, 42, true},
		{math.MaxUint32, math.MaxUint32, true},
		{0, 1, false},
		{1, 0, false},
		{2024010101, 2024010102, false},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d_eq_%d", tt.s1, tt.s2), func(t *testing.T) {
			assert.Equal(t, tt.want, SerialEqual(tt.s1, tt.s2))
		})
	}
}

func TestSerialCompare(t *testing.T) {
	tests := []struct {
		name   string
		s1, s2 uint32
		want   int
	}{
		// Normal ordering
		{"1 cmp 2", 1, 2, -1},
		{"2 cmp 1", 2, 1, 1},

		// Equal
		{"42 cmp 42", 42, 42, 0},

		// Wraparound
		{"0xFFFFFFFF cmp 0 (wraps)", math.MaxUint32, 0, -1},
		{"0 cmp 0xFFFFFFFF (wraps back)", 0, math.MaxUint32, 1},

		// Undefined (exactly 2^31 apart)
		{"0 cmp 2^31 (undefined)", 0, 1 << 31, 0},
		{"2^31 cmp 0 (undefined)", 1 << 31, 0, 0},

		// Near boundary
		{"0 cmp 2^31-1 (max valid)", 0, (1 << 31) - 1, -1},
		{"0 cmp 2^31+1 (wraps to >)", 0, uint32((1 << 31) + 1), 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SerialCompare(tt.s1, tt.s2)
			assert.Equal(t, tt.want, got,
				"SerialCompare(%d, %d) = %d, want %d", tt.s1, tt.s2, got, tt.want)
		})
	}
}
