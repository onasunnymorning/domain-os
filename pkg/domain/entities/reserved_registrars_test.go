package entities

import "testing"

func TestIsSpecialReservedGurID(t *testing.T) {
	tests := []struct {
		name   string
		gurID  int
		expect bool
	}{
		{"9995 is special reserved", 9995, true},
		{"9996 is special reserved", 9996, true},
		{"9997 is special reserved", 9997, true},
		{"9998 is NOT special reserved (TLD-scoped)", 9998, false},
		{"9999 is NOT special reserved (TLD-scoped)", 9999, false},
		{"regular accredited registrar", 1234, false},
		{"zero", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsSpecialReservedGurID(tt.gurID)
			if got != tt.expect {
				t.Errorf("IsSpecialReservedGurID(%d) = %v, want %v", tt.gurID, got, tt.expect)
			}
		})
	}
}

func TestSpecialReservedGurIDs_AllHaveDescriptions(t *testing.T) {
	for id, desc := range SpecialReservedGurIDs {
		if desc == "" {
			t.Errorf("SpecialReservedGurIDs[%d] has empty description", id)
		}
	}
}

func TestTLDScopedReservedGurIDs_AllHaveDescriptions(t *testing.T) {
	for id, desc := range TLDScopedReservedGurIDs {
		if desc == "" {
			t.Errorf("TLDScopedReservedGurIDs[%d] has empty description", id)
		}
	}
}
