# TLD CRUD Implementation Plan

## Overview
Implement full CRUD (Create, Read, Update, Delete) functionality for Top-Level Domains (TLDs) in the frontend dashboard, following the same patterns established with Registry Operators.

---

## 📊 Backend API Analysis

### Available Endpoints
Based on `internal/interface/rest/tld_controller.go`:

| Method | Endpoint | Purpose | Status |
|--------|----------|---------|--------|
| GET | `/tlds` | List TLDs with pagination & filters | ✅ Available |
| GET | `/tlds/count` | Get count of TLDs | ✅ Available |
| GET | `/tlds/:tldName` | Get single TLD by name | ✅ Available |
| POST | `/tlds` | Create new TLD | ✅ Available |
| DELETE | `/tlds/:tldName` | Delete TLD by name | ✅ Available |
| POST | `/tlds/:tldName/status/:status` | Set TLD status | ✅ Available (Phase 2) |
| DELETE | `/tlds/:tldName/status` | Delete TLD status | ✅ Available (Phase 2) |

**Note**: There is NO UPDATE/PUT endpoint for TLDs. The backend only supports Create and Delete for the TLD itself. Status changes are separate operations.

### TLD Entity Structure
From `internal/domain/entities/tld.go`:

```go
type TLD struct {
    Name              DomainName  // ASCII name (A-label), e.g., "com", "xn--example"
    Type              TLDType     // "generic", "country-code", or "second-level"
    UName             DomainName  // Unicode name (U-label), auto-set for IDNs
    RyID              ClIDType    // Registry Operator ID (3-16 ASCII chars)
    AllowEscrowImport bool        // Whether escrow imports are allowed
    EnableDNS         bool        // Whether DNS is enabled
    Phases            []Phase     // TLD phases (Launch, GA, etc.)
    CreatedAt         time.Time
    UpdatedAt         time.Time
}
```

### TLD Types (Auto-detected)
- **Generic TLD (gTLD)**: > 2 chars, no dots (e.g., "com", "org", "tech")
- **Country-Code TLD (ccTLD)**: Exactly 2 chars (e.g., "uk", "jp", "us")
- **Second-Level Domain (SLD)**: Contains dots (e.g., "co.uk", "com.au")

### List/Filter Query Parameters
- `pagesize` (int): Number of results per page
- `cursor` (string): Base64-encoded cursor for pagination
- `name_like` (string): Partial match on TLD name
- `type_equals` (string): Exact match on type ("generic", "country-code", "second-level")
- `ryid_equals` (string): Exact match on Registry Operator ID

### Validation Rules

#### TLD Name
- Must be a valid domain name label(s)
- Min: 1 character
- Max: 63 characters per label
- Can be ASCII (A-label) or IDN (xn-- prefix)
- No leading/trailing hyphens
- Country-code TLDs must be exactly 2 characters
- Second-level must contain exactly one dot

#### Registry Operator ID (RyID)
- Min: 3 characters
- Max: 16 characters
- ASCII characters only
- No leading/trailing whitespace

---

## 🎯 Implementation Plan

### Phase 1: Core CRUD (This Phase)

#### 1.1 API Client Layer
**File**: `frontend/lib/api/tlds.ts` (NEW)

```typescript
import { apiClient } from './client';

export interface TLD {
  Name: string;
  Type: 'generic' | 'country-code' | 'second-level';
  UName: string;
  RyID: string;
  AllowEscrowImport: boolean;
  EnableDNS: boolean;
  Phases: any[]; // Will type properly in Phase 2
  CreatedAt: string;
  UpdatedAt: string;
}

export interface CreateTLDRequest {
  Name: string;
  RyID: string;
}

export interface ListQueryParams {
  pagesize?: number;
  cursor?: string;
  name_like?: string;
  type_equals?: 'generic' | 'country-code' | 'second-level';
  ryid_equals?: string;
}

export const tldsApi = {
  list: async (params?: ListQueryParams) => {
    const { data } = await apiClient.get('/tlds', { params });
    return data;
  },
  
  count: async (params?: ListQueryParams) => {
    const { data } = await apiClient.get('/tlds/count', { params });
    return data;
  },
  
  get: async (name: string) => {
    const { data } = await apiClient.get(`/tlds/${name}`);
    return data;
  },
  
  create: async (tld: CreateTLDRequest) => {
    const { data } = await apiClient.post('/tlds', tld);
    return data;
  },
  
  delete: async (name: string) => {
    const { data } = await apiClient.delete(`/tlds/${name}`);
    return data;
  },
};
```

