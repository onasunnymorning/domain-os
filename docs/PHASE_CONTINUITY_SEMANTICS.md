# Phase Continuity Semantics: [Inclusive Start, Exclusive End)

## Overview

We've implemented **[inclusive start, exclusive end)** semantics for Phase timing. This solves the critical timing precision problem where phases need to touch at boundaries without gaps or overlaps.

## The Problem

With the previous implementation, if two phases touched at the same timestamp:

```go
phase1.Ends   = 2025-10-03 00:00:00.000000000 UTC
phase2.Starts = 2025-10-03 00:00:00.000000000 UTC
```

At exactly `2025-10-03 00:00:00.000000000 UTC`:
- ❌ Phase1: `Starts.Before(now) && Ends.After(now)` → `false` (NOT active)
- ❌ Phase2: `Starts.Before(now) && Ends.After(now)` → `false` (NOT active)

**Result**: At that exact nanosecond, NEITHER phase would be active! 😱

## The Solution

With **[inclusive start, exclusive end)** semantics:

```go
// A phase is active during [Starts, Ends)
// - Active if now >= Starts (inclusive)
// - Active if now < Ends (exclusive)
```

Now at exactly `2025-10-03 00:00:00.000000000 UTC`:
- ❌ Phase1: `now >= Starts && now < Ends` → `false` (because `now >= Ends`)
- ✅ Phase2: `now >= Starts && now < Ends` → `true` (because `now >= Starts`)

**Result**: Perfect handoff! At the boundary, phase2 becomes active exactly when phase1 ends.

## Implementation Details

### 1. IsCurrentlyActive()

```go
func (p *Phase) IsCurrentlyActive() bool {
	now := time.Now().UTC()
	// Active if: now >= Starts AND (no end OR now < Ends)
	return !now.Before(p.Starts) && (p.Ends == nil || now.Before(*p.Ends))
}
```

**Semantics**: A phase is active during `[Starts, Ends)` 
- A request at exactly `Starts` time belongs to this phase
- A request at exactly `Ends` time belongs to the NEXT phase

### 2. OverlapsWith()

```go
func (p *Phase) OverlapsWith(other *Phase) bool {
	// ... handle nil cases ...
	
	// if both phases have an end date: [A, B) and [C, D)
	// They overlap if: A < D AND C < B
	// They touch (no overlap) if: B == C OR D == A
	return p.Starts.Before(*other.Ends) && other.Starts.Before(*p.Ends)
}
```

**Key Property**: Phases can now touch at boundaries without overlap:
- Phase1 `[2025-10-01, 2025-10-03)` and Phase2 `[2025-10-03, 2025-10-05)` → **NO OVERLAP** ✓

### 3. New Helper Methods

#### IsContinuousWith()
```go
func (p *Phase) IsContinuousWith(other *Phase) bool {
	if p.Ends == nil || other.Ends == nil {
		return false
	}
	return p.Ends.Equal(other.Starts) || other.Ends.Equal(p.Starts)
}
```

Checks if two phases are perfectly continuous (no gap, no overlap).

#### SuggestNextPhaseStart()
```go
func (p *Phase) SuggestNextPhaseStart() time.Time {
	if p.Ends == nil {
		return time.Time{}
	}
	return *p.Ends
}
```

Returns the exact timestamp where the next phase should start to maintain continuity.

## Usage Examples

### Creating Continuous Phases

```go
// Phase 1
phase1, _ := NewPhase("ga1", "GA", time.Date(2025, 10, 1, 0, 0, 0, 0, time.UTC))
phase1.SetEnd(time.Date(2025, 10, 3, 0, 0, 0, 0, time.UTC))

// Phase 2 - use SuggestNextPhaseStart() for perfect continuity
nextStart := phase1.SuggestNextPhaseStart() // Returns: 2025-10-03 00:00:00 UTC
phase2, _ := NewPhase("ga2", "GA", nextStart)
phase2.SetEnd(time.Date(2025, 10, 5, 0, 0, 0, 0, time.UTC))

// Verify
phase1.OverlapsWith(phase2)       // false ✓
phase1.IsContinuousWith(phase2)   // true ✓
```

### At the Exact Boundary Moment

```go
boundary := time.Date(2025, 10, 3, 0, 0, 0, 0, time.UTC)

// Simulate IsCurrentlyActive at boundary
phase1Active := !boundary.Before(phase1.Starts) && boundary.Before(*phase1.Ends)
// false (boundary is not < Ends)

phase2Active := !boundary.Before(phase2.Starts) && boundary.Before(*phase2.Ends)  
// true (boundary >= Starts and boundary < Ends)
```

## Benefits

1. **No Gaps**: Phases can touch at boundaries without leaving any moment uncovered
2. **No Overlaps**: At any given moment, only one GA phase is active
3. **Intuitive**: Matches how most systems work (database ranges, time intervals)
4. **Simple**: No need to deal with nanosecond arithmetic
5. **Deterministic**: Clear rules about which phase handles requests at boundaries

## Testing

Comprehensive test suite in `phase_continuity_test.go` validates:

- ✅ Touching phases do not overlap
- ✅ Overlapping by 1 nanosecond is detected
- ✅ Gaps are detected
- ✅ Perfect continuity is verified
- ✅ Exact boundary moment behavior
- ✅ SuggestNextPhaseStart() returns correct times

## Migration Notes

### For API Users

The external API behavior should remain the same. The key difference is:

**Before**: Phases touching at the same timestamp would overlap
**After**: Phases touching at the same timestamp are continuous (no overlap)

### For Frontend

The UI already shows phases correctly. No changes needed, but you can now confidently:
- Set phase2.Starts = phase1.Ends for perfect continuity
- Show phases touching at boundaries without gaps

### For Existing Data

Existing phases in the database are not affected. The new semantics only apply to:
1. Creating new phases
2. Checking if phases are active
3. Validating overlaps

## Further Improvements

With this foundation, we can now implement:
1. **Timeline Validation** - Service layer that validates entire phase timeline
2. **Gap Detection** - Identify unintentional gaps between phases
3. **Auto-Continuity** - API that automatically creates continuous phases
4. **Visual Warnings** - UI indicators for gaps/overlaps

## References

- Implementation: `pkg/domain/entities/phase.go`
- Tests: `pkg/domain/entities/phase_continuity_test.go`
- Related Docs: `EPP_PRODUCTION_ARCHITECTURE.md`, `PHASE_TIMELINE_COMPLETE.md`
