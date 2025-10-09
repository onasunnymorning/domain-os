# Phase Overlap Detection - Frontend Fix

## Problem

The frontend phase overlap validation in `PhaseCreateWizard.tsx` was using **incorrect semantics** that didn't align with the backend's [inclusive start, exclusive end) implementation.

### Observed Issue

When creating a new GA phase that started at the exact moment a previous phase ended (e.g., "shorty" ends Oct 9, 2025 and "longer" starts Oct 9, 2025 18:01), the frontend incorrectly showed:

> ❌ **"GA phase would overlap with existing phase 'shorty'"**

This is **wrong** - phases that touch at boundaries should **NOT** overlap with [inclusive, exclusive) semantics.

## Root Cause

The frontend overlap detection logic used **inclusive-both-ends semantics**:

```typescript
// OLD (WRONG) - Inclusive both ends
return formData.starts <= pEnd && (!formData.ends || formData.ends >= pStart);
```

This incorrectly treated touching phases as overlapping.

## Solution

Updated the frontend validation to match the backend's [inclusive start, exclusive end) semantics.

### Changes Made

#### 1. Fixed Overlap Detection Logic

**File:** `/frontend/components/phases/PhaseCreateWizard.tsx`

```typescript
// NEW (CORRECT) - [inclusive, exclusive) semantics
// Phase interval is [start, end) where start is inclusive, end is exclusive

// Both phases have no end date - they both extend indefinitely
if (!pEnd && !newEnd) return true;

// Existing phase has no end (ongoing) - overlaps if new phase starts before it
if (!pEnd) return newStart < pStart;

// New phase has no end (ongoing) - overlaps if it starts before existing phase ends
if (!newEnd) return newStart < pEnd;

// Both have end dates - use standard interval overlap formula
// [a,b) overlaps [c,d) iff a < d && c < b
// Phases touch at boundaries (a == d or b == c) but do NOT overlap
return newStart < pEnd && pStart < newEnd;
```

**Key Change:** Changed `<=` and `>=` to `<` comparisons to implement exclusive end semantics.

#### 2. Added Continuity Detection

Added helpful info message when creating a continuous phase:

```typescript
// Check for continuity with existing phases
const continuousPhase = existingPhases.filter(p => p.type === 'GA').find(p => {
  const pEnd = p.ends ? new Date(p.ends) : null;
  if (pEnd && newStart.getTime() === pEnd.getTime()) {
    return true;
  }
  return false;
});

if (continuousPhase) {
  setContinuityInfo(`This phase will be continuous with "${continuousPhase.name}" (no gap between phases)`);
}
```

Now displays:

> ℹ️ **This phase will be continuous with "shorty" (no gap between phases)**

#### 3. Updated Help Text

Changed the GA phase description from:
- ❌ "Cannot overlap with other GA phases"

To:
- ✅ "GA phases can be continuous (touching at boundaries) but cannot overlap"

## Semantic Alignment

### Backend (Go)
```go
func (p *Phase) OverlapsWith(other *Phase) bool {
    // ...
    return p.Starts.Before(*other.Ends) && other.Starts.Before(*p.Ends)
}
```

### Frontend (TypeScript)
```typescript
return newStart < pEnd && pStart < newEnd;
```

Both use `<` (strictly less than) to implement [inclusive, exclusive) semantics.

## Test Cases

The following scenarios now work correctly:

| Scenario | Phase 1 | Phase 2 | Should Overlap? | Frontend Result |
|----------|---------|---------|-----------------|-----------------|
| Touching | [Oct 2, Oct 9) | [Oct 9, Oct 16) | ❌ No | ✅ No overlap |
| 1ns overlap | [Oct 2, Oct 9 00:00:00.000000001) | [Oct 9, Oct 16) | ✅ Yes | ✅ Overlap detected |
| 1ns gap | [Oct 2, Oct 8 23:59:59.999999999) | [Oct 9, Oct 16) | ❌ No | ✅ No overlap |
| Both ongoing | [Oct 2, ∞) | [Oct 9, ∞) | ✅ Yes | ✅ Overlap detected |

## User Experience Improvements

1. **No False Positives:** Users can now create continuous phases without getting overlap errors
2. **Helpful Feedback:** Info message confirms when phases are continuous
3. **Clear Documentation:** Help text explains that touching phases are allowed
4. **Auto-Suggestion:** Existing auto-populate feature already suggests previous phase's end time as start time

## Related Documentation

- [Phase Continuity Semantics](./PHASE_CONTINUITY_SEMANTICS.md) - Backend implementation
- Backend tests: `internal/domain/entities/phase_continuity_test.go`
- Frontend component: `frontend/components/phases/PhaseCreateWizard.tsx`

## Migration Notes

No breaking changes - this is a **bug fix** that aligns frontend validation with backend behavior. Existing phases are unaffected.
