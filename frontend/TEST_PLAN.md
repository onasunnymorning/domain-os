# Frontend CRUD Testing Plan

## Current Status
✅ Backend API running on: http://localhost:8080  
✅ Frontend running on: http://localhost:3000  
✅ Database: PostgreSQL (via Docker Compose)  
✅ Current Registry Operators: 0

## Authentication
- Token: `the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all`
- Stored in: `frontend/.env.local` as `NEXT_PUBLIC_API_TOKEN`
- Currently bypassing auth (mock login will be added in Phase 4)

## Test Scenarios

### 1. Create Registry Operator ✅
**URL**: http://localhost:3000/registry-operators/create

**Test Data**:
- RyID: `TEST-RY-001`
- Name: `Test Registry Inc.`
- Email: `contact@testregistry.com`
- URL: `https://testregistry.com`
- Phone: `+1.1234567890`
- Fax: `+1.0987654321`

**Expected Result**:
- Form validation passes
- API creates the operator
- Toast notification: "Registry operator created successfully"
- Redirect to detail page: `/registry-operators/TEST-RY-001`

**API Endpoint**: `POST /registry-operators`

### 2. List Registry Operators ✅
**URL**: http://localhost:3000/registry-operators

**Expected Result**:
- Table shows the created operator
- Search box functional
- Create button visible
- Actions column has View and Delete buttons

**Test Search**:
- Search for "Test" → Should show TEST-RY-001
- Search for "contact@test" → Should show TEST-RY-001
- Search for "nonexistent" → Should show empty state

**API Endpoint**: `GET /registry-operators?search={query}`

### 3. View Registry Operator Details ✅
**URL**: http://localhost:3000/registry-operators/TEST-RY-001

**Expected Result**:
- RyID badge displayed
- All contact info shown (email, URL as clickable link, phone, fax)
- Created/Updated timestamps visible
- Edit and Delete buttons present
- Back navigation works

**API Endpoint**: `GET /registry-operators/{ryid}`

### 4. Edit Registry Operator ✅
**URL**: http://localhost:3000/registry-operators/TEST-RY-001/edit

**Test Changes**:
- Name: `Test Registry Inc. (Updated)`
- Email: `updated@testregistry.com`
- URL: `https://updated.testregistry.com`

**Expected Result**:
- Form pre-filled with existing data
- RyID field is disabled (can't change primary key)
- Form validation works
- API updates the operator
- Toast notification: "Registry operator updated successfully"
- Redirect back to detail page
- Updated values displayed

**API Endpoint**: `PUT /registry-operators/{ryid}`

### 5. Delete Registry Operator ✅
**Test from List Page**:
1. Go to http://localhost:3000/registry-operators
2. Click Delete button on TEST-RY-001
3. Confirm deletion in alert dialog
4. Operator removed from list

**Test from Detail Page**:
1. Go to http://localhost:3000/registry-operators/TEST-RY-001
2. Click Delete button
3. Confirm deletion in alert dialog
4. Redirect to list page
5. Operator not in list

**Expected Result**:
- Confirmation dialog appears
- API deletes the operator
- Toast notification: "Registry operator deleted successfully"
- Operator removed from database

**API Endpoint**: `DELETE /registry-operators/{ryid}`

## Validation Tests

### Form Validation (Create & Edit Pages)

**RyID Validation** (Create page only):
- ❌ Empty → "RyID is required"
- ❌ `test-123` → "RyID must contain only uppercase letters, numbers, and hyphens"
- ✅ `TEST-RY-001` → Valid

**Name Validation**:
- ❌ Empty → "Name is required"
- ❌ `AB` → "Name must be at least 3 characters"
- ✅ `Test Registry Inc.` → Valid

**Email Validation**:
- ❌ Empty → "Email is required"
- ❌ `notanemail` → "Invalid email address"
- ✅ `contact@testregistry.com` → Valid

**URL Validation** (Optional):
- ✅ Empty → Valid (optional field)
- ❌ `not-a-url` → "Invalid URL"
- ✅ `https://testregistry.com` → Valid

**Phone/Fax Validation** (Optional):
- ✅ Empty → Valid (optional)
- ✅ `+1.1234567890` → Valid
- ✅ Any format accepted (backend validates E.164)

## Error Handling Tests

### Network Errors
1. Stop Docker containers: `docker compose down`
2. Try to create/edit/delete → Should show error toast
3. Try to load list/detail → Should show error state

### API Errors
1. Invalid token → 401 Unauthorized
2. Non-existent RyID → 404 Not Found
3. Duplicate RyID → 409 Conflict (or appropriate error)

### Edge Cases
1. Very long names (>255 chars)
2. Special characters in RyID
3. Multiple rapid deletions
4. Concurrent edits

## Testing Checklist

- [ ] Create a new Registry Operator
- [ ] Verify it appears in the list
- [ ] Search for it by name
- [ ] Search for it by email
- [ ] View detail page
- [ ] Edit the operator
- [ ] Verify changes saved
- [ ] Delete from detail page
- [ ] Create another operator
- [ ] Delete from list page
- [ ] Test all validation rules
- [ ] Test error states (API down)
- [ ] Test loading states
- [ ] Test mobile responsiveness
- [ ] Test navigation flows

## Quick Test Commands

```bash
# Check API is running
curl http://localhost:8080/ping

# Get token from Doppler
doppler secrets get ADMIN_TOKEN --plain

# List registry operators
curl -H "Authorization: Bearer the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all" \
  http://localhost:8080/registry-operators | jq '.'

# Create via API (for testing)
curl -X POST http://localhost:8080/registry-operators \
  -H "Authorization: Bearer the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all" \
  -H "Content-Type: application/json" \
  -d '{
    "RyID": "TEST-API-001",
    "Name": "API Test Registry",
    "Email": "api@test.com",
    "URL": {"value": "https://api.test.com"},
    "Voice": {"value": "+1.5555555555"}
  }'

# Get specific operator
curl -H "Authorization: Bearer the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all" \
  http://localhost:8080/registry-operators/TEST-API-001 | jq '.'

# Delete via API
curl -X DELETE http://localhost:8080/registry-operators/TEST-API-001 \
  -H "Authorization: Bearer the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all"
```

## Next Steps After Testing

Once CRUD is validated:
1. **Phase 4**: Implement mock authentication (login page, protected routes)
2. **Phase 5**: Add polish (loading skeletons, pagination, optimistic updates)
3. **Phase 6**: Prepare for deployment

## Known Issues / Notes

- Next.js warning about workspace root (non-critical, can fix in next.config.ts)
- API token currently hardcoded in .env.local (will be replaced with real auth)
- No pagination yet (will be needed for production)
- No form debouncing on search (minor UX improvement)