**Acceptance Criteria**:
- ✅ All API methods properly typed
- ✅ Error handling via axios interceptor (already configured)
- ✅ Token authentication automatic (already configured)

---

#### 1.2 React Query Hooks
**File**: `frontend/lib/hooks/useTLDs.ts` (NEW)

```typescript
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { tldsApi, CreateTLDRequest, ListQueryParams } from '../api/tlds';
import { toast } from 'sonner';

// List TLDs with pagination/filters
export function useTLDs(params?: ListQueryParams) {
  return useQuery({
    queryKey: ['tlds', params],
    queryFn: () => tldsApi.list(params),
  });
}

// Get TLD count
export function useTLDsCount(params?: ListQueryParams) {
  return useQuery({
    queryKey: ['tlds-count', params],
    queryFn: () => tldsApi.count(params),
  });
}

// Get single TLD
export function useTLD(name: string) {
  return useQuery({
    queryKey: ['tld', name],
    queryFn: () => tldsApi.get(name),
    enabled: !!name,
  });
}

// Create TLD
export function useCreateTLD() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: (tld: CreateTLDRequest) => tldsApi.create(tld),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tlds'] });
      queryClient.invalidateQueries({ queryKey: ['tlds-count'] });
      toast.success('TLD created successfully');
    },
    onError: (error: any) => {
      toast.error(error.response?.data?.error || 'Failed to create TLD');
    },
  });
}

// Delete TLD
export function useDeleteTLD() {
  const queryClient = useQueryClient();
  
  return useMutation({
    mutationFn: (name: string) => tldsApi.delete(name),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['tlds'] });
      queryClient.invalidateQueries({ queryKey: ['tlds-count'] });
      toast.success('TLD deleted successfully');
    },
    onError: (error: any) => {
      const message = error.response?.data?.error || 'Failed to delete TLD';
      toast.error(message);
    },
  });
}
```

**Acceptance Criteria**:
- ✅ All hooks follow React Query best practices
- ✅ Proper cache invalidation on mutations
- ✅ Toast notifications for user feedback
- ✅ Error handling with backend error messages

---

#### 1.3 List Page
**File**: `frontend/app/tlds/page.tsx` (NEW)

**Features**:
- Table with columns: Name, Type, UName (if IDN), RyID, DNS Enabled, Phases Count, Actions
- Pagination support
- Search by name (debounced)
- Filter by Type dropdown
- Filter by RyID dropdown (populated from Registry Operators)
- Create button → navigates to `/tlds/create`
- Row actions: View Details, Delete (with confirmation)
- Empty state when no TLDs
- Loading skeleton

**Layout**: Similar to Registry Operators list page

**Acceptance Criteria**:
- ✅ Displays paginated TLD list
- ✅ Search works with 300ms debounce
- ✅ Filters update URL query params
- ✅ Delete shows confirmation dialog
- ✅ Responsive design
- ✅ Accessible (ARIA labels, keyboard navigation)

---

#### 1.4 Create Page
**File**: `frontend/app/tlds/create/page.tsx` (NEW)

**Form Fields**:
1. **TLD Name** * (required)
   - Input type: text
   - Placeholder: "com" or "example"
   - Validation:
     - Required
     - 1-63 characters
     - Valid domain label format
     - No leading/trailing hyphens
     - Pattern: `/^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/i`
   - Helper text: "Enter the TLD name (e.g., com, org, uk, co.uk)"

2. **Registry Operator** * (required)
   - Input type: Select/Autocomplete
   - Options: Fetch from `/registry-operators` API
   - Display: `${RyID} - ${Name}`
   - Validation: Required
   - Helper text: "Select the registry operator managing this TLD"

**Read-Only Display** (after creation):
- **Type**: Auto-detected based on name (show after validation)
- **UName**: Auto-set for IDN TLDs (show if applicable)

**Form Validation** (using Zod):
```typescript
const formSchema = z.object({
  Name: z
    .string()
    .min(1, 'TLD Name is required')
    .max(63, 'TLD Name must not exceed 63 characters')
    .regex(/^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$/i, 'Invalid TLD name format')
    .refine((val) => !val.startsWith('-') && !val.endsWith('-'), 'TLD name cannot start or end with hyphen'),
  RyID: z.string().min(1, 'Registry Operator is required'),
});
```

