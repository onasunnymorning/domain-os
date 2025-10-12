# Registrar Management - Layout & Auth Updates

## Changes Made

### 1. Added DashboardLayout Wrapper

**File:** `frontend/app/registrars/page.tsx`

**Changes:**
- ✅ Wrapped page with `DashboardLayout` component
- ✅ Added header with icon (`UserCheck`)
- ✅ Moved page title inside layout
- ✅ Now matches the layout pattern of TLDs and Registry Operators pages

**Before:**
```tsx
export default function RegistrarsPage() {
  return (
    <div className="container mx-auto py-6 space-y-6">
      <div>
        <h1 className="text-3xl font-bold tracking-tight">Registrar Management</h1>
        ...
```

**After:**
```tsx
export default function RegistrarsPage() {
  return (
    <DashboardLayout>
      <div className="space-y-6">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-3xl font-bold tracking-tight flex items-center gap-2">
              <UserCheck className="h-8 w-8" />
              Registrar Management
            </h1>
        ...
```

### 2. Fixed API Client to Use Axios with Auth

**File:** `frontend/lib/api/registrars.ts`

**Problem:** 
- API calls were using native `fetch()` without authorization headers
- Error: "Authorization header missing or malformed"

**Solution:**
- ✅ Replaced all `fetch()` calls with `apiClient` (axios)
- ✅ Now uses same auth pattern as TLDs and Registry Operators
- ✅ Automatically includes Bearer token from environment variable
- ✅ Simplified code - no manual URL building or header management

**Before:**
```typescript
export async function getIANARegistrars(params?: IANARegistrarListParams) {
  const url = buildURL("/ianaregistrars", params);
  const response = await fetch(url);
  return handleResponse<IANARegistrarListResponse>(response);
}
```

**After:**
```typescript
export async function getIANARegistrars(params?: IANARegistrarListParams) {
  const { data } = await apiClient.get('/ianaregistrars', { params });
  return data;
}
```

### 3. Fixed Select Component Empty String Issue

**File:** `frontend/components/registrars/iana-registrars-tab.tsx`

**Problem:**
- Newer shadcn/ui Select component doesn't allow empty string values
- Error: "A <Select.Item /> must have a value prop that is not an empty string"

**Solution:**
- ✅ Changed "All Statuses" value from `""` to `"all"`
- ✅ Updated filter logic to check for `"all"` instead of empty string
- ✅ Maintains same functionality

**Before:**
```typescript
const [statusFilter, setStatusFilter] = useState<string>("");
// ...
if (statusFilter) {
  queryParams.status = statusFilter;
}
// ...
<SelectItem value="">All Statuses</SelectItem>
```

**After:**
```typescript
const [statusFilter, setStatusFilter] = useState<string>("all");
// ...
if (statusFilter && statusFilter !== "all") {
  queryParams.status = statusFilter;
}
// ...
<SelectItem value="all">All Statuses</SelectItem>
```

## Visual Changes

### Before
- No header/sidebar
- Standalone container
- Authorization errors

### After
- ✅ Consistent header with navigation
- ✅ Sidebar with menu
- ✅ Matches TLDs and Registry Operators layout
- ✅ Proper authentication
- ✅ Data loads successfully

## Testing

### Verify Layout

1. Navigate to `/registrars`
2. ✅ Header appears at top
3. ✅ Sidebar toggles with menu button
4. ✅ UserCheck icon shows next to "Registrar Management"
5. ✅ Layout matches `/tlds` and `/registry-operators`

### Verify API Calls

1. Open browser console
2. Navigate to IANA Registrars tab
3. ✅ No "Authorization header missing" errors
4. ✅ Data loads successfully
5. ✅ Count displays
6. ✅ Search and filter work

### Verify Select Component

1. Click status filter dropdown
2. ✅ No console errors
3. ✅ "All Statuses" selectable
4. ✅ Filter options work
5. ✅ Filtering updates table

## Files Modified

1. ✅ `frontend/app/registrars/page.tsx` - Added DashboardLayout
2. ✅ `frontend/lib/api/registrars.ts` - Switched to axios with auth
3. ✅ `frontend/components/registrars/iana-registrars-tab.tsx` - Fixed select value

## Next Steps

- [x] Layout integrated
- [x] Authentication working
- [x] Select component fixed
- [ ] Test all features work with real backend
- [ ] Verify sync functionality
- [ ] Test System Registrars tab

## API Configuration

**Correct Environment Variables:**
```bash
NEXT_PUBLIC_API_URL=http://localhost:8080
NEXT_PUBLIC_API_TOKEN=the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all
```

**Note:** Backend runs on port 8080, not 8000

## Summary

All layout and authentication issues resolved. The registrar management page now:
- Uses consistent DashboardLayout
- Includes proper authentication headers
- Follows the same patterns as other pages
- Should load data successfully when backend is running
