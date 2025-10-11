# Agent Function: end_phase

## Summary
Added `end_phase` function to Alpaca Agent, bringing the total number of agent functions to **9**. This function allows users to set or update the end date for existing phases through natural language conversation.

## Function Details

### Purpose
Set or update the end date for a phase, matching the functionality available in the UI's phase drawer "End Phase" dialog.

### Parameters
- **tld_name** (required): TLD name without dot (e.g., "my.alpaca")
- **phase_name** (required): Phase name (e.g., "Sunrise", "General Availability")
- **ends** (required): End date in ISO 8601 format (e.g., "2026-03-01T00:00:00Z")

### Backend Endpoint
- **Method**: PUT
- **Path**: `/tlds/{tldName}/phases/{phaseName}/end`
- **Body**: `{ "ends": "2026-03-01T00:00:00Z" }`
- **Command**: `EndPhaseCommand` from `internal/application/commands/phase_commands.go`

### Validation Rules (Backend)
- End date must be in the future
- End date must be after the phase start date
- Only applies to existing phases

## Implementation

### Agent Function Definition
```go
{
    Type: openai.ToolTypeFunction,
    Function: &openai.FunctionDefinition{
        Name:        "end_phase",
        Description: "Set or update the end date for a phase. Use this to close a phase or set when it should end.",
        Parameters: jsonschema.Definition{
            Type: jsonschema.Object,
            Properties: map[string]jsonschema.Definition{
                "tld_name": {
                    Type:        jsonschema.String,
                    Description: "TLD name without dot (e.g., 'alpaca', 'my.alpaca')",
                },
                "phase_name": {
                    Type:        jsonschema.String,
                    Description: "Phase name (e.g., 'Sunrise', 'General Availability')",
                },
                "ends": {
                    Type:        jsonschema.String,
                    Description: "End date in ISO 8601 format (e.g., '2026-03-01T00:00:00Z'). Must be after the phase start date and in the future.",
                },
            },
            Required: []string{"tld_name", "phase_name", "ends"},
        },
    },
}
```

### Implementation Function
```go
func (f *Functions) endPhase(args map[string]interface{}) (string, error) {
    tldName := args["tld_name"].(string)
    phaseName := args["phase_name"].(string)

    body := map[string]interface{}{
        "ends": args["ends"],
    }

    path := fmt.Sprintf("/tlds/%s/phases/%s/end", tldName, phaseName)
    resp, err := f.adminClient.Put(path, body)
    if err != nil {
        return "", fmt.Errorf("failed to set end date for phase: %w", err)
    }

    return fmt.Sprintf("Successfully set end date for phase '%s' on TLD '%s'. Response: %s", phaseName, tldName, string(resp)), nil
}
```

## Use Cases

### 1. Close an Open-Ended Phase
**User**: "Set an end date for the Sunrise phase on my.alpaca to March 1st 2026"

**Agent Action**:
```
end_phase(
  tld_name="my.alpaca",
  phase_name="Sunrise",
  ends="2026-03-01T00:00:00Z"
)
```

### 2. Extend a Phase
**User**: "The Sunrise phase should end on March 31st instead of March 1st"

**Agent Action**: Updates the end date to the new value

### 3. Shorten a Phase
**User**: "Actually, let's end Sunrise on February 15th 2026"

**Agent Action**: Updates the end date to the earlier date (if valid)

### 4. Natural Language Variations
All of these work:
- "Set the end date for Sunrise to March 1st"
- "End the Sunrise phase on my.alpaca on March 1st 2026"
- "The Sunrise phase should finish on March 1st"
- "Close Sunrise phase March 1st 2026"

## Testing Examples

### Basic Test
```
User: "Set the end date for the Sunrise phase on my.alpaca to March 1st 2026"
Expected: Phase end date updated successfully
```

### Multi-Step Conversation
```
User: "I want to end a phase"
Agent: "Which TLD and phase would you like to set an end date for?"
User: "The Sunrise phase on my.alpaca"
Agent: "When should the Sunrise phase end?"
User: "March 1st 2026"
Agent: Calls end_phase and confirms success
```

### Error Cases
```
User: "End the Sunrise phase yesterday"
Expected: Backend validation error (end date must be in the future)

User: "End Sunrise on my.alpaca on December 15th 2025"
Expected: Backend validation error (end date must be after start date)
```

## UI Alignment

This function replicates the "Set End Date" functionality from the Phase Detail Drawer:

**UI Features**:
- Calendar picker to select end date
- Validation: must be after start date and in the future
- Shows phase name in dialog
- Displays current start date for reference

**Agent Features**:
- Natural language date parsing
- Backend validation (same rules)
- Contextual understanding (can infer TLD/phase from conversation)
- Immediate confirmation

## Files Modified

1. **internal/agent/functions/functions.go**
   - Added `end_phase` function definition
   - Added `endPhase` implementation
   - Added case to switch statement

2. **AGENT_TESTING_GUIDE.md**
   - Updated function count to 9
   - Added `end_phase` test section
   - Updated test flows

## Related Backend Components

- **Controller**: `internal/interface/rest/phase_controller.go::EndPhase`
- **Service**: `internal/application/services/phase_service.go::EndPhase`
- **Command**: `internal/application/commands/phase_commands.go::EndPhaseCommand`
- **Entity**: `internal/domain/entities/tld.go::EndPhase`
- **Route**: `PUT /tlds/:tldName/phases/:phaseName/end`

## Complete Agent Function List (9 total)

1. ✅ **create_registry_operator** - Create RO
2. ✅ **list_registry_operators** - List all ROs
3. ✅ **create_tld** - Create TLD
4. ✅ **list_tlds** - List all TLDs
5. ✅ **get_tld_info** - Get TLD details
6. ✅ **create_phase** - Create phase
7. 🆕 **end_phase** - Set phase end date
8. ✅ **list_phases** - List phases
9. ✅ **search_domains** - Search domains

## Next Steps

Ready to test:
```
1. Create a phase without an end date:
   "Create a Sunrise phase for my.alpaca starting January 1st 2026"

2. Set the end date:
   "Set the end date for Sunrise to March 1st 2026"

3. Verify:
   "What phases does my.alpaca have?"
   (Should show Sunrise with end date)
```

## Date
October 11, 2025