**Buttons**:
- Cancel (navigate back)
- Create TLD (submit)

**Acceptance Criteria**:
- ✅ Form validation works client-side
- ✅ Shows real-time validation errors
- ✅ Submits to backend with proper error handling
- ✅ Redirects to `/tlds` on success
- ✅ Shows TLD type preview as user types
- ✅ Accessible form with proper labels

---

#### 1.5 Detail/View Page
**File**: `frontend/app/tlds/[name]/page.tsx` (NEW)

**Sections**:

1. **TLD Information Card**
   - Name (with badge for type: gTLD/ccTLD/SLD)
   - UName (if IDN)
   - Type (with colored badge)
   - Registry Operator (RyID with link to operator details)
   - DNS Enabled (Yes/No badge)
   - Escrow Import Allowed (Yes/No)
   - Created At
   - Updated At

2. **Phases Card** (Read-only for Phase 1)
   - Count of phases
   - "Phases management coming soon" message
   - Link to future phases page

3. **Actions**
   - Delete TLD (with confirmation)
   - Back to List button

**Acceptance Criteria**:
- ✅ Displays all TLD details
- ✅ Shows loading state while fetching
- ✅ Handles 404 for non-existent TLD
- ✅ Delete action with confirmation
- ✅ Breadcrumb navigation
- ✅ Responsive cards

---

#### 1.6 Dashboard Integration
**File**: `frontend/app/page.tsx` (UPDATE)

**Add new dashboard card**:
```typescript
{
  title: "TLDs",
  value: tldCountData?.Count?.toString() ?? '0',
  description: "Active top-level domains",
  icon: Globe, // from lucide-react
  href: "/tlds",
}
```

**Acceptance Criteria**:
- ✅ TLD count card displays correctly
- ✅ Card links to `/tlds` list page
- ✅ Shows loading state ("...")
- ✅ Matches existing card styling

---

#### 1.7 Navigation Menu
**File**: `frontend/components/layout/DashboardLayout.tsx` (UPDATE)

**Add TLDs menu item**:
```typescript
{
  title: "TLDs",
  href: "/tlds",
  icon: Globe, // from lucide-react
}
```

**Acceptance Criteria**:
- ✅ TLD menu item shows in sidebar
- ✅ Active state highlights when on TLD pages
- ✅ Icon displays correctly

---

### Phase 2: Advanced Features (Future)

**Not included in this phase**:
- Phase management (create, edit, delete phases for a TLD)
- Status management (set/delete TLD status)
- DNS record management
- Escrow import features
- Edit TLD details (only Create/Delete in Phase 1)
- Registrar accreditation management

**These will be separate tickets after Phase 1 is complete.**

---

## 🧪 Testing Checklist

### Manual Testing
- [ ] Create TLD with generic name (e.g., "test")
- [ ] Create TLD with ccTLD name (e.g., "zz")
- [ ] Create TLD with SLD name (e.g., "co.test")
- [ ] Create TLD with IDN name (e.g., "xn--example")
- [ ] Attempt to create duplicate TLD (should fail)
- [ ] Attempt to create TLD with invalid name (should fail client-side)
- [ ] List TLDs with pagination
- [ ] Search TLDs by name
- [ ] Filter TLDs by type
- [ ] Filter TLDs by RyID
- [ ] View TLD details
- [ ] Delete TLD without phases (should succeed)
- [ ] Attempt to delete TLD with active phases (should fail with error message)
- [ ] Dashboard TLD count updates after create/delete
- [ ] Responsive layout on mobile/tablet/desktop

### Validation Testing
- [ ] Empty TLD name → error
- [ ] TLD name < 1 char → error
- [ ] TLD name > 63 chars → error
- [ ] TLD name with leading hyphen → error
- [ ] TLD name with trailing hyphen → error
- [ ] TLD name with invalid characters → error
- [ ] No Registry Operator selected → error
- [ ] Valid inputs → success

---

## 📁 File Structure

