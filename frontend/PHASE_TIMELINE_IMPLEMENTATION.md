# Phase Timeline Implementation Plan

## Understanding the Backend Phase Logic

### Phase Rules (from backend analysis)
1. **GA Phases**: 
   - Only ONE can be active at a time
   - Cannot overlap with each other
   - Must have explicit start/end dates to prevent overlap
   - Rolling 2-3 times per year for config changes

2. **Launch Phases**:
   - Multiple can be active simultaneously (typically 1-3, max seen)
   - CAN overlap with each other
   - CAN overlap/coexist with GA phases
   - Temporary in nature

3. **Phase Lifecycle**:
   - Current phases: `IsCurrentlyActive()` - time between start and end
   - Past phases: ended before now
   - Future phases: starting after now
   - Historic phases are kept for traceability

### Data Structure
```typescript
interface Phase {
  id: number;
  name: string;
  type: 'GA' | 'Launch';
  starts: string; // ISO date
  ends?: string | null; // ISO date, can be null for ongoing
  prices: Price[];
  fees: Fee[];
  premiumListName?: string | null;
  createdAt: string;
  updatedAt: string;
  tldName: string;
  policy: PhasePolicy;
}
```

## UI Design: Dual-Swimlane Timeline

### Layout Concept
```
┌─────────────────────────────────────────────────────────────┐
│  Phase Timeline - .example                                   │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  GA Phases                                                   │
│  ├────────────┤ ← Past    [████ Current ████] → Future     │
│     Phase 1              Phase 2 (Active)      Phase 3      │
│                                                              │
├─────────────────────────────────────────────────────────────┤
│  Launch Phases                                               │
│  ├──┤ ├──┤            [██] [██]                            │
│   SR  EAP            Claim  Land                            │
│                                                              │
├─────────────────────────────────────────────────────────────┤
│  [+ Add Phase]                                              │
└─────────────────────────────────────────────────────────────┘
```

### Visual Hierarchy

**Current GA Phase (Focal Point)**:
- Larger card/pill
- Highlighted with sunset glow/border
- Centered in view
- Shows key config at a glance

**Past Phases**:
- Smaller, muted
- Scroll left to view
- Checkmark icon
- Collapsed details

**Future Phases**:
- Medium size
- Outline style
- Clock icon
- Scroll right to view

**Launch Phases**:
- Smaller pills
- Color-coded by type
- Aligned under GA timeline
- Show overlap visually

## Component Structure

```
PhaseTimeline/
├── PhaseTimeline.tsx          # Main container
├── GATimeline.tsx             # Top swimlane for GA phases
├── LaunchTimeline.tsx         # Bottom swimlane for Launch phases
├── PhaseCard.tsx              # Individual phase display
├── PhaseDetail.tsx            # Expanded phase view (modal/drawer)
├── PhaseCreateWizard.tsx      # Multi-step phase creation
└── types.ts                   # TypeScript interfaces
```

## Features

### 1. Timeline View (Read-Only Initially)
- Horizontal scrollable timeline
- Dual swimlanes (GA top, Launch bottom)
- Current GA phase centered and highlighted
- Visual indicators:
  - ✓ Past phases (completed)
  - ● Current phases (active)
  - ○ Future phases (scheduled)
  - Connecting lines showing continuity

### 2. Phase Card (Collapsed State)
```
┌─────────────────┐
│ ● Phase Name    │
│ Jan 15 - Jun 30 │
│ 2024            │
└─────────────────┘
```
Shows:
- Status indicator (✓ ● ○)
- Phase name
- Date range
- Type badge (GA/Launch)

### 3. Phase Card (Expanded - Click)
Opens drawer/modal showing:
- Full phase name and type
- Start/end dates (formatted nicely)
- Policy details:
  - Registration/Renewal grace periods
  - Transfer settings
  - Label length rules
  - Auto-renewal settings
- Prices by currency
- Fees
- Premium list (if applicable)
- Edit/Delete buttons (for future phases)

### 4. Phase Creation Wizard (Simple)
**Step 1: Basic Info**
- Phase name (required)
- Type: GA or Launch (radio buttons)
- Start date (datetime picker)
- End date (optional datetime picker)

