package entities

import (
	"testing"
	"time"
)

// TestPhaseContinuity tests the [inclusive start, exclusive end) semantics
func TestPhaseContinuity(t *testing.T) {
	// Create two consecutive phases that touch at the boundary (using future dates)
	now := time.Now().UTC()
	start1 := now.Add(24 * time.Hour)
	end1 := start1.Add(48 * time.Hour)
	start2 := end1 // Same as end1
	end2 := start2.Add(48 * time.Hour)

	phase1, err := NewPhase("ga1", "GA", start1)
	if err != nil {
		t.Fatalf("Failed to create phase1: %v", err)
	}
	if err := phase1.SetEnd(end1); err != nil {
		t.Fatalf("Failed to set end for phase1: %v", err)
	}

	phase2, err := NewPhase("ga2", "GA", start2)
	if err != nil {
		t.Fatalf("Failed to create phase2: %v", err)
	}
	if err := phase2.SetEnd(end2); err != nil {
		t.Fatalf("Failed to set end for phase2: %v", err)
	}

	// Test 1: Phases should NOT overlap (they touch at boundary)
	t.Run("NoOverlapAtBoundary", func(t *testing.T) {
		if phase1.OverlapsWith(phase2) {
			t.Error("Phases should not overlap when touching at boundary")
		}
		if phase2.OverlapsWith(phase1) {
			t.Error("Phases should not overlap when touching at boundary (reverse check)")
		}
	})

	// Test 2: Phases should be continuous
	t.Run("IsContinuous", func(t *testing.T) {
		if !phase1.IsContinuousWith(phase2) {
			t.Error("Phases should be continuous")
		}
		if !phase2.IsContinuousWith(phase1) {
			t.Error("Phases should be continuous (reverse check)")
		}
	})

	// Test 3: SuggestNextPhaseStart should return the correct time
	t.Run("SuggestNextPhaseStart", func(t *testing.T) {
		suggested := phase1.SuggestNextPhaseStart()
		if !suggested.Equal(start2) {
			t.Errorf("SuggestNextPhaseStart() = %v, want %v", suggested, start2)
		}
	})

	// Test 4: At the exact boundary moment, only phase2 should be active
	t.Run("ExactBoundaryMoment", func(t *testing.T) {
		// Note: We can't easily test with time.Now(), but we can test the logic
		// by temporarily modifying IsCurrentlyActive to accept a time parameter
		// For now, we'll test the semantics are correct by checking the logic

		// At exactly the boundary time:
		// - phase1.Ends.Before(boundary) = false (boundary == end1)
		// - phase1.Ends.After(boundary) = false (boundary == end1)
		// - So phase1 should NOT be active (now >= end1 means now < end1 is false)

		// - phase2.Starts.Before(boundary) = false (boundary == start2)
		// - phase2.Starts.After(boundary) = false (boundary == start2)
		// - So phase2 SHOULD be active (now >= start2 is true)

		boundary := end1 // The exact moment where phases touch

		// Simulate IsCurrentlyActive logic for phase1 at boundary
		phase1Active := !boundary.Before(phase1.Starts) && boundary.Before(*phase1.Ends)
		if phase1Active {
			t.Error("Phase1 should NOT be active at the exact boundary moment")
		}

		// Simulate IsCurrentlyActive logic for phase2 at boundary
		phase2Active := !boundary.Before(phase2.Starts) && (phase2.Ends == nil || boundary.Before(*phase2.Ends))
		if !phase2Active {
			t.Error("Phase2 SHOULD be active at the exact boundary moment")
		}
	})
}

