# Agent list_phases Function Fix

## Issue
The `list_phases` agent function was using incorrect parameters and endpoint, causing API errors when trying to list phases for a TLD.

**Error Message:**
```
"Error while attempting to list the phases for the TLD 'alpaca' due to an API error"
```

## Root Cause
The agent function was incorrectly configured:

### Before (WRONG)
- **Parameter**: `tld_id` (numeric TLD ID)
- **Endpoint**: `/phases?limit=50&tld_id={id}`
- **Issue**: This endpoint doesn't exist - phases are always scoped to a TLD

### Backend Reality
- **Endpoint**: `/tlds/{tldName}/phases`
- **Route**: Defined as `/tlds/:tldName/phases` in phase_controller.go
- **TLD Name**: Comes from path parameter, not query string
- **Method**: Uses `ListPhasesByTLD` service method

## Solution

Updated the agent function to match the backend API contract:

### After (CORRECT)
- **Parameter**: `tld_name` (string TLD name without dot)
- **Endpoint**: `/tlds/{tldName}/phases?limit=50`
- **Example**: `/tlds/my.alpaca/phases?limit=50`

## Code Changes

### 1. Function Definition
**Before:**
```go
{
    Name: "list_phases",
    Description: "List all phases in the system or for a specific TLD...",
    Parameters: {
        "tld_id": {  // ❌ WRONG
            Type: jsonschema.String,
            Description: "Filter by TLD ID (optional)",
        },
        "limit": {
            Type: jsonschema.Number,
            Description: "Maximum number of results to return (default: 50)",
        },
    },
    // No required fields - tld_id was optional ❌
}
```

**After:**
```go
{
    Name: "list_phases",
    Description: "List all phases for a specific TLD. Use this to show phase information for a TLD.",
    Parameters: {
        "tld_name": {  // ✅ CORRECT
            Type: jsonschema.String,
            Description: "TLD name without dot (e.g., 'alpaca', 'my.alpaca')",
        },
        "limit": {
            Type: jsonschema.Number,
            Description: "Maximum number of results to return (default: 50)",
        },
    },
    Required: ["tld_name"],  // ✅ Now required
}
```

### 2. Implementation Function
**Before:**
```go
func (f *Functions) listPhases(args map[string]interface{}) (string, error) {
    limit := 50
    if l, ok := args["limit"].(float64); ok {
        limit = int(l)
    }

    path := fmt.Sprintf("/phases?limit=%d", limit)  // ❌ Wrong endpoint
    if tldID, ok := args["tld_id"].(string); ok && tldID != "" {
        path += fmt.Sprintf("&tld_id=%s", tldID)  // ❌ Wrong parameter
    }

    resp, err := f.adminClient.Get(path)
    if err != nil {
        return "", fmt.Errorf("failed to list phases: %w", err)
    }

    return string(resp), nil
}
```

**After:**
```go
func (f *Functions) listPhases(args map[string]interface{}) (string, error) {
    tldName := args["tld_name"].(string)  // ✅ Get TLD name
    
    limit := 50
    if l, ok := args["limit"].(float64); ok {
        limit = int(l)
    }

    path := fmt.Sprintf("/tlds/%s/phases?limit=%d", tldName, limit)  // ✅ Correct endpoint

    resp, err := f.adminClient.Get(path)
    if err != nil {
        return "", fmt.Errorf("failed to list phases for TLD '%s': %w", tldName, err)  // ✅ Better error
    }

    return string(resp), nil
}
```

## Testing

### Test Queries
All of these should now work:

```
"What phases are currently active in TLD alpaca?"
"List phases for my.alpaca"
"Show me all phases for the alpaca TLD"
"What phases does my.alpaca have?"
```

### Expected Behavior
1. Agent receives query about phases
2. Extracts TLD name from user's question (e.g., "alpaca" or "my.alpaca")
3. Calls `list_phases(tld_name="my.alpaca")`
4. Makes API call to `/tlds/my.alpaca/phases?limit=50`
5. Returns phase information including:
   - Phase name
   - Phase type (Launch or GA)
   - Start date
   - End date (if set)
   - Prices and fees
   - Policy information

### Example Conversation
```
User: "What phases are currently active in TLD alpaca?"
Agent: Calls list_phases(tld_name="alpaca")
API: GET /tlds/alpaca/phases?limit=50
Response: Returns array of phases (possibly empty if no phases created yet)
Agent: "The alpaca TLD currently has no phases" OR "The alpaca TLD has the following phases: ..."
```

## Backend Endpoint Details

### Route Definition
From `internal/interface/rest/phase_controller.go`:
```go
phaseGroup := e.Group("/tlds/:tldName/phases", handler)
{
    phaseGroup.GET("", ctrl.ListPhases)  // Lists phases for TLD
    phaseGroup.GET("active", ctrl.ListActivePhasesPerTLD)  // Active phases only
    phaseGroup.GET(":phaseName", ctrl.GetPhase)  // Specific phase
    // ... other routes
}
```

### Controller Implementation
```go
func (ctrl *PhaseController) ListPhases(ctx *gin.Context) {
    // Gets TLD name from path parameter
    tldName := ctx.Param("tldName")
    
    // Calls service with TLD name
    phases, err := ctrl.phaseService.ListPhasesByTLD(ctx, tldName, pageSize, pageCursor)
    
    // Returns phases for this TLD
    ctx.JSON(200, response)
}
```

## Impact

### Before Fix
❌ `list_phases` function completely non-functional
❌ Agent couldn't list phases for any TLD
❌ Errors when asking about phases

### After Fix
✅ `list_phases` function works correctly
✅ Agent can list phases for any TLD
✅ Proper error messages if TLD doesn't exist
✅ Matches backend API contract exactly

## Related Issues Fixed

This fix also clarifies that:
1. **There is no global "list all phases" endpoint** - phases are always scoped to a TLD
2. **TLD names are used, not IDs** - consistent with create_phase and end_phase functions
3. **tld_name is required** - can't list phases without specifying which TLD

## Files Modified

1. **internal/agent/functions/functions.go**
   - Updated `list_phases` function definition
   - Fixed `listPhases` implementation
   - Changed from `tld_id` to `tld_name`
   - Changed endpoint from `/phases` to `/tlds/{tldName}/phases`

## Date
October 11, 2025

## See Also
- AGENT_PHASE_TYPE_FIX.md - Phase type enum fix (Launch vs GA)
- AGENT_END_PHASE_FUNCTION.md - end_phase function addition
- AGENT_TESTING_GUIDE.md - Testing guide for all functions
