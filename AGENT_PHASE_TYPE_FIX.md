# Agent Phase Type Fix

## Issue
The `create_phase` agent function was using incorrect phase type enum values that didn't match the backend's actual supported types.

## Root Cause
Domain-OS backend (`internal/domain/entities/phase.go`) only supports **TWO** phase types:
- `"GA"` (General Availability)
- `"Launch"` (for all launch phases: Sunrise, Landrush, EAP, etc.)

However, the agent function was incorrectly defined with:
```go
Enum: []string{"sunrise", "landrush", "GA", "eap", "custom"}
```

## Backend Architecture
In Domain-OS, phases work as follows:
- **Phase Name**: Descriptive name (e.g., "Sunrise", "Landrush", "General Availability")
- **Phase Type**: Technical classification - either `"Launch"` or `"GA"`
  - `"Launch"` covers all pre-GA launch phases (Sunrise, Landrush, EAP, Custom, etc.)
  - `"GA"` is for General Availability phases only

This is defined in `internal/domain/entities/phase.go`:
```go
const (
	PhaseTypeGA     PhaseType = "GA"
	PhaseTypeLaunch PhaseType = "Launch"
)
```

## Solution
Updated the agent function definition in `internal/agent/functions/functions.go`:

**Before:**
```go
"type": {
    Type:        jsonschema.String,
    Description: "Phase type: 'sunrise', 'landrush', 'GA', 'eap', 'custom'",
    Enum:        []string{"sunrise", "landrush", "GA", "eap", "custom"},
}
```

**After:**
```go
"type": {
    Type:        jsonschema.String,
    Description: "Phase type: 'Launch' for launch phases (Sunrise, Landrush, EAP, etc.) or 'GA' for General Availability",
    Enum:        []string{"Launch", "GA"},
}
```

Also updated the name description to clarify the distinction:
```go
"name": {
    Type:        jsonschema.String,
    Description: "Phase name (e.g., 'Sunrise', 'Landrush', 'General Availability'). This is a descriptive name for the phase.",
}
```

## Testing
To create a Sunrise phase for a TLD:
```
Create a Sunrise phase for my.alpaca starting January 1st 2026, no end date
```

The agent will now correctly call the API with:
- **name**: "Sunrise" (descriptive name)
- **type**: "Launch" (technical type)
- **starts**: "2026-01-01T00:00:00Z"
- **ends**: (omitted for open-ended phase)

## Impact
- ✅ Phase creation now works correctly
- ✅ Agent uses proper phase type taxonomy
- ✅ Aligns with backend validation rules
- ✅ Clear distinction between phase name (descriptive) and type (technical classification)

## Related Files
- `internal/agent/functions/functions.go` - Agent function definitions (fixed)
- `internal/domain/entities/phase.go` - Backend phase type constants (source of truth)
- `internal/application/commands/phase_commands.go` - CreatePhaseCommand structure
- `internal/interface/rest/phase_controller.go` - Phase REST endpoints

## Date
January 11, 2026 (note: current system date is October 11, 2025, but user request referenced 2026)