```
frontend/
├── app/
│   ├── tlds/
│   │   ├── page.tsx                    # List page (NEW)
│   │   ├── create/
│   │   │   └── page.tsx                # Create page (NEW)
│   │   └── [name]/
│   │       └── page.tsx                # Detail/View page (NEW)
│   └── page.tsx                        # Dashboard (UPDATE - add TLD card)
├── lib/
│   ├── api/
│   │   └── tlds.ts                     # API client (NEW)
│   └── hooks/
│       └── useTLDs.ts                  # React Query hooks (NEW)
└── components/
    └── layout/
        └── DashboardLayout.tsx         # Nav menu (UPDATE - add TLD link)
```

---

## 🎨 UI/UX Design Notes

### TLD Type Badges
- **gTLD**: Blue badge (generic)
- **ccTLD**: Green badge (country-code)
- **SLD**: Purple badge (second-level)

### DNS Enabled Badge
- **Enabled**: Green checkmark badge
- **Disabled**: Gray X badge

### Phases Count Display
- Show count as badge: "3 phases"
- If 0 phases: "No phases" (muted)

### Delete Confirmation
```
Are you sure you want to delete the TLD "com"?

This action cannot be undone. This will permanently delete the TLD
and all associated data.

Note: TLDs with active phases cannot be deleted.
```

---

## ⚡ Performance Considerations

1. **Debounced Search**: 300ms delay on name search
2. **Pagination**: Default 20 items per page
3. **Registry Operator Dropdown**: Cache the list, only fetch once
4. **Optimistic Updates**: Not needed for TLDs (server is source of truth)

---

## 🔒 Security Notes

- All API calls use existing Bearer token authentication
- No sensitive data in TLD entity
- Delete operation has server-side validation (no delete with active phases)
- Form validation client + server side

---

## 🚀 Rollout Plan

### Step 1: Backend Verification (30 min)
- [ ] Start backend services: `make dev`
- [ ] Test GET `/tlds` endpoint
- [ ] Test GET `/tlds/count` endpoint
- [ ] Test POST `/tlds` endpoint
- [ ] Test DELETE `/tlds/:name` endpoint
- [ ] Verify error responses

### Step 2: API Client & Hooks (1 hour)
- [ ] Create `lib/api/tlds.ts`
- [ ] Create `lib/hooks/useTLDs.ts`
- [ ] Test hooks with existing backend data

### Step 3: List Page (2 hours)
- [ ] Create `app/tlds/page.tsx`
- [ ] Implement table with pagination
- [ ] Add search and filters
- [ ] Add delete action
- [ ] Test with real data

### Step 4: Create Page (2 hours)
- [ ] Create `app/tlds/create/page.tsx`
- [ ] Build form with validation
- [ ] Integrate Registry Operator dropdown
- [ ] Add TLD type preview
- [ ] Test form submission

### Step 5: Detail Page (1.5 hours)
- [ ] Create `app/tlds/[name]/page.tsx`
- [ ] Display all TLD information
- [ ] Add delete action
- [ ] Handle loading and errors

### Step 6: Dashboard Integration (30 min)
- [ ] Update `app/page.tsx` with TLD card
- [ ] Update `DashboardLayout.tsx` with nav item
- [ ] Test navigation flow

### Step 7: Testing & Polish (1 hour)
- [ ] Run through manual testing checklist
- [ ] Fix any bugs found
- [ ] Test responsive design
- [ ] Test accessibility

**Total Estimated Time**: 8.5 hours

---

## 📝 Notes

- **No Update Endpoint**: The backend doesn't have an update/PUT endpoint for TLDs. Once created, only deletion is supported for the TLD itself. Status changes and phase management are separate operations.
- **Type Auto-Detection**: TLD type is automatically determined by the backend based on the name, not user input.
- **UName Auto-Set**: For IDN TLDs (xn-- prefix), the Unicode name is automatically set by the backend.
- **Phases**: Phase management is explicitly out of scope for Phase 1. The detail page will show phase count and a "coming soon" message.

---

## ✅ Definition of Done

- [ ] All 7 rollout steps completed
- [ ] Manual testing checklist 100% passed
- [ ] Code follows existing patterns (Registry Operators)
- [ ] No TypeScript errors
- [ ] No console errors/warnings
- [ ] Responsive on mobile, tablet, desktop
- [ ] Accessible (keyboard navigation, ARIA labels)
- [ ] Documentation updated (if needed)
- [ ] Ready for code review

