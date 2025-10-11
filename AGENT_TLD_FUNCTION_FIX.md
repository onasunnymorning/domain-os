# Agent TLD Creation Function Fix

## Issue Fixed

### ❌ Problem: TLD Type Parameter Confusion

The `create_tld` agent function was asking users to specify the TLD `type` (generic/geographic/sponsored), but the backend automatically determines this based on the TLD name.

**User Experience:**
```
User: Create a TLD "alpaca" under registry operator "AlpacaNames"
Agent: Now, could you specify the TLD Type you'd like to set for "alpaca"? 
       It can be either "generic", "geographic", or "sponsored".
User: 😕 (unnecessary friction)
```

### ✅ Solution: Remove Type Parameter

The backend's `NewTLD` function automatically calls `setTLDType()` to determine the type based on the TLD name, so the agent doesn't need to ask for it.

## Changes Made

### 1. Updated Function Definition
**File:** `internal/agent/functions/functions.go`

**Before:**
```go
Name:        "create_tld",
Description: "Create a new top-level domain. Use this when the user wants to set up a new TLD like .shop or .brand.",
Parameters: jsonschema.Definition{
    Type: jsonschema.Object,
    Properties: map[string]jsonschema.Definition{
        "tld": {
            Type:        jsonschema.String,
            Description: "TLD name without dot (e.g., 'shop', 'brand')",
        },
        "registry_operator_id": {
            Type:        jsonschema.String,
            Description: "Registry operator ID (snowflake ID)",
        },
        "type": {  // ❌ REMOVED
            Type:        jsonschema.String,
            Description: "TLD type: 'generic', 'geographic', or 'sponsored'",
            Enum:        []string{"generic", "geographic", "sponsored"},
        },
    },
    Required: []string{"tld", "registry_operator_id", "type"},  // ❌ type removed from required
},
```

**After:**
```go
Name:        "create_tld",
Description: "Create a new top-level domain under a registry operator. Use this when the user wants to set up a new TLD like .shop or .brand. The TLD type (generic/geographic/sponsored) is automatically determined by the backend.",
Parameters: jsonschema.Definition{
    Type: jsonschema.Object,
    Properties: map[string]jsonschema.Definition{
        "tld": {
            Type:        jsonschema.String,
            Description: "TLD name without dot (e.g., 'shop', 'brand', 'alpaca')",
        },
        "registry_operator_id": {
            Type:        jsonschema.String,
            Description: "Registry operator RyID (e.g., 'AlpacaNames')",
        },
    },
    Required: []string{"tld", "registry_operator_id"},  // ✅ Only 2 parameters
},
```

### 2. Updated Implementation to Match API Contract
**File:** `internal/agent/functions/functions.go`

The backend API expects `Name` and `RyID` (uppercase), not `tld` and `registry_operator_id`.

**Before:**
```go
func (f *Functions) createTLD(args map[string]interface{}) (string, error) {
    body := map[string]interface{}{
        "tld":                  args["tld"],
        "registry_operator_id": args["registry_operator_id"],
        "type":                 args["type"],  // ❌ REMOVED
    }
    
    resp, err := f.adminClient.Post("/tlds", body)
    if err != nil {
        return "", fmt.Errorf("failed to create TLD: %w", err)
    }
    
    return fmt.Sprintf("Successfully created TLD .%s. Response: %s", args["tld"], string(resp)), nil
}
```

**After:**
```go
func (f *Functions) createTLD(args map[string]interface{}) (string, error) {
    body := map[string]interface{}{
        "Name": args["tld"],                        // ✅ Matches API: Name
        "RyID": args["registry_operator_id"],      // ✅ Matches API: RyID
    }
    
    resp, err := f.adminClient.Post("/tlds", body)
    if err != nil {
        return "", fmt.Errorf("failed to create TLD: %w", err)
    }
    
    return fmt.Sprintf("Successfully created TLD .%s under registry operator %s. Response: %s", 
        args["tld"], args["registry_operator_id"], string(resp)), nil
}
```

## Backend API Reference

The backend's `CreateTLDRequest` struct only requires two fields:

```go
// internal/interface/rest/request/create_tld_request.go
type CreateTLDRequest struct {
    Name string `json:"Name" binding:"required"`
    RyID string `json:"RyID" binding:"required"`
}
```

This maps to the domain entity:

```go
// internal/domain/entities/tld.go
func NewTLD(name, RyID string) (*TLD, error) {
    d, err := NewDomainName(name)
    if err != nil {
        return nil, err
    }
    validatedRyID, err := NewClIDType(RyID)
    if err != nil {
        return nil, err
    }
    tld := &TLD{Name: *d}
    tld.RyID = validatedRyID
    tld.SetUname()
    tld.setTLDType()  // ✅ Type is set automatically here
    tld.CreatedAt = RoundTime(time.Now().UTC())
    return tld, nil
}
```

## Testing

### ✅ Expected User Experience

**Simple, streamlined conversation:**
```
User: Create a TLD "alpaca" under registry operator "AlpacaNames"
Agent: Successfully created TLD .alpaca under registry operator AlpacaNames
```

**Or step-by-step:**
```
User: Can you create a TLD?
Agent: Sure! I'll need:
      - TLD name (without the dot)
      - Registry operator RyID
      
User: TLD is "alpaca" and the operator is "AlpacaNames"
Agent: Successfully created TLD .alpaca under registry operator AlpacaNames
```

### Test Cases

1. **Create TLD with all info in one message:**
   ```
   User: Create TLD "alpaca" for registry operator "AlpacaNames"
   ```

2. **Create TLD interactively:**
   ```
   User: Create a new TLD
   Agent: [asks for tld and registry_operator_id]
   User: [provides info]
   ```

3. **Verify no type parameter is requested:**
   - Agent should NEVER ask "what type of TLD?"
   - Agent should NEVER mention generic/geographic/sponsored

## Deployment

Backend rebuild required:
```bash
make dev-build
```

## Related Documentation

- Backend entity: `internal/domain/entities/tld.go`
- API request: `internal/interface/rest/request/create_tld_request.go`
- API controller: `internal/interface/rest/tld_controller.go`
- Previous fixes: `AGENT_UI_FIXES.md`
