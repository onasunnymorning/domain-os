# Fee Editing Implementation

## Overview
Added complete add/remove workflow for phase fees in the PhaseDetailDrawer component, allowing users to add new fees and remove existing ones.

## Changes Made

### 1. API Layer (`frontend/lib/api/phases.ts`)
Added two new functions:

```typescript
addFee: async (tldName: string, phaseName: string, fee: {...}): Promise<Phase['fees'][0]>
```
- Endpoint: `POST /tlds/{tldName}/phases/{phaseName}/fees`
- Payload: `{ name, currency, amount, refundable }`
- Returns: Created Fee entity

```typescript
deleteFee: async (tldName: string, phaseName: string, feeName: string, currency: string): Promise<void>
```
- Endpoint: `DELETE /tlds/{tldName}/phases/{phaseName}/fees/{feeName}/{currency}`
- No response body

### 2. React Query Hooks (`frontend/lib/hooks/usePhases.ts`)

**useAddFee**
- Accepts fee object with name, currency, amount, and refundable
- Invalidates phase and list queries on success

**useDeleteFee**
- Accepts feeName and currency
- Invalidates phase and list queries on success

### 3. UI Component (`frontend/components/phases/PhaseDetailDrawer.tsx`)

#### State Management
- `isEditingFees`: Boolean to toggle edit mode
- `newFee`: Form state for adding new fee with 4 fields

#### Handlers
- `handleEditFees()`: Enters edit mode
- `handleCancelEditFees()`: Exits edit mode, clears form
- `handleAddFee()`: Validates and submits new fee
- `handleDeleteFee(feeName, currency)`: Removes existing fee

#### UI Features

**Edit Button**
- Appears below "Fees" heading when not editing
- Only visible when section is expanded
- Matches pattern from Pricing and Policy sections

**Read Mode**
- Shows all existing fees with name, currency, amount, and refundable status
- Amount shown with proper currency symbols (converted from cents)
- Refundable indicator shown as " • Refundable" suffix

**Edit Mode**
- Each existing fee gets a "Remove" button (right-aligned)
- Add new fee form appears at bottom with:
  - Fee name input (text)
  - Currency code input (3-char, auto-uppercase)
  - Amount input (in cents)
  - Refundable checkbox
  - "Add Fee" button (disabled until form valid)
  - "Done" button to exit edit mode

**Action Buttons**
- Add Fee: Validates all fields, shows loading state
- Remove: Appears next to each fee in edit mode
- Done: Exits edit mode

## Fee Structure

Fees have these fields:
- **name**: Identifier for the fee (e.g., "application_fee")
- **currency**: 3-letter currency code (e.g., "USD")
- **amount**: Cost in cents (smallest currency unit)
- **refundable**: Boolean indicating if fee can be refunded

## Backend Integration

**POST /tlds/{tldName}/phases/{phaseName}/fees**
- Fee identified by name + currency tuple (composite key)
- Validates phase exists and allows updates
- Returns 400 if fee with same name+currency exists
- Returns 404 if TLD/phase not found
- Currency automatically uppercased

**DELETE /tlds/{tldName}/phases/{phaseName}/fees/{feeName}/{currency}**
- Validates phase exists and allows updates
- Case-insensitive currency matching
- Returns 204 even if fee doesn't exist (idempotent)
- Returns 404 if TLD/phase not found

## User Experience

**Workflow:**
1. User opens phase detail drawer
2. Expands Fees accordion
3. Clicks "Edit" button
4. Remove buttons appear next to existing fees
5. Add new fee form appears at bottom
6. User can:
   - Remove existing fees
   - Add new fees with name, currency, amount, refundable
7. Click "Done" to exit edit mode
8. Data refreshes automatically

**UX Highlights:**
- Edit button positioned consistently with Pricing/Policy
- Remove buttons only in edit mode (clean read view)
- Form validation prevents incomplete submissions
- Auto-uppercase currency codes
- Checkbox for refundable (simpler than toggle)
- Instant refetch on add/remove
- Clear visual separation between existing and new
- Amount display with currency symbols

## Key Differences from Price Editing

1. **Composite Key**: Fees identified by name + currency (not just currency)
2. **Simpler Form**: Only 4 fields vs 5 for prices
3. **Checkbox vs Toggle**: Refundable uses checkbox (boolean input)
4. **Fee Name**: Must specify a name for each fee
5. **Single Amount**: Only one amount field (not 4 like prices)

## Cache Management

Both mutations invalidate:
- Specific phase query: `['phase', tldName, phaseName]`
- List query: `['phases', tldName]`

This ensures drawer and list views stay in sync.

## Validation

**Frontend:**
- Fee name: Required, text input
- Currency code: Required, max 3 chars, auto-uppercase
- Amount: Required, positive number
- Refundable: Boolean checkbox
- Add button disabled until name, currency, and amount are filled

**Backend:**
- Fee name + currency uniqueness enforced
- Phase must allow updates
- Amount must be positive integer
- Refundable defaults to false if omitted

## Testing Considerations

1. **Add Fee**: Verify new fee appears immediately
2. **Remove Fee**: Verify fee removed from list
3. **Duplicate Fee**: Try adding same name+currency twice
4. **Form Validation**: Test incomplete forms
5. **Case Sensitivity**: Test currency codes in mixed case
6. **Refundable Toggle**: Test checkbox state
7. **Cache Updates**: Verify list view reflects changes
8. **Edit Mode**: Verify Done button exits properly
9. **Loading States**: Check disabled states during operations
10. **Delete Key**: Verify both feeName and currency are needed

## Pattern Consistency

All three editing features (Policy, Pricing, Fees) now follow the same pattern:
- Edit button appears below section title when expanded
- Edit mode shows action buttons (Remove/Delete)
- Add forms appear at bottom in edit mode
- Done button exits edit mode
- Changes happen immediately with auto-refetch
- Loading states on all mutation buttons

## Future Enhancements

Potential improvements:
- Bulk add multiple fees
- Copy fees from another phase
- Fee templates/presets
- Fee history/audit log
- Inline editing of amounts (requires backend update endpoint)
- Confirmation dialog for remove
- Fee name suggestions/autocomplete
- Currency selector dropdown
- Amount calculator/converter
