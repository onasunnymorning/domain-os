# Registrar API Response Structure Fix

## Issue

**Error:** `Cannot read properties of undefined (reading 'length')`

**Cause:** The TypeScript types and component code were using lowercase property names (`data`, `meta`) but the actual API response uses capitalized property names (`Data`, `Meta`).

## Root Cause

The backend API follows Go naming conventions where exported fields are capitalized:

```go
type ListItemResult struct {
    Data interface{}
    Meta *MetaData
}
```

This translates to JSON as:
```json
{
  "Data": [...],
  "Meta": {
    "Cursor": "...",
    "Count": 123,
    "PageSize": 50
  }
}
```

But the frontend types were defined with lowercase:
```typescript
interface IANARegistrarListResponse {
  data: IANARegistrar[];  // ❌ Wrong - should be Data
  meta?: { ... };          // ❌ Wrong - should be Meta
}
```

## Files Fixed

### 1. Type Definitions

**File:** `frontend/lib/types/registrar.ts`

**Changes:**
- ✅ Changed `data` → `Data`
- ✅ Changed `meta` → `Meta`
- ✅ Changed nested meta properties to use capital case (`Cursor`, `Count`, `PageSize`, `NextLink`)

**Before:**
```typescript
export interface IANARegistrarListResponse {
  data: IANARegistrar[];
  meta?: {
    cursor: string;
    count: number;
    pageSize: number;
    nextLink?: string;
  };
}
```

**After:**
```typescript
export interface IANARegistrarListResponse {
  Data: IANARegistrar[];
  Meta?: {
    Cursor: string;
    Count: number;
    PageSize: number;
    NextLink?: string;
  };
}
```

### 2. IANA Registrars Tab Component

**File:** `frontend/components/registrars/iana-registrars-tab.tsx`

**Changes:**
- ✅ Updated all references from `data.data` → `data.Data`
- ✅ Updated all references from `data.meta` → `data.Meta`
- ✅ Added null check: `!data.Data || data.Data.length === 0`

**Before:**
```typescript
{data.data.length === 0 ? (
  // ...
) : (
  // ...
  {data.data.map((registrar) => (
    // ...
  ))}
)}

{data.meta && data.data.length > 0 && (
  <div>Showing {data.data.length} registrars</div>
)}
```

**After:**
```typescript
{!data.Data || data.Data.length === 0 ? (
  // ...
) : (
  // ...
  {data.Data.map((registrar) => (
    // ...
  ))}
)}

{data.Meta && data.Data && data.Data.length > 0 && (
  <div>Showing {data.Data.length} registrars</div>
)}
```

### 3. System Registrars Tab Component

**File:** `frontend/components/registrars/system-registrars-tab.tsx`

**Changes:**
- ✅ Same updates as IANA tab
- ✅ Updated all `data.data` → `data.Data`
- ✅ Updated all `data.meta` → `data.Meta`

## Verification

The API response structure matches other endpoints in the application:

**TLDs API:**
```typescript
const tlds = data?.Data || [];  // Uses Data with capital D
```

**Registry Operators API:**
```typescript
const operators = data?.Data || [];  // Uses Data with capital D
```

**Registrars API (now fixed):**
```typescript
const registrars = data?.Data || [];  // Uses Data with capital D ✅
```

## Testing

After this fix:
1. ✅ Page loads without errors
2. ✅ IANA registrars display correctly
3. ✅ System registrars display correctly
4. ✅ Empty state shows when no data
5. ✅ Count displays at bottom of table

## Consistency

All API responses in the application now consistently use:
- `Data` for the array of items (capital D)
- `Meta` for pagination metadata (capital M)
- Nested properties like `Cursor`, `Count`, `PageSize` (capital first letter)

This matches the Go backend's JSON serialization conventions.
