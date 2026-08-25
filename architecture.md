# Architecture

High-level shape of `domain-os`: what the layers are, where they live, and how requests flow through them.

**This document describes structure. It does not define rules.** The architectural rules — with `file:line` evidence, a class saying how well each actually holds, and every known violation enumerated — live in [`docs/INVARIANTS.md`](docs/INVARIANTS.md) and are cited by ID (`INV-01`…`INV-14`). Where the two disagree, `docs/INVARIANTS.md` is authoritative, because it carries the evidence.

That split is deliberate. An architecture document that also asserts rules tends to describe the system as intended rather than as built, and the gap is invisible until someone relies on it.

---

## Architectural style

Hexagonal (ports and adapters), organised around Domain-Driven Design patterns. The domain defines the ports; outer layers implement them.

Four things hold this together, each backed by an invariant:

- **The domain is the core, and depends outward on nothing.** No `internal/**` import, no persistence or transport machinery. Pure value-object libraries (money, UUID, IDNA, DNS name handling) are permitted and used. — `INV-14`
- **Layers meet at interfaces.** The domain declares what it needs from persistence; the infrastructure layer supplies it.
- **Long-running work runs on Temporal.** Domain lifecycle, escrow import, zone/serial drift, FX updates and similar multi-step flows are workflows, not in-process loops. Workflow code stays deterministic; all IO sits in activities. — `INV-06`
- **Nondeterminism, tenancy and error semantics are boundary concerns**, each with its own invariant — see `INV-02` (tenancy), `INV-07`/`INV-09` (errors), `INV-10` (context).

---

## Layered structure

Note the split across two top-level directories. The domain is under `pkg/`; everything else is under `internal/`.

### 1. Domain — `pkg/domain/`

The business core, importable without dragging in infrastructure.

- **Entities** (`pkg/domain/entities/`) — business concepts (`Domain`, `Contact`, `Host`, `Registrar`, `TLD`) carrying their own validation. Constructed through `New<X>(...) (*X, error)` constructors that reject invalid state. — `INV-08`
- **Value objects** — constrained string types carrying their own validation: `DomainName`, `ClIDType`, `E164Type` (phone numbers), `AuthInfoType`, `CCType`, `ContactStatusType`, and the tenancy scope types in `scope.go`. Monetary amounts use `github.com/Rhymond/go-money` rather than a bespoke type.
- **Repository ports** (`pkg/domain/repositories/`) — 25 interfaces across 26 files, describing what the domain needs from persistence. Implementations live in the infrastructure layer.
- **Query and filter types** (`pkg/domain/queries/`) — pagination and read filters, notably `ListItemsQuery{PageSize, PageCursor, Filter}`. Pagination is cursor-based throughout. — `INV-11`

### 2. Application — `internal/application/`

Orchestrates domain objects into use cases.

- **Services** (`services/`) — business use cases, composing repositories and entities.
- **Workflows** (`workflows/`) and **activities** (`activities/`) — Temporal. Workflows orchestrate and stay replay-safe; activities perform all IO.
- **Commands** (`commands/`) — input DTOs for state-changing operations. Read filters live in `pkg/domain/queries/` instead. — `INV-13`
- **Interfaces** (`interfaces/`) — service-level contracts.
- **Controllers, mappers, helpers** — supporting glue.

### 3. Infrastructure — `internal/infrastructure/`

Adapters providing technical capability.

- **Database** (`db/postgres/`) — concrete repository implementations over **GORM**, dropping to raw SQL where a query needs it. Also holds the GORM persistence models, which are distinct types from the domain entities.
- **Temporal** (`temporal/`) — client and worker configuration.
- **Storage** (`storage/`) — S3-compatible object storage.
- **External adapters** — `api/frankfurter` (FX rates), `api/mosapi` (ICANN MoSAPI), `web/ianaregistrars`, `web/icannregistrars`, `web/icannspec5` (registry data feeds), `dns/` (resolver), `auth/`.

### 4. Interface — `internal/interface/`

Inbound adapters translating external protocols into application calls.

- **`rest/`** — HTTP controllers (Gin), request/response types, auth and context middleware.
- **`mcp/`** — Model Context Protocol server.
- **`cli/`** — command-line operations (escrow, JISC).
- **`api/`** — an admin HTTP *client* used by internal tooling, not an inbound handler.

