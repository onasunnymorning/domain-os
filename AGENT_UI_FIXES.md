# Agent UI and Function Fixes

## Issues Fixed

### 1. ✅ Chat Drawer Scroll Issue
**File:** `frontend/components/agent/agent-chat.tsx`

**Problem:** Messages wouldn't scroll properly, content was getting cut off

**Solution:** 
- Added `min-h-0` to ScrollArea to allow flex shrinking
- Added `shrink-0` to header and input sections to prevent them from shrinking
- Moved padding from ScrollArea to the inner messages div

**Changes:**
```tsx
// Before
<ScrollArea className="flex-1 p-4" ref={scrollAreaRef}>
  <div className="space-y-4">

// After  
<ScrollArea className="flex-1 min-h-0" ref={scrollAreaRef}>
  <div className="space-y-4 p-4">
```

### 2. ✅ Double X Close Button
**File:** `frontend/components/agent/agent-chat.tsx`

**Problem:** There were two X buttons - one in the AgentChat header and one in the SheetContent (from Radix UI)

**Solution:** 
- Removed the X button and its close logic from AgentChat header
- Kept the SheetContent's built-in close button
- Removed unused X icon import
- Simplified header structure

**Changes:**
```tsx
// Before
<div className="flex items-center justify-between p-4 border-b">
  <div className="flex items-center gap-2">
    {/* header content */}
  </div>
  {onClose && (
    <Button variant="ghost" size="icon" onClick={onClose}>
      <X className="w-4 h-4" />
    </Button>
  )}
</div>

// After
<div className="flex items-center gap-2 p-4 border-b shrink-0">
  {/* header content - no close button */}
</div>
```

### 3. ✅ Registry Operator RyID Confusion
**Files:** 
- `internal/agent/functions/functions.go`
- `cmd/api/ry-admin/ryAdminAPI.go`

**Problem:** The create_registry_operator function was missing the required `ryID` parameter, causing the agent to repeatedly ask for it and fail

**Root Cause:** The function definition didn't include ryID as a parameter, but the backend API requires it (it's a required field in the RegistryOperator entity)

**Solution:**

1. **Added ryID to function definition:**
```go
Properties: map[string]jsonschema.Definition{
  "ryID": {
    Type:        jsonschema.String,
    Description: "Registry operator ID/handle (e.g., 'AlpacaNames'). This should be a unique identifier without spaces.",
  },
  "name": {
    Type:        jsonschema.String,
    Description: "Registry operator name (e.g., 'Acme Registry')",
  },
  "email": {
    Type:        jsonschema.String,
    Description: "Contact email address",
  },
  "url": {
    Type:        jsonschema.String,
    Description: "Website URL (optional)",
  },
},
Required: []string{"ryID", "name", "email"},
```

2. **Updated function implementation to include ryID in request:**
```go
func (f *Functions) createRegistryOperator(args map[string]interface{}) (string, error) {
  body := map[string]interface{}{
    "ryID":  args["ryID"],  // Added
    "name":  args["name"],
    "email": args["email"],
  }
  // ... rest of implementation
}
```

3. **Updated success message to show ryID:**
```go
return fmt.Sprintf("Successfully created registry operator '%s' (RyID: %s). Response: %s", 
  args["name"], args["ryID"], string(resp)), nil
```

### 4. ✅ Backend API URL Fix (Previously)
**File:** `cmd/api/ry-admin/ryAdminAPI.go`

**Problem:** Agent was trying to call external API URL (`api.dos.dev.geoff.it`) instead of localhost

**Solution:** Changed agent service initialization to use localhost:
```go
// Before
adminAPIURL := fmt.Sprintf("http://%s:%s", os.Getenv("API_HOST"), os.Getenv("API_PORT"))

// After (agent runs in same container as API)
adminAPIURL := fmt.Sprintf("http://localhost:%s", os.Getenv("API_PORT"))
```

## Testing

After these fixes:

### Test Scroll:
1. Open agent (⌘K)
2. Send multiple messages to create a long conversation
3. Verify messages scroll smoothly
4. Verify header and input stay fixed

### Test Close Button:
1. Open agent (⌘K)
2. Verify only ONE X button appears (top right of sheet)
3. Click X to close - should work
4. Press ⌘K to reopen

### Test Registry Operator Creation:
1. Open agent (⌘K)
2. Say: "Can you create a registry operator"
3. Agent should ask for: Name, RyID, and Email
4. Provide all three (e.g., "Alpaca Names", "AlpacaNames", "me@alpaca.com")
5. Verify successful creation without repeated RyID requests

Example conversation:
```
User: Can you create a registry operator
Agent: Sure, I can help with that. Could you provide me with the following details...
      - The name of the registry operator (e.g., 'Acme Registry')
      - The RyID (unique identifier/handle, e.g., 'AlpacaNames')
      - The contact email address
      - The website URL (optional)

User: Alpaca Names is the name, use me@alpaca.com as email and if you need to supply a RyID use AlpacaNames
Agent: Successfully created registry operator 'Alpaca Names' (RyID: AlpacaNames)...
```

## Files Modified

1. `frontend/components/agent/agent-chat.tsx`
   - Fixed scroll behavior
   - Removed duplicate close button
   - Removed unused X icon import

2. `internal/agent/functions/functions.go`
   - Added ryID parameter to create_registry_operator function definition
   - Updated implementation to include ryID in request body
   - Improved success message

3. `cmd/api/ry-admin/ryAdminAPI.go`
   - Changed agent API URL to use localhost instead of API_HOST env var

## Deployment

Changes require backend rebuild:
```bash
make dev-build
```

Frontend changes are picked up automatically by Next.js dev server (hot reload).
