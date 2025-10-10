# Policy Editing Implementation

## Overview
Implemented a complete edit workflow for phase policies in the PhaseDetailDrawer component, allowing users to modify all policy fields through an intuitive UI.

## Changes Made

### 1. API Layer (`frontend/lib/api/phases.ts`)
Added new function to update phase policy:
```typescript
updatePolicy: async (tldName: string, phaseName: string, policy: Phase['policy']): Promise<Phase>
```
- Endpoint: `PUT /tlds/{tldName}/phases/{phaseName}/policy`
- Payload: `{ Policy: policy }`
- Returns: Updated Phase entity

### 2. React Query Hook (`frontend/lib/hooks/usePhases.ts`)
Added `useUpdatePhasePolicy` mutation hook:
- Accepts policy object
- Invalidates both specific phase and list queries on success
- Returns mutation with pending state

### 3. UI Component (`frontend/components/phases/PhaseDetailDrawer.tsx`)

#### State Management
- `isEditingPolicy`: Boolean to toggle edit mode
- `editedPolicy`: Temporary copy of policy being edited

#### Handlers
- `handleEditPolicy()`: Enters edit mode, clones policy
- `handleCancelEditPolicy()`: Exits edit mode, discards changes
- `handleSavePolicy()`: Calls mutation, resets edit mode on success
- `handlePolicyChange()`: Updates individual policy fields

#### UI Features

**Edit Button**
- Appears in Policy accordion header when not editing
- Positioned on the right side, stops event propagation to prevent accordion toggle

**Read Mode**
- Visual label length slider (1-63 chars range)
- Grace periods in 2-column grid
- Toggle switches for boolean values
- Currency symbol display for base currency

**Edit Mode**
- Number inputs for all numeric fields:
  - Label lengths (min/max)
  - 6 grace periods (registration, renewal, autoRenewal, transfer, redemption, pendingdelete)
  - Transfer lock period
  - Max horizon
- Interactive toggle switches for:
  - Allow autorenew
  - Requires validation
- Text input for base currency (3-char uppercase)

**Action Buttons**
- Save Changes: Calls mutation, shows loading state
- Cancel: Discards changes, exits edit mode
- Both disabled while saving

## Policy Fields Covered

### Numeric Fields (9)
1. `minLabelLength` (1-63)
2. `maxLabelLength` (1-63)
3. `registrationGP` (days)
4. `renewalGP` (days)
5. `autoRenewalGP` (days)
6. `transferGP` (days)
7. `redemptionGP` (days)
8. `pendingdeleteGP` (days)
9. `transferLockPeriod` (days)
10. `maxHorizon` (years)

### Boolean Fields (2)
1. `allowAutorenew`
2. `requiresValidation`

### String Field (1)
1. `baseCurrency` (3-char code, auto-uppercased)

## User Experience

**Workflow:**
1. User opens phase detail drawer
2. Expands Policy accordion section
3. Clicks "Edit" button in accordion header
4. Form appears with all current values
5. User modifies desired fields
6. Clicks "Save Changes" to persist (or "Cancel" to discard)
7. On success: form closes, data refreshes
8. On error: form stays open, error shown

**UX Highlights:**
- Edit button in accordion header (doesn't interfere with expand/collapse)
- Smooth transition between read and edit modes
- Preserves visual design (2-column grid layout)
- Disabled state during save operation
- Instant validation (min/max attributes on number inputs)
- Auto-uppercase for currency codes

## Backend Integration

The backend endpoint validates:
- Phase must allow updates (`phase.CanUpdate()`)
- Typically only future/current phases can be edited
- Past phases are locked for traceability

## Cache Management

On successful save:
- Invalidates specific phase query: `['phases', tldName, phaseName]`
- Invalidates list query: `['phases', tldName]`
- Ensures both drawer and list view reflect changes

## Testing Considerations

1. **Permission Testing**: Verify only updatable phases show edit functionality
2. **Field Validation**: Test numeric bounds (e.g., label length 1-63)
3. **Toggle Behavior**: Verify boolean fields toggle correctly
4. **Currency Format**: Ensure 3-char uppercase enforcement
5. **Cancel Behavior**: Verify changes are discarded
6. **Save Behavior**: Verify changes persist and UI updates
7. **Loading States**: Check disabled state during save
8. **Error Handling**: Verify error messages display properly

## Future Enhancements

Potential improvements:
- Form validation before save (e.g., min <= max label length)
- Dirty state indicator (unsaved changes warning)
- Success toast notification
- Error toast for failed saves
- Optimistic updates for instant feedback
- Field-level error messages
- Currency dropdown instead of text input
- Grace period presets or recommendations
