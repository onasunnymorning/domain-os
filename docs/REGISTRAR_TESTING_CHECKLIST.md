# Registrar Management - Testing Checklist

## 🚀 Pre-Testing Setup

### 1. Backend Setup

```bash
# Navigate to project root
cd /Users/gprins/Code/Geoff/domain-os

# Start the backend
make run

# Verify backend is running
curl http://localhost:8000/health
```

**Expected:** Backend starts successfully on port 8000

### 2. Frontend Setup

```bash
# Navigate to frontend
cd frontend

# Install dependencies (if needed)
npm install

# Start development server
npm run dev
```

**Expected:** Frontend starts on http://localhost:3000

### 3. Environment Variables

```bash
# Check .env.local exists
cat frontend/.env.local
```

**Expected:**
```
NEXT_PUBLIC_API_BASE_URL=http://localhost:8000
```

---

## 📋 IANA Registrars Tab Testing

### Initial Load

- [ ] Navigate to http://localhost:3000/registrars
- [ ] IANA Registrars tab is selected by default
- [ ] Table loads and displays registrars
- [ ] Total count displays (should match backend count)
- [ ] Last sync timestamp appears
- [ ] No console errors

**Expected Result:** Table shows IANA registrars with GurID, Name, Status, RDAP URL

### Search Functionality

- [ ] Type in search box: "google"
- [ ] Table updates automatically
- [ ] Results show registrars containing "google" in name
- [ ] Clear search box
- [ ] Table shows all registrars again

**Expected Result:** Real-time filtering works, case-insensitive

### Status Filtering

- [ ] Click status dropdown
- [ ] Select "Accredited"
- [ ] Table shows only Accredited registrars
- [ ] Status badges show "Accredited" in default color
- [ ] Select "Terminated"
- [ ] Table shows only Terminated registrars
- [ ] Status badges show "Terminated" in red
- [ ] Select "All Statuses"
- [ ] Table shows all registrars

**Expected Result:** Filtering works, badge colors match status

### Combined Filtering

- [ ] Enter search term: "network"
- [ ] Select status: "Accredited"
- [ ] Table shows only Accredited registrars with "network" in name
- [ ] Count updates accordingly

**Expected Result:** Multiple filters work together

### Sync Functionality

- [ ] Click "Sync from IANA" button
- [ ] Button shows "Syncing..." with spinner
- [ ] Button is disabled during sync
- [ ] Toast notification appears: "IANA registrar data has been updated"
- [ ] Table refreshes automatically
- [ ] Last sync timestamp updates

**Expected Result:** Sync completes successfully, data updates

**Note:** Sync may take 10-30 seconds (downloads from IANA)

### RDAP URLs

- [ ] Find a registrar with RDAP URL
- [ ] Click the RDAP URL
- [ ] Opens in new tab
- [ ] Shows RDAP data (if accessible)

**Expected Result:** Links work, open in new tab

### Status Badge Colors

- [ ] Accredited badge: Default blue
- [ ] Terminated badge: Red/destructive
- [ ] Reserved badge: Secondary/gray
- [ ] Unknown badge: Outline

**Expected Result:** Colors match status semantics

### Loading States

- [ ] Refresh page
- [ ] Loading spinner appears
- [ ] "Loading registrars..." message shows
- [ ] Table appears when loaded

**Expected Result:** Smooth loading experience

### Error Handling

**Test 1: Backend Down**
- [ ] Stop backend server
- [ ] Refresh page
- [ ] Error message appears: "Error loading registrars: ..."
- [ ] No crash, graceful error display

**Test 2: Network Error During Sync**
- [ ] Stop backend
- [ ] Click "Sync from IANA"
- [ ] Error toast appears
- [ ] Button re-enables

**Expected Result:** Errors handled gracefully, no crashes

### Empty States

- [ ] Enter search: "zzzzzzzzz" (no matches)
- [ ] Message appears: "No registrars found matching your criteria"
- [ ] No broken UI

**Expected Result:** Clean empty state

---

## 📋 System Registrars Tab Testing

### Initial Load

- [ ] Click "System Registrars" tab
- [ ] Tab switches successfully
- [ ] Table loads and displays system registrars
- [ ] Total count displays
- [ ] Columns show: ClID, Name, IANA ID, Status, Auto-renew
- [ ] No console errors

**Expected Result:** Table shows system registrars

### Status Badges

- [ ] OK status: Default blue badge
- [ ] Terminated status: Red badge
- [ ] Readonly status: Secondary/gray badge

**Expected Result:** Badge colors appropriate

### Auto-renew Badges

- [ ] Enabled: Default blue badge showing "Enabled"
- [ ] Disabled: Outline badge showing "Disabled"

**Expected Result:** Auto-renew status clear

### Search (Disabled)

- [ ] Search input is disabled/grayed out
- [ ] Can't type in search box

**Expected Result:** Future feature placeholder

---