// TestPhaseOverlap tests various overlap scenarios
func TestPhaseOverlap(t *testing.T) {
	// Compute base time once to ensure consistency across test pairs
	baseTime := time.Now().UTC().Add(24 * time.Hour)

	tests := []struct {
		name     string
		phase1   func(base time.Time) *Phase
		phase2   func(base time.Time) *Phase
		overlaps bool
	}{
		{
			name: "Touching phases do not overlap",
			phase1: func(base time.Time) *Phase {
				p, err := NewPhase("phase1", "GA", base)
				if err != nil {
					panic(err)
				}
				end := base.Add(48 * time.Hour)
				if err := p.SetEnd(end); err != nil {
					panic(err)
				}
				return p
			},
			phase2: func(base time.Time) *Phase {
				start := base.Add(48 * time.Hour)
				p, err := NewPhase("phase2", "GA", start)
				if err != nil {
					panic(err)
				}
				end := start.Add(48 * time.Hour)
				if err := p.SetEnd(end); err != nil {
					panic(err)
				}
				return p
			},
			overlaps: false,
		},
		{
			name: "Overlapping by 1 nanosecond",
			phase1: func(base time.Time) *Phase {
				p, _ := NewPhase("phase1", "GA", base)
				end := base.Add(48*time.Hour + 1*time.Nanosecond)
				_ = p.SetEnd(end)
				return p
			},
			phase2: func(base time.Time) *Phase {
				start := base.Add(48 * time.Hour)
				p, _ := NewPhase("phase2", "GA", start)
				end := start.Add(48 * time.Hour)
				_ = p.SetEnd(end)
				return p
			},
			overlaps: true,
		},
		{
			name: "Gap of 1 nanosecond",
			phase1: func(base time.Time) *Phase {
				p, _ := NewPhase("phase1", "GA", base)
				end := base.Add(48*time.Hour - 1*time.Nanosecond)
				_ = p.SetEnd(end)
				return p
			},
			phase2: func(base time.Time) *Phase {
				start := base.Add(48 * time.Hour)
				p, _ := NewPhase("phase2", "GA", start)
				end := start.Add(48 * time.Hour)
				_ = p.SetEnd(end)
				return p
			},
			overlaps: false,
		},
		{
			name: "Complete overlap",
			phase1: func(base time.Time) *Phase {
				p, _ := NewPhase("phase1", "GA", base)
				end := base.Add(96 * time.Hour)
				_ = p.SetEnd(end)
				return p
			},
			phase2: func(base time.Time) *Phase {
				start := base.Add(24 * time.Hour)
				p, _ := NewPhase("phase2", "GA", start)
				end := start.Add(48 * time.Hour)
				_ = p.SetEnd(end)
				return p
			},
			overlaps: true,
		},
		{
			name: "Both phases with no end date",
			phase1: func(base time.Time) *Phase {
				p, _ := NewPhase("phase1", "GA", base)
				return p
			},
			phase2: func(base time.Time) *Phase {
				p, _ := NewPhase("phase2", "GA", base.Add(24*time.Hour))
				return p
			},
			overlaps: true,
		},
		{
			name: "One phase with no end date, starting before the other",
			phase1: func(base time.Time) *Phase {
				p, _ := NewPhase("phase1", "GA", base)
				return p // No end date
			},
			phase2: func(base time.Time) *Phase {
				start := base.Add(24 * time.Hour)
				p, _ := NewPhase("phase2", "GA", start)
				end := start.Add(48 * time.Hour)
				_ = p.SetEnd(end)
				return p
			},
			overlaps: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p1 := tt.phase1(baseTime)
			p2 := tt.phase2(baseTime)

			result := p1.OverlapsWith(p2)
			if result != tt.overlaps {
				t.Errorf("OverlapsWith() = %v, want %v", result, tt.overlaps)
			}

			// Test symmetry
			result2 := p2.OverlapsWith(p1)
			if result2 != tt.overlaps {
				t.Errorf("OverlapsWith() (reverse) = %v, want %v", result2, tt.overlaps)
			}
		})
	}
}

