# Ask G — Registry Support Orchestrator

Ask G is a staff-facing support agent that answers registrar/registrant
escalations by retrieving registry state through existing read-only application
services and reasoning over it.

> **MVP scope:** This is an intern-grade, staff-facing assist tool. A human
> support rep is always the safety net. The agent drafts; the human ships.
> Nothing is customer-facing in this iteration.

## Architecture

```
Caller → Orchestrator.Ask(ctx, question, scope)
  → loop:
      → ModelProvider.Generate(ctx, req)  // Anthropic adapter
      ← ToolCalls
      → ToolExecutor.Execute(ctx, toolName, input)
          → switch toolName:
              "get_domain" → domainService.GetDomainByName(...)
              "get_tld"    → tldService.GetTLDByName(...)
          → map entity → output struct (reuses MCP output types)
      → feed results back as normalized ToolResults
  ← Final ModelResponse with structured outcome
  → parse JSON → Result{Outcome, Answer, Evidence, ...}
```

### Key Design Decisions

- **In-process:** Tools call `DomainService`/`TLDService` directly — no MCP
  transport, no HTTP round-trip.
- **Provider-agnostic:** The orchestrator depends on `ModelProvider`, an
  interface. The Anthropic adapter is the reference implementation. New
  providers require zero changes to the loop.
- **Testable:** Both `ModelProvider` and `ToolExecutor` are injectable
  interfaces. The full test suite runs without a live model or database.
- **Three outcomes:** `answer` (grounded), `escalate` (insufficient data),
  `action_required` (mutation detected → routed to human).
- **Bounded:** Hard iteration cap (default 10). On cap, the orchestrator
  returns an escalation with whatever evidence was gathered.

## Package Layout

```
internal/askg/
  provider.go         # ModelProvider interface + normalized types
  toolexec.go         # ToolExecutor interface
  result.go           # Output contract (Result, Evidence, Outcome, CallerScope)
  config.go           # Configuration struct
  prompt.go           # Runtime system prompt (versioned)
  orchestrator.go     # Agent loop
  tools.go            # InProcessToolExecutor + tool definitions
  orchestrator_test.go
  tools_test.go
  testutil_test.go

  provider/
    anthropic/
      anthropic.go      # Anthropic Claude adapter
      anthropic_test.go

cmd/askg/
  main.go             # Standalone CLI entrypoint
```

## Usage

### CLI

```bash
# Set required environment variables
export ANTHROPIC_API_KEY=sk-ant-...
export DB_USER=postgres DB_PASS=... DB_HOST=localhost DB_PORT=5432 DB_NAME=dominos DB_SSLMODE=disable

# Run a query
go run ./cmd/askg "What is the status of example.best?"

# Override model (optional)
LLM_MODEL=claude-sonnet-4-6 go run ./cmd/askg "Is example.best in redemption?"
```

### Programmatic

```go
cfg := askg.Config{
    Model:  "claude-sonnet-4-6",
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
}

executor := askg.NewInProcessToolExecutor(domainService, tldService, nil, logger)
provider := anthropic.NewAdapter(cfg)
orch := askg.NewOrchestrator(provider, executor, cfg, logger)

result, err := orch.Ask(ctx, "What is the status of example.best?",
    askg.CallerScope{UserID: "staff@example.com"})
```

## Configuration

| Env Variable | Default | Description |
|-------------|---------|-------------|
| `ANTHROPIC_API_KEY` | (required) | Anthropic API key |
| `ANTHROPIC_BASE_URL` | (Anthropic default) | Override API base URL |
| `LLM_MODEL` | `claude-sonnet-4-6` | Main model for tool-use |
| `ASKG_CLASSIFIER_MODEL` | `claude-haiku-4-5` | Cheaper model for intent classification (future) |
| `ASKG_USER_ID` | `cli-user` | Staff member identity for audit logging |
| `DB_USER`, `DB_PASS`, etc. | — | Database credentials (same as other servers) |

## Testing

```bash
# Run all tests (no database or API key needed)
go test ./internal/askg/... -v

# Run with coverage
go test ./internal/askg/... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Available Tools

| Tool | Description | Service Method |
|------|-------------|----------------|
| `get_domain` | Look up domain registry state | `DomainService.GetDomainByName` |
| `get_tld` | Look up TLD configuration | `TLDService.GetTLDByName` |

## Output Contract

Results are one of three discriminated outcomes:

```json
// answer — grounded in tool results
{"outcome": "answer", "answer": "...", "evidence": [...], "iterations": 2}

// escalate — insufficient data, out of scope
{"outcome": "escalate", "reason": "...", "evidence": [...], "iterations": 3}

// action_required — mutation detected, routed to human
{"outcome": "action_required", "reason": "...", "action": "...", "evidence": [...]}
```

Every claim in `answer` is supported by entries in `evidence`, which records
the tool name, input, and (scoped) result for each invocation.
