# Alpaca Agent - Function Testing Guide

## Available Functions (9 total)

### ✅ Already Tested
1. **create_registry_operator** - Creates a new RO
2. **create_tld** - Creates a new TLD under an RO
3. **create_phase** - Creates a new phase for a TLD

### 🧪 Ready to Test

### 4. **list_registry_operators**
Lists all registry operators in the system.

**Test queries:**
- "List all registry operators"
- "Show me the ROs"
- "What registry operators do we have?"

**Expected:** Returns a list of ROs including "Alpaca Names" that you just created

---

### 5. **list_tlds**
Lists all TLDs in the system.

**Test queries:**
- "List all TLDs"
- "Show me all top-level domains"
- "What TLDs exist?"

**Expected:** Returns a list including ".alpaca" TLD you just created

---

### 6. **get_tld_info**
Get detailed information about a specific TLD.

**Test queries:**
- "Tell me about the alpaca TLD"
- "Get info on .alpaca"
- "What's the status of the alpaca domain?"

**Expected:** Returns detailed info about .alpaca including:
- RyID (AlpacaNames)
- Type (generic/geographic/sponsored - auto-determined)
- DNS settings
- Phases (if any)

---

### 7. **create_phase** ✅ TESTED
Create a new phase for a TLD (Sunrise, Landrush, GA, etc.)

**Test queries:**
- "Create a Sunrise phase for my.alpaca TLD starting January 1st 2026"
- "Add a General Availability phase to my.alpaca"
- "Set up a Landrush phase for my.alpaca"

**Parameters needed:**
- name (e.g., "Sunrise", "General Availability")
- tld_name (TLD name without dot, e.g., "my.alpaca")
- type ("Launch" for all pre-GA phases, "GA" for General Availability)
- starts (ISO 8601 format, e.g., "2026-01-01T00:00:00Z")
- ends (optional - omit for open-ended phases)

**Phase Types:**
- `"Launch"` - For Sunrise, Landrush, EAP, and other launch phases
- `"GA"` - For General Availability only

**Note:** The phase **name** is descriptive (e.g., "Sunrise"), while the **type** is technical classification ("Launch" or "GA")

---

### 8. **end_phase** 🆕
Set or update the end date for an existing phase.

**Test queries:**
- "Set the end date for the Sunrise phase on my.alpaca to March 1st 2026"
- "End the Sunrise phase on my.alpaca on February 15th 2026"
- "The Sunrise phase should end on March 31st 2026"

**Parameters needed:**
- tld_name (TLD name without dot, e.g., "my.alpaca")
- phase_name (Phase name, e.g., "Sunrise")
- ends (ISO 8601 format, must be after phase start date and in the future)

**Example:**
```
User: "Set an end date for the Sunrise phase on my.alpaca to March 1st 2026"
Agent calls: end_phase(tld_name="my.alpaca", phase_name="Sunrise", ends="2026-03-01T00:00:00Z")
```

**Use Cases:**
- Close an open-ended phase
- Extend or shorten a phase
- Set a specific end date for planning

---

### 9. **list_phases**
List all phases for a specific TLD.

**Test queries:**
- "What phases are currently active in TLD alpaca?"
- "Show me phases for my.alpaca TLD"
- "What phases does my.alpaca have?"
- "List the phases for alpaca"

**Parameters needed:**
- tld_name (TLD name without dot, e.g., "my.alpaca") - REQUIRED

**Expected:** Returns phase information including:
- Phase name
- Type (Launch or GA)
- Start/end dates
- Prices and fees
- Policy information

**Note:** Phases are always scoped to a TLD - you must specify which TLD to list phases for.

---

### 10. **search_domains**
Search for domains by name pattern.

**Test queries:**
- "Search for domains with 'test' in the name"
- "Find domains like example.alpaca"
- "Are there any domains registered under .alpaca?"

**Expected:** Returns matching domains (may be empty if no domains registered yet)

---

## Recommended Test Flow

### Flow 1: List & Info Operations
1. **List ROs**: "List all registry operators"
2. **List TLDs**: "Show me all TLDs"
3. **Get TLD Info**: "Tell me about the alpaca TLD"

### Flow 2: Phase Management
1. **Create Phase**: "Create a Sunrise phase for my.alpaca TLD starting on January 1st 2026"
2. **List Phases**: "What phases does my.alpaca have?"
3. **End Phase**: "Set the end date for Sunrise phase to March 1st 2026"

### Flow 3: Domain Search
1. **Search Domains**: "Search for domains under .alpaca"

---

## Complex Conversational Tests

### Test Natural Language Understanding:
- "I want to see all the registry operators we have"
- "Can you show me what TLDs are available?"
- "What do you know about the .alpaca domain?"
- "Let's add a sunrise phase to alpaca starting next month"

### Test Error Handling:
- "Get info on a TLD that doesn't exist" (e.g., "Tell me about .nonexistent")
- "Search for a domain with invalid characters"

### Test Multi-Step Operations:
- "I want to create a new phase for alpaca" → Agent should ask for details
- "Add a General Availability phase" → Should ask which TLD

---

## What's Currently NOT Supported

The following are NOT implemented yet (Phase 2/3):
- ❌ Creating domains
- ❌ Managing registrars
- ❌ Setting prices/fees via agent
- ❌ Managing premium lists
- ❌ Updating RO/TLD information
- ❌ Deleting resources

---

## Testing Tips

1. **Start Simple**: Test basic list operations first
2. **Check Responses**: Verify data matches what you created
3. **Test Edge Cases**: Try non-existent resources, invalid inputs
4. **Natural Language**: Mix formal and casual phrasing
5. **Multi-Turn**: Have conversations, don't just send single commands

---

## Quick Test Script

Here's a suggested test sequence:

```
1. "List all registry operators"
2. "Show me all TLDs"  
3. "Tell me about the alpaca TLD"
4. "Create a Sunrise phase for alpaca starting January 1st 2025"
5. "What phases does alpaca have?"
6. "Search for domains under alpaca"
```

---

## Known Limitations

- **Phase Creation**: Requires TLD ID (numeric), agent needs to look it up first
- **Date Formats**: Must be ISO 8601 (e.g., "2025-01-15T00:00:00Z")
- **Search**: Basic pattern matching, not full-text search
- **No Updates**: Can only create, not modify existing resources

Enjoy testing! 🦙