---

## Entry points — `cmd/`

| Binary | Role |
|---|---|
| `cmd/api` | Admin REST API (`ry-admin`) — the main service |
| `cmd/epp` | EPP server (registrar-facing protocol) |
| `cmd/whois` | WHOIS service |
| `cmd/mcp` | MCP server |
| `cmd/askg` | "Ask G" agent CLI |
| `cmd/workers` | Temporal workers (`unified`) |
| `cmd/cli` | Operational tooling (escrow, import, sync, lifecycle, registrars) |
| `cmd/tools`, `cmd/play` | Code generation and scratch utilities |

Dependencies are wired at these composition roots and injected downward; nothing below `cmd/` constructs its own configuration or clients from the environment.

---

## Patterns in use

### Repository pattern

Data access is expressed through repository interfaces defined in `pkg/domain/repositories/` and implemented in `internal/infrastructure/db/postgres/`. This is what allows a storage backend to be swapped, and repositories to be mocked in unit tests.

**Accurately: this holds in the service layer, not universally.** Eleven files in `internal/application/` hold a `*gorm.DB` or `*sql.DB` directly rather than going through a port — nine Temporal activities (bulk escrow import, snapshot/seed, event relay, serial drift, TLD cleanup, spec5 sweep, tombstone backfill) and two services (`csv_to_sqlite_service.go`, `jisc_service.go`). Most are bulk data-movement paths where a row-oriented port would be the wrong shape. Treat direct DB access in the application layer as something that needs justifying in review, not as something that never happens.

### Temporal for long-running processes

Used where execution must survive process restarts and partial failure: escrow import, domain lifecycle sweeps, zone serial drift detection, FX updates, event relay. Provides retries, durable state and replay.

The correctness constraint is `INV-06`: workflow code must be deterministic, so clock, randomness, network and IO belong in activities. This currently holds with no violations.

### Transactional outbox

Domain events are written to a `domain_events` table and drained by a Temporal relay workflow, rather than published inline. Telemetry is never on the delivery path. — `INV-01`

Two known defects in this path are tracked in [#408](https://github.com/onasunnymorning/domain-os/issues/408): the event insert is not currently in the same transaction as the business write, and publish errors are logged rather than returned.

### Dependency injection

Repositories, clients and loggers are constructed at the composition roots in `cmd/` and injected into services and handlers. There is no service locator and no global container.

### Factory / constructor pattern

Entities are created through constructors that validate, so an entity that exists is an entity in a valid state. — `INV-08`

---

## Testing

### What CI actually enforces

On every pull request: `go test -race ./...` across all packages, Go-native API integration tests against a real PostgreSQL service, frontend tests with `npm audit`, a gitleaks secret scan, and Trivy image scanning that fails on CRITICAL/HIGH.

Two gaps worth knowing before relying on CI:

- **`golangci-lint` runs with `--issues-exit-code=0`** — lint findings never fail the build.
- **Coverage is measured and reported, but never gated.** No threshold is enforced, and nothing checks that a change ships with tests.

### What we aim for

These are intentions, not gates — nothing below is machine-checked today:

- Unit tests for entities, value objects and application services, focused on business rules and edge cases.
- Integration tests for repositories and infrastructure adapters, run against real containerised dependencies rather than mocks.
- Workflow tests using the Temporal test framework, covering state transitions and activity orchestration.
- New features and bug fixes arrive with tests. Code that is hard to test is treated as architectural debt — the interfaces and injection above exist largely to make testing possible.

Local runs use `make test`, which starts a throwaway PostgreSQL on port 5433 rather than reusing the dev stack.

---

## Where this document is not authoritative

- **Rules and their real-world adherence** — [`docs/INVARIANTS.md`](docs/INVARIANTS.md). Each invariant carries evidence and a class (A: holds; B: intended, violations listed).
- **Decisions and their rationale** — [`docs/adr/`](docs/adr/). Notably [ADR-0006](docs/adr/0006-tenancy-model.md) for the two-sided tenancy model behind `INV-02`.
- **Technology choices** — [`stack.md`](stack.md). **Contains at least one claim contradicted by the code; see `UNR-03` in `docs/INVARIANTS.md`.**
