# Price Editing Implementation

## Overview
Added complete add/remove workflow for phase prices in the PhaseDetailDrawer component, allowing users to add new currency pricing and remove existing ones.

## Changes Made

### 1. API Layer (`frontend/lib/api/phases.ts`)
Added two new functions:

```typescript
addPrice: async (tldName: string, phaseName: string, price: {...}): Promise<Phase['prices'][0]>
```
- Endpoint: `POST /tlds/{tldName}/phases/{phaseName}/prices`
- Payload: `{ currency, registrationAmount, renewalAmount, transferAmount, restoreAmount }`
- Returns: Created Price entity

```typescript
deletePrice: async (tldName: string, phaseName: string, currency: string): Promise<void>
```
- Endpoint: `DELETE /tlds/{tldName}/phases/{phaseName}/prices/{currency}`
- No response body

### 2. React Query Hooks (`frontend/lib/hooks/usePhases.ts`)

**useAddPrice**
- Accepts price object with all 4 amounts
- Invalidates phase and list queries on success

**useDeletePrice**
- Accepts currency code
- Invalidates phase and list queries on success

### 3. UI Component (`frontend/components/phases/PhaseDetailDrawer.tsx`)

#### State Management
- `isEditingPrices`: Boolean to toggle edit mode
- `newPrice`: Form state for adding new currency with 5 fields (currency + 4 amounts)

#### Handlers
- `handleEditPrices()`: Enters edit mode
- `handleCancelEditPrices()`: Exits edit mode, clears form
- `handleAddPrice()`: Validates and submits new price
- `handleDeletePrice(currency)`: Removes existing price

#### UI Features

**Edit Button**
- Appears below "Pricing" heading when not editing
- Only visible when section is expanded

**Read Mode**
- Shows all existing prices with 4 amounts in 2x2 grid
- Currency code displayed as header
- Amounts shown with proper currency symbols

**Edit Mode**
- Each existing price gets a "Remove" button
- Add new currency form appears at bottom with:
  - Currency code input (3-char, auto-uppercase)
  - 4 amount inputs (registration, renewal, transfer, restore)
  - All amounts in cents
  - "Add Currency" button (disabled until form valid)
  - "Done" button to exit edit mode

**Action Buttons**
- Add Currency: Validates all fields, shows loading state
- Remove: Appears next to each currency in edit mode
- Done: Exits edit mode (replaces Cancel, simpler UX)

## Price Structure

All amounts are in cents (smallest currency unit):
- **registrationAmount**: Cost to register a domain
- **renewalAmount**: Cost to renew a domain
- **transferAmount**: Cost to transfer a domain
- **restoreAmount**: Cost to restore a deleted domain

## User Experience

**Workflow:**
1. User opens phase detail drawer
2. Expands Pricing accordion
3. Clicks "Edit" button
4. Remove buttons appear next to existing prices
5. Add new currency form appears at bottom
6. User can:
   - Remove existing currencies
   - Add new currencies with all 4 amounts
7. Click "Done" to exit edit mode
8. Data refreshes automatically

**UX Highlights:**
- Edit button positioned like Policy section
- Remove buttons only in edit mode (clean read view)
- Form validation prevents incomplete submissions
- Auto-uppercase currency codes
- Instant refetch on add/remove
- Clear visual separation between existing and new
- Grid layout maintains consistency with read mode

## Backend Integration

**POST /tlds/{tldName}/phases/{phaseName}/prices**
- Validates phase exists and allows updates
- Checks currency doesn't already exist
- Returns 400 if duplicate currency
- Returns 404 if TLD/phase not found

**DELETE /tlds/{tldName}/phases/{phaseName}/prices/{currency}**
- Validates phase exists and allows updates
- Case-insensitive currency matching
- Returns 404 if not found

## Cache Management

Both mutations invalidate:
- Specific phase query: `['phase', tldName, phaseName]`
- List query: `['phases', tldName]`

This ensures drawer and list views stay in sync.

## Key Differences from Policy Editing

1. **No Update**: Prices can only be added or removed, not modified in place
2. **Multiple Items**: Can have many prices (one per currency)
3. **Complex Form**: 5 fields needed for each new price
4. **Immediate Action**: Add/remove happens immediately, no "Save" button
5. **Done vs Cancel**: Simpler "Done" button since changes are already saved

## Validation

**Frontend:**
- Currency code: Required, max 3 chars, auto-uppercase
- All amounts: Required, must be positive numbers
- Add button disabled until all fields valid

**Backend:**
- Currency uniqueness enforced
- Phase must allow updates
- All amounts must be positive integers

## Testing Considerations

1. **Add Price**: Verify new currency appears immediately
2. **Remove Price**: Verify currency removed from list
3. **Duplicate Currency**: Try adding same currency twice
4. **Form Validation**: Test incomplete forms
5. **Case Sensitivity**: Test currency codes in mixed case
6. **Cache Updates**: Verify list view reflects changes
7. **Edit Mode**: Verify Done button exits properly
8. **Loading States**: Check disabled states during operations

## Future Enhancements

Potential improvements:
- Bulk add multiple currencies
- Copy pricing from another phase
- Import/export pricing CSV
- Price history/audit log
- Currency conversion calculator
- Preset pricing templates
- Inline editing of amounts (requires backend update endpoint)
- Confirmation dialog for remove