**Step 2: Policy (Optional - Use Defaults)**
- "Copy from previous phase" checkbox (checked by default)
- Or "Customize policy" to show form with:
  - Registration/Renewal GP
  - Transfer settings
  - etc.

**Step 3: Pricing (Optional)**
- Add prices by currency
- Add fees
- Reference premium list

**Step 4: Review & Create**
- Summary of all settings
- Validation warnings (e.g., overlap detection)
- Create button

### 5. Interaction Patterns

**Desktop**:
- Horizontal scrolling (mouse wheel, trackpad)
- Click phase card → drawer slides in from right
- Hover → show tooltip with key details
- Drag to scroll timeline

**Mobile**:
- Touch scroll horizontally
- Tap phase → full-screen modal
- Swipe to dismiss
- Stacked vertically: GA timeline, then Launch timeline

### 6. Smart Features

**Overlap Visualization**:
- Launch phases that overlap shown in same vertical space
- Connected with subtle background shading
- Shows relationship to GA phase

**Date Formatting**:
- "Active since Jan 15, 2024"
- "Ends in 45 days"
- "Started 3 months ago"
- Relative dates for better understanding

**Configuration Delta**:
- When viewing past phase, show badge: "3 changes from previous"
- Click to see diff view
- Highlights what changed (prices, grace periods, etc.)

**Validation Warnings**:
- "This GA phase overlaps with..." (prevent creation)
- "No end date set - phase runs indefinitely"
- "Launch phase extends beyond GA phase"

## Implementation Phases

### Phase 1 (MVP - This Session):
✅ Create component structure
✅ Fetch and display phases
✅ Dual swimlane layout
✅ Current phase highlighted
✅ Basic click to view details
✅ Responsive (mobile-friendly)

### Phase 2 (Next Session):
- Phase creation wizard
- Policy editing
- Price/fee management
- Delete future phases

### Phase 3 (Polish):
- Drag to reorder (future phases)
- Configuration diff view
- Phase templates
- Bulk operations

## Technical Approach

### Hooks
```typescript
// Fetch all phases for a TLD
usePhasesForTLD(tldName: string)

// Separate GA and Launch
const gaPhases = phases.filter(p => p.type === 'GA')
const launchPhases = phases.filter(p => p.type === 'Launch')

// Categorize by status
const currentGAPhase = gaPhases.find(p => isActive(p))
const pastGAPhases = gaPhases.filter(p => isPast(p))
const futureGAPhases = gaPhases.filter(p => isFuture(p))
```

### Layout Strategy
- CSS Grid for dual-swimlane
- Flexbox for horizontal scrolling
- Intersection Observer for "scroll to current"
- Framer Motion for smooth transitions

### Date Utilities
```typescript
const isActive = (phase: Phase) => {
  const now = new Date()
  const start = new Date(phase.starts)
  const end = phase.ends ? new Date(phase.ends) : null
  return start <= now && (!end || end > now)
}

const isPast = (phase: Phase) => {
  if (!phase.ends) return false
  return new Date(phase.ends) < new Date()
}

const isFuture = (phase: Phase) => {
  return new Date(phase.starts) > new Date()
}
```

## Visual Style (Sunset Theme)

**Current GA Phase**:
- Border: `border-[oklch(0.7_0.15_45)]` (warm orange)
- Background: `bg-gradient-to-br from-[oklch(0.95_0.05_45)] to-[oklch(0.98_0.02_45)]`
- Glow effect with box-shadow

**Past Phases**:
- Muted gray/beige
- `opacity-60`
- Checkmark in sunset color

**Future Phases**:
- Outline style with dashed border
- Clock icon
- Lighter sunset color

**Launch Phases**:
- Smaller pills
- Different sunset shade (maybe more purple/pink)
- Badge style

## Next Steps

1. Create `PhaseTimeline.tsx` component
2. Add to TLD detail page
3. Implement basic dual-swimlane layout
4. Style current phase as focal point
5. Add click handlers for detail view
6. Test on mobile

Shall we start building?