## 📋 Tab Navigation

### Switching Tabs

- [ ] Click "IANA Registrars" tab
- [ ] IANA content shows
- [ ] Click "System Registrars" tab
- [ ] System content shows
- [ ] Click back to "IANA Registrars"
- [ ] Previous search/filter state preserved

**Expected Result:** Smooth tab switching, state preserved

---

## 📋 API Integration Tests

### Backend Endpoints

```bash
# Test IANA endpoints
curl http://localhost:8000/ianaregistrars | jq
curl http://localhost:8000/ianaregistrars/count | jq
curl http://localhost:8000/ianaregistrars/1 | jq

# Test with filters
curl "http://localhost:8000/ianaregistrars?status=Accredited&pagesize=10" | jq
curl "http://localhost:8000/ianaregistrars?name_like=google" | jq

# Test sync
curl -X PUT http://localhost:8000/sync/iana-registrars | jq

# Test System endpoints
curl http://localhost:8000/registrars | jq
curl http://localhost:8000/registrars/count | jq
```

**Expected Result:** All endpoints return valid JSON responses

---

## 📋 Browser Testing

### Chrome/Edge

- [ ] All features work
- [ ] No console errors
- [ ] No layout issues

### Firefox

- [ ] All features work
- [ ] No console errors
- [ ] No layout issues

### Safari (if available)

- [ ] All features work
- [ ] No console errors
- [ ] No layout issues

---

## 📋 Responsive Design

### Desktop (1920x1080)

- [ ] Layout looks good
- [ ] Table fits properly
- [ ] No horizontal scroll

### Laptop (1440x900)

- [ ] Layout adapts
- [ ] All elements visible

### Tablet (768x1024)

- [ ] Layout responsive
- [ ] Table scrolls horizontally if needed

### Mobile (375x667)

- [ ] Layout adapts
- [ ] Tabs work
- [ ] Table scrolls

---

## 📋 Performance Testing

### Initial Load Time

- [ ] Page loads in < 2 seconds (with backend running)
- [ ] Data appears quickly
- [ ] No lag in UI

### Search Performance

- [ ] Search updates immediately
- [ ] No noticeable delay
- [ ] Table re-renders smoothly

### Filter Performance

- [ ] Filter updates immediately
- [ ] No lag when changing filters

### Sync Performance

- [ ] Sync completes in reasonable time (10-30s)
- [ ] UI remains responsive during sync
- [ ] Progress indication clear

---

## 📋 React Query Testing

### Cache Behavior

1. Load IANA registrars
2. Switch to System tab
3. Switch back to IANA tab
   - [ ] Data loads instantly from cache
   - [ ] No loading spinner

4. Wait 5+ minutes
5. Switch tabs
   - [ ] Data refetches in background
   - [ ] Shows stale data first

### Mutation Invalidation

1. Click sync
2. Wait for completion
   - [ ] Count updates if changed
   - [ ] Table data refreshes
   - [ ] Cache invalidated

---

## 📋 Accessibility Testing

### Keyboard Navigation

- [ ] Tab key moves between elements
- [ ] Enter activates buttons
- [ ] Escape closes dropdowns
- [ ] All interactive elements focusable

### Screen Reader (if available)

- [ ] Table headers announced
- [ ] Status badges have meaningful text
- [ ] Buttons have clear labels

---

## 📋 Error Scenarios

### Scenario 1: Empty Database

```bash
# Clear IANA registrars from DB
# Restart backend
```

- [ ] Table shows empty state
- [ ] Count shows 0
- [ ] Sync still works

### Scenario 2: Network Timeout

- [ ] Slow network simulation
- [ ] Loading state persists
- [ ] Eventually shows data or error

### Scenario 3: Invalid Filters

- [ ] Apply filter with no matches
- [ ] Empty state shows
- [ ] Can clear filter

---

## 📋 TypeScript & Code Quality

### Type Checking

```bash
cd frontend
npx tsc --noEmit
```

- [ ] No type errors in registrar files
- [ ] Only test file errors (expected)

### Linting

```bash
npm run lint
```

- [ ] No linting errors
- [ ] Code follows conventions

---

## 📋 Documentation Review

- [ ] README files clear
- [ ] API documentation accurate
- [ ] Quick reference useful
- [ ] Architecture diagrams correct

---

## 🐛 Bug Tracking

### Issues Found

| Issue | Severity | Status | Notes |
|-------|----------|--------|-------|
|       |          |        |       |

### Test Results Summary

- **Tests Passed:** __ / __
- **Tests Failed:** __
- **Bugs Found:** __
- **Critical Issues:** __

---

## ✅ Sign-Off

- [ ] All critical features tested
- [ ] No blocking bugs
- [ ] Documentation complete
- [ ] Ready for user testing
- [ ] Ready for code review

**Tested By:** _________________

**Date:** _________________

**Notes:**
_________________________________________
_________________________________________
_________________________________________