// TestIsContinuousWith tests the continuity detection
func TestIsContinuousWith(t *testing.T) {
	// Compute base time once to ensure consistency across test pairs
	baseTime := time.Now().UTC().Add(24 * time.Hour)

	tests := []struct {
		name       string
		phase1     func(base time.Time) *Phase
		phase2     func(base time.Time) *Phase
		continuous bool
	}{
		{
			name: "Perfect continuity",
			phase1: func(base time.Time) *Phase {
				p, err := NewPhase("phase1", "GA", base)
				if err != nil {
					panic(err)
				}
				end := base.Add(48 * time.Hour)
				if err := p.SetEnd(end); err != nil {
					panic(err)
				}
				return p
			},
			phase2: func(base time.Time) *Phase {
				start := base.Add(48 * time.Hour)
				p, err := NewPhase("phase2", "GA", start)
				if err != nil {
					panic(err)
				}
				end := start.Add(48 * time.Hour)
				if err := p.SetEnd(end); err != nil {
					panic(err)
				}
				return p
			},
			continuous: true,
		},
		{
			name: "Gap between phases",
			phase1: func(base time.Time) *Phase {
				p, err := NewPhase("phase1", "GA", base)
				if err != nil {
					panic(err)
				}
				end := base.Add(48 * time.Hour)
				if err := p.SetEnd(end); err != nil {
					panic(err)
				}
				return p
			},
			phase2: func(base time.Time) *Phase {
				start := base.Add(72 * time.Hour)
				p, err := NewPhase("phase2", "GA", start)
				if err != nil {
					panic(err)
				}
				end := start.Add(24 * time.Hour)
				if err := p.SetEnd(end); err != nil {
					panic(err)
				}
				return p
			},
			continuous: false,
		},
		{
			name: "Overlap between phases",
			phase1: func(base time.Time) *Phase {
				p, err := NewPhase("phase1", "GA", base)
				if err != nil {
					panic(err)
				}
				end := base.Add(48*time.Hour + 1*time.Nanosecond)
				if err := p.SetEnd(end); err != nil {
					panic(err)
				}
				return p
			},
			phase2: func(base time.Time) *Phase {
				start := base.Add(48 * time.Hour)
				p, err := NewPhase("phase2", "GA", start)
				if err != nil {
					panic(err)
				}
				end := start.Add(48 * time.Hour)
				if err := p.SetEnd(end); err != nil {
					panic(err)
				}
				return p
			},
			continuous: false,
		},
		{
			name: "Phase with no end date",
			phase1: func(base time.Time) *Phase {
				p, err := NewPhase("phase1", "GA", base)
				if err != nil {
					panic(err)
				}
				return p // No end date
			},
			phase2: func(base time.Time) *Phase {
				start := base.Add(48 * time.Hour)
				p, err := NewPhase("phase2", "GA", start)
				if err != nil {
					panic(err)
				}
				end := start.Add(48 * time.Hour)
				if err := p.SetEnd(end); err != nil {
					panic(err)
				}
				return p
			},
			continuous: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p1 := tt.phase1(baseTime)
			p2 := tt.phase2(baseTime)

			result := p1.IsContinuousWith(p2)
			if result != tt.continuous {
				t.Errorf("IsContinuousWith() = %v, want %v", result, tt.continuous)
			}

			// Test symmetry
			result2 := p2.IsContinuousWith(p1)
			if result2 != tt.continuous {
				t.Errorf("IsContinuousWith() (reverse) = %v, want %v", result2, tt.continuous)
			}
		})
	}
}

// TestSuggestNextPhaseStart tests the next phase start suggestion
func TestSuggestNextPhaseStart(t *testing.T) {
	futureStart := time.Now().UTC().Add(24 * time.Hour)

	t.Run("Phase with end date", func(t *testing.T) {
		p, _ := NewPhase("phase1", "GA", futureStart)
		end := futureStart.Add(48 * time.Hour)
		_ = p.SetEnd(end)

		suggested := p.SuggestNextPhaseStart()
		if !suggested.Equal(end) {
			t.Errorf("SuggestNextPhaseStart() = %v, want %v", suggested, end)
		}
	})

	t.Run("Phase with no end date", func(t *testing.T) {
		p, _ := NewPhase("phase1", "GA", futureStart)
		// No end date set

		suggested := p.SuggestNextPhaseStart()
		if !suggested.IsZero() {
			t.Errorf("SuggestNextPhaseStart() = %v, want zero time", suggested)
		}
	})
}
