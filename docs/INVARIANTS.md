# Architectural invariants — DRAFT for review

**Analysed at:** `0dccd6d59414984245a36eb3d9f2a77b440ca164`
**Date:** 2026-08-10
**Status:** Draft. Nothing here is settled. Expected review action is deletion and correction.

This document exists so architectural review can be a link to a section ID instead of an essay.

## How to read this

Every claim carries `file:line` evidence at the SHA above. Nothing is asserted without it.

| Class | Meaning |
|---|---|
| **A — Enforced** | Holds consistently everywhere it applies. |
| **B — Intended but leaky** | Holds in most places; every violation is cited individually. |
| **C — Observed pattern** | A convention exists; whether it is deliberate is unknown. Stated as a question. |
| **D — Contradiction** | Two parts of the repo do incompatible things. No rule proposed. |

**ID rules.** `INV-nn` are the confirmed invariants. `PROP-nn` are proposals awaiting confirmation — **they are not rules and must not be cited as rules.** `UNR-nn` are unresolved contradictions. IDs are stable and are never reused after retirement.

## Index

| ID | Rule | Class | Section |
|---|---|---|---|
| INV-01 | Outbox is the path of record; telemetry is never load-bearing | A | Invariants |
| INV-02 | Tenant scope required on every executor call | **D** | Unresolved |
| INV-03 | No vendor LLM SDK outside `ModelProvider` | A | Invariants |
| INV-04 | Evidence provenance mandatory on terminal agent outcomes | **B** | Invariants |
| INV-05 | Reconciliation before record creation | **B** (process only) | Invariants |
| INV-06 | Determinism boundaries around workflows | A | Invariants |
| INV-07 | Errors are wrapped with `%w`, not `%v` | **B** | Invariants |
| PROP-01…03, 05…10 | Observed conventions | C | Proposed |
| ~~PROP-04~~ | Retired — promoted to INV-07 on 2026-08-10 | — | — |
| UNR-01…03 | Contradictions | D | Unresolved |

---

# Invariants

## INV-01 — The event outbox is the path of record; telemetry is never load-bearing

**Rule.** Delivery-guaranteed event flow depends only on the database outbox and the Temporal relay. No span, exporter, or collector is on the correctness path.

**Why.** Telemetry is sampled, lossy, and may be disabled without notice. Anything that must not be lost cannot depend on it. Instrumenting the relay is fine; substituting telemetry for it is not.

**Class: A** — holds, but note *why* it holds (below).

**Evidence.** Port: [`pkg/domain/repositories/event_publisher.go:9`](../pkg/domain/repositories/event_publisher.go#L9). Outbox row and flag: [`internal/infrastructure/db/postgres/domain_event.go:25`](../internal/infrastructure/db/postgres/domain_event.go#L25) (`Published bool` — comment reads "outbox relay flag"). Relay: [`internal/application/workflows/eventRelay.go:40`](../internal/application/workflows/eventRelay.go#L40), [`internal/application/activities/eventRelayActivities.go:80`](../internal/application/activities/eventRelayActivities.go#L80). `TraceID` on an event is a plain string read from `ctx.Value("trace_id")`, not a span handle — [`internal/application/services/domain_service.go:1761`](../internal/application/services/domain_service.go#L1761).

**Caveat on the classification.** `go.opentelemetry.io` appears in `go.sum` only — zero first-party Go imports, nothing in `go.mod`'s require blocks. There is also no AMQP/Kafka/NATS/SNS/SQS anywhere. **This invariant currently holds because no telemetry infrastructure exists in application code at all**, not because a boundary is being defended. It is untested against the case it was written for. Treat it as A-by-vacancy.

**Two adjacent defects, surfaced not fixed:**
1. **The outbox is not transactional.** The event insert is a separate `db.Create` ([`internal/infrastructure/db/postgres/event_publisher.go:46`](../internal/infrastructure/db/postgres/event_publisher.go#L46)) with no shared `tx` with the business write. A crash between the two loses the event.
2. **Publish failures are swallowed in all six producers.** The pattern is `if err := ...Publish(...); err != nil { log.Printf(...) }` — the business operation succeeds even if nothing reaches the outbox: [`domain_service.go:1787`](../internal/application/services/domain_service.go#L1787), [`registrar_service.go:329`](../internal/application/services/registrar_service.go#L329), [`host_service.go:347`](../internal/application/services/host_service.go#L347), [`contact_service.go:152`](../internal/application/services/contact_service.go#L152), [`accreditation_service.go:153`](../internal/application/services/accreditation_service.go#L153), [`phase_service.go:220`](../internal/application/services/phase_service.go#L220).

---

## INV-03 — No vendor LLM SDK is imported outside the `ModelProvider` boundary

**Rule.** Vendor model SDKs are imported in exactly one adapter package. Everything else depends on the `ModelProvider` interface.

**Why.** Model vendors change on a timescale shorter than the product. Confining the SDK to one package means a swap is one new adapter, not a codebase sweep. It also keeps vendor request/response shapes out of the domain vocabulary.

**Class: A** — holds cleanly across the full import graph.

**Evidence.** Interface: [`internal/askg/provider.go:108`](../internal/askg/provider.go#L108) — `Generate(ctx, ModelRequest) (ModelResponse, error)` and `Name() string`. The **only** vendor-LLM imports in the entire non-vendor tree: [`internal/askg/provider/anthropic/anthropic.go:14-15`](../internal/askg/provider/anthropic/anthropic.go#L14). Zero hits for `go-openai`, `genai`, `generative-ai-go`, `cohere`, `mistral`, `ollama`, `langchain`, or direct calls to `api.anthropic.com` / `api.openai.com`. Composition roots import the *adapter package*, never the SDK: [`cmd/api/ry-admin/ryAdminAPI.go:16`](../cmd/api/ry-admin/ryAdminAPI.go#L16), [`cmd/askg/main.go:26`](../cmd/askg/main.go#L26). A second implementation exists for tests: [`internal/askg/provider/fake_provider.go:37`](../internal/askg/provider/fake_provider.go#L37).

**Soft coupling worth a decision.** Model *identifier strings* (`DefaultModel`, `DefaultClassifier` at [`anthropic.go:21-22`](../internal/askg/provider/anthropic/anthropic.go#L21)) are referenced from `cmd/` wiring, so swapping providers still means touching entrypoints. Not an SDK leak — flagging it as the one seam that would resist a swap.

---

## INV-04 — Terminal agent outcomes carry evidence provenance

**Rule.** Every claim in `Result.Answer` must be supported by an entry in `Result.Evidence`.

**Why.** An agent answer about registry state that cannot be traced to the retrieval that produced it is unfalsifiable, and therefore unusable in an operational decision. Provenance is what makes the output auditable rather than merely plausible.

**Class: B — intended, documented, enforced by nothing.**

The rule is written down at [`internal/askg/result.go:27`](../internal/askg/result.go#L27) — "Every claim in Result.Answer must be supported by something in Evidence." There is no mechanism behind it. `Result` ([`result.go:35`](../internal/askg/result.go#L35)) is a plain struct with all fields exported: **no constructor, no `Validate()` method, no method set at all.** `Evidence []Evidence` ([`result.go:49`](../internal/askg/result.go#L49)) is an ordinary nil-able slice.

**Violations, individually:**

1. [`internal/askg/eval/runner.go:119-122`](../internal/askg/eval/runner.go#L119) — constructs `&askg.Result{Outcome: askg.OutcomeEscalate, Reason: ...}` with the `Evidence` field **omitted entirely**. Proof the zero-provenance shape is reachable and already reached.
2. [`internal/askg/orchestrator.go:187-193`](../internal/askg/orchestrator.go#L187) — JSON-parse-failure fallback; passes `allEvidence` through unchecked.
3. [`internal/askg/orchestrator.go:210-214`](../internal/askg/orchestrator.go#L210) — the happy path; same unchecked passthrough.
4. [`internal/askg/orchestrator.go:247-253`](../internal/askg/orchestrator.go#L247) — `escalateWithEvidence`, reached from `:74` (model error), `:97` (max tokens), `:157` (iteration cap). On a first-iteration model failure it emits `Evidence: nil`.

For 2–4: `allEvidence` is declared at [`orchestrator.go:52`](../internal/askg/orchestrator.go#L52) and appended only at [`:137`](../internal/askg/orchestrator.go#L137), inside the tool-call branch. A first-turn direct answer never enters that branch, so the slice is nil.

**The one guard is weaker than its name.** `ScoreProvenanceIntegrity` ([`internal/askg/eval/scoring.go:93-119`](../internal/askg/eval/scoring.go#L93)) checks only the *reverse* direction — that each evidence entry maps to a tool call that ran, catching fabricated provenance. An empty slice yields zero orphans and **passes**, reporting "all 0 evidence entries have matching tool calls". It also lives in the offline eval harness, not the request path: the REST handler returns `Result` unchecked at [`internal/interface/rest/agent_controller.go:91`](../internal/interface/rest/agent_controller.go#L91).

---

## INV-05 — Records are never created against a truncated or unreconciled dataset

**Rule.** Bulk record creation does not proceed against a dataset that has not been reconciled against its source.

**Why.** A truncated escrow deposit that ingests silently produces a registry that looks healthy and is wrong. The damage is discovered later, by someone else, in production.

**Class: B — this is a process invariant held by operator discipline. It is not enforced by code. State that plainly.**

**No reconciler over records exists.** Every reconciliation-shaped thing in the repo is either about Temporal *schedules* ([`internal/infrastructure/bootstrap/ensure.go:153`](../internal/infrastructure/bootstrap/ensure.go#L153)) or is reporting-only.

**Violations, individually:**

1. **Post-ingestion verification is explicitly non-blocking, by its own doc comment.** [`internal/application/activities/verify_ingestion.go:82`](../internal/application/activities/verify_ingestion.go#L82): "Verification failures are informational — they do not fail the workflow."
2. **Its result is stored and never branched on.** [`internal/application/workflows/escrowImport.go:547`](../internal/application/workflows/escrowImport.go#L547) writes `state.VerificationPassed`; [`:566`](../internal/application/workflows/escrowImport.go#L566) copies it into the result. Nothing reads it as a condition. Ingestion already ran at [`:392`](../internal/application/workflows/escrowImport.go#L392).
3. **The one check that would catch truncation is downgraded to a warning.** `entity_count_consistency` ([`internal/application/activities/escrow_import.go:4638`](../internal/application/activities/escrow_import.go#L4638)) compares staged domain count against the source-analysis sum — exactly the truncation check. Its `Severity` is `"warning"` at [`:4640`](../internal/application/activities/escrow_import.go#L4640). `AddCheck` ([`:4374`](../internal/application/activities/escrow_import.go#L4374)) fails the report **only** on `Severity == "error"`. A staged DB missing 40% of its domains logs a mismatch and proceeds to ingestion.
4. **The sync path has no completeness guard.** [`internal/application/workflows/syncRegistrarsWorkflow.go:144`](../internal/application/workflows/syncRegistrarsWorkflow.go#L144) — the `Count == 0` test selects bootstrap-vs-incremental mode; it is not a sanity check. Nothing tests whether a fetched IANA list is suspiciously small before bulk-creating registrars.
5. **The serial-drift path writes runs unconditionally.** [`internal/application/activities/serialDriftActivities.go:172-207`](../internal/application/activities/serialDriftActivities.go#L172) persists the run and every observation regardless of how many nameservers responded; failed probes are stored with a populated `Error` field.

**A real gate does exist**, and is worth keeping in view: [`internal/application/workflows/escrowImport.go:340`](../internal/application/workflows/escrowImport.go#L340) halts before ingestion when `!qaOut.Passed`. Three `error`-severity checks can trip it (`unmapped_primary_clids`, `null_primary_clids`, `registrar_mapping_completeness`). The gate works; the truncation check simply is not wired into it.

---

## INV-06 — Nondeterminism sits behind activity-shaped boundaries

**Rule.** Workflow code is deterministic and stateless per request. Clock, randomness, network, and IO live in activities. The retry layer stays swappable.

**Why.** Temporal replays workflow code against history. A wall-clock read or a network call in workflow scope produces a nondeterminism panic on replay — usually in production, usually during an incident, usually on the workflow you least want to lose.

**Class: A** — holds strongly, and appears to be actively maintained.

**Evidence.** Across every file in `internal/application/workflows/` **including tests**: zero occurrences of `time.Now()`, `math/rand`, `crypto/rand`, `uuid.New`, `net/http`, or `http.Get/Post/Client/NewRequest`. The `time` import appears in 15 files and is used only for `time.Duration` constants, `time.Time` struct fields, and `time.RFC3339` formatting. Against that: **69 `workflow.Now(ctx)` calls and 72 `workflow.ExecuteActivity` calls** across the package — e.g. [`serialDrift.go:17`](../internal/application/workflows/serialDrift.go#L17), [`tombstoneBackfill.go:45`](../internal/application/workflows/tombstoneBackfill.go#L45), [`updateFX.go:40`](../internal/application/workflows/updateFX.go#L40).

Nondeterminism is correctly concentrated in `internal/application/activities/` (~40 files import `net/http`; `time.Now()` at [`eventRelayActivities.go:169`](../internal/application/activities/eventRelayActivities.go#L169), `math/rand` at [`verify_ingestion.go:9`](../internal/application/activities/verify_ingestion.go#L9)) — legal there. The boundary looks deliberately maintained: [`internal/application/activities/listPurgeableDomains.go:21`](../internal/application/activities/listPurgeableDomains.go#L21) carries a comment explaining that the reference time is passed *in from the workflow* rather than defaulting to the activity's own clock.

**One item outside Temporal scope, flagged separately.** [`internal/askg/tools.go:341`](../internal/askg/tools.go#L341) uses `time.Now().UTC()` inside business logic (`deriveDomainRGPPhase`). Ask G is not a Temporal workflow, so this carries no replay risk — but it makes the function untestable at date boundaries without clock injection, and its own comment says it "mirrors the MCP server's deriveRGPPhase function", so the duplicated logic can drift.

---

## INV-07 — Errors are wrapped with `%w`, not `%v`

*Confirmed 2026-08-10. Promoted from `PROP-04`, which is retired.*

**Rule.** An error included in a `fmt.Errorf` message is formatted with `%w`.

**Why.** `%v` flattens an error to a string and breaks the chain, so no caller above it can use `errors.Is` or `errors.As`. This codebase defines 159 sentinel errors, mostly in `pkg/domain/entities` — every one of them is unmatchable through a `%v`. The cost is paid at the point someone tries to handle a specific failure and finds they can only string-match it.

**Class: B — the convention is overwhelming, and 49 sites break it.**

444 `fmt.Errorf(... %w ...)` sites against **49** non-wrapping ones. All 49 are in non-test code; test code is clean. (Count is `errorlint`'s with `errorf: true`, run against this SHA. A `%v`-only grep undercounts it at 31 — `errorlint` also catches `%s`-on-error and cases where the error is not the trailing argument.)

**Violations — nine files, one holding half:**

| Count | File |
|---|---|
| **25** | [`cmd/cli/registrars/importer/importRegistrars.go`](../cmd/cli/registrars/importer/importRegistrars.go) |
| 5 | [`internal/application/services/dnssec_service.go`](../internal/application/services/dnssec_service.go) |
| 4 | [`internal/infrastructure/web/icannregistrars/getCreateCommands.go`](../internal/infrastructure/web/icannregistrars/getCreateCommands.go) |
| 4 | [`internal/application/commands/registrar_commands.go`](../internal/application/commands/registrar_commands.go) |
| 3 | [`internal/infrastructure/web/ianaregistrars/iana_repository.go`](../internal/infrastructure/web/ianaregistrars/iana_repository.go) |
| 2 | [`pkg/domain/entities/domain.go:499`](../pkg/domain/entities/domain.go#L499), [`:505`](../pkg/domain/entities/domain.go#L505) |
| 2 | [`internal/infrastructure/web/icannspec5/icann_repository.go`](../internal/infrastructure/web/icannspec5/icann_repository.go) |
| 2 | [`internal/infrastructure/web/icannregistrars/readFile.go`](../internal/infrastructure/web/icannregistrars/readFile.go) |
| 2 | [`cmd/cli/registrars/dbimporter/importRegistrarsDB.go`](../cmd/cli/registrars/dbimporter/importRegistrarsDB.go) |

By layer: `cmd/cli` 27 · `internal/infrastructure` 11 · `internal/application` 9 · `pkg/domain` 2.

**The two in `pkg/domain/entities/domain.go` are the ones that matter.** An unwrapped error in the domain layer defeats `errors.Is` for every caller above it. The 25 in the CLI importer are the bulk of the work and the lowest stakes.

Tracked for cleanup and enforcement in [#404](https://github.com/onasunnymorning/domain-os/issues/404).

---

# Proposed — pending Geoff's confirmation

**These are not rules.** They are consistent patterns found in the code, presented as questions. Answer each yes / no / no-longer. Nothing here should be cited in review until it has been promoted.

### PROP-01 — Transaction boundaries have no owner
Only 8 explicit transaction sites exist, spread across three layers: services ([`jisc_service.go:52`](../internal/application/services/jisc_service.go#L52), [`:513`](../internal/application/services/jisc_service.go#L513), [`csv_to_sqlite_service.go:334`](../internal/application/services/csv_to_sqlite_service.go#L334), [`:448`](../internal/application/services/csv_to_sqlite_service.go#L448)), activities ([`serialDriftActivities.go:175`](../internal/application/activities/serialDriftActivities.go#L175), [`escrow_import.go:2602`](../internal/application/activities/escrow_import.go#L2602), [`:3143`](../internal/application/activities/escrow_import.go#L3143)), and repositories ([`fx_repository.go:30`](../internal/infrastructure/db/postgres/fx_repository.go#L30)). The entire core registry service set (domain, registrar, host, contact, tld, accreditation) opens none — which is also why INV-01's outbox write is not atomic with its business write.
**Q: Should transaction ownership sit at one named layer, or is per-case placement intentional?**

### PROP-02 — Validation lives in entity constructors, not at the edge
30 `New<X>(...) (*X, error)` constructors and 24 `Validate()`/`IsValid()` methods across 74 files in `pkg/domain/entities/` (e.g. [`registrar.go:124`](../pkg/domain/entities/registrar.go#L124), [`contact.go:147`](../pkg/domain/entities/contact.go#L147), [`tld.go:65`](../pkg/domain/entities/tld.go#L65)). Against that, only **18** `binding:"..."` gin tags exist across the whole REST layer. The edge appears to trust the domain rather than duplicate its checks.
**Q: Is "validation is the entity's job, handlers don't re-check" the rule?**

### PROP-03 — Sentinel errors are an entity-layer convention
159 `Err… = errors.New(...)` declarations, concentrated in `pkg/domain/entities` (36 files). Outer layers barely declare any: services 4, queries 2, commands 2, and one apiece in rest / infrastructure-web / dns / db / api.
**Q: Are sentinels meant to be a domain-vocabulary tool specifically, with outer layers wrapping rather than declaring?**

### ~~PROP-04~~ — promoted, see [INV-07](#inv-07--errors-are-wrapped-with-w-not-v)
Confirmed 2026-08-10. The ID `PROP-04` is retired and will not be reused.

### PROP-05 — Repository methods take `ctx` first
121 of 124 interface methods in `pkg/domain/repositories/` lead with `ctx context.Context`. The three exceptions: [`idGenerator_interface.go:5`](../pkg/domain/repositories/idGenerator_interface.go#L5), [`:6`](../pkg/domain/repositories/idGenerator_interface.go#L6), [`iana_interface.go:8`](../pkg/domain/repositories/iana_interface.go#L8).
**Q: Rule, with three to fix — or are the ID generator and IANA list legitimately context-free?**

### PROP-06 — Pagination is cursor-based, never offset
Where repository interfaces paginate they use `pageSize int, pageCursor string`: [`ianaRegistrar_interface.go:12`](../pkg/domain/repositories/ianaRegistrar_interface.go#L12), [`phase_repository_interface.go:16`](../pkg/domain/repositories/phase_repository_interface.go#L16), [`:17`](../pkg/domain/repositories/phase_repository_interface.go#L17). No `OFFSET`-style signature appears in the port layer.
**Q: Is cursor pagination a rule for new list endpoints?**

### PROP-07 — Eager loading is expressed as a `preload…bool` parameter
[`tld_interface.go:12`](../pkg/domain/repositories/tld_interface.go#L12) (`preloadAll`), [`registrar_interface.go:13`](../pkg/domain/repositories/registrar_interface.go#L13) (`preloadTLDs`), [`domain_repository.go:16`](../pkg/domain/repositories/domain_repository.go#L16) and [`:32`](../pkg/domain/repositories/domain_repository.go#L32) (`preloadHosts`). Each names a different relation, so the idiom is a boolean flag rather than a shared option type.
**Q: Keep the boolean-flag idiom, or is this drifting toward a load-options struct?**

### PROP-08 — Logging idiom is determined by layer
**Four** idioms are in use. Non-test counts:

| Package | `log.*` | `slog.` | Temporal logger | `zap.` |
|---|---|---|---|---|
| `application/workflows` | 0 | 0 | **25** | 0 |
| `application/activities` | 4 | 0 | **58** | 1 |
| `askg` | 0 | **56** | 0 | 0 |
| `application/services` | **209** | 17 | 0 | 0 |
| `interface/rest` | 5 | 13 | 0 | 0 |
| `infrastructure` | 24 | 2 | 0 | 8 |
| `cmd` | — | — | 0 | **47** |

Temporal code is unambiguous and clean. Elsewhere the split reads as age rather than policy: the newest code (`askg`) is all-`slog`, the oldest (`services`) is all-`log.Printf`, and `zap` is confined to composition roots plus the two infrastructure types it is injected into ([`event_publisher.go:13`](../internal/infrastructure/db/postgres/event_publisher.go#L13), [`lifecycleActivities.go`](../internal/application/activities/lifecycleActivities.go)). There is no project logger wrapper type — `zap` is passed by dependency injection, `slog` and `log` are used package-globally.
**Q: Is `slog` the direction of travel — and if so, what happens to the injected `zap` loggers, which are the only ones that are testable by injection?**

### PROP-09 — Schema is GORM `AutoMigrate`, env-gated, with no versioned migrations
[`internal/infrastructure/db/postgres/connection.go:15`](../internal/infrastructure/db/postgres/connection.go#L15), gated by `AUTO_MIGRATE` (default `false` — [`internal/config/env_registry.go:69`](../internal/config/env_registry.go#L69)). Ownership is asserted in a comment at [`cmd/api/ry-admin/seed.go:114`](../cmd/api/ry-admin/seed.go#L114): "admin-api owns migration; we only ever write rows". There are **no** `.sql` migration files anywhere — the only SQL-ish init script is `docker/postgres-init/01-temporal.sh`. A note at [`connection.go:82`](../internal/infrastructure/db/postgres/connection.go#L82) records that AutoMigrate never drops indexes, so cleanup is manual.
**Q: Is "one service owns AutoMigrate, no migration tool" the deliberate position?**

### PROP-10 — Write and read models are split into `commands/` and `queries/`
`internal/application/commands/` holds per-aggregate command types; `internal/application/queries/` holds filters and read shapes. The split is consistently populated.
**Q: Is this a CQRS commitment new work must follow, or a naming convention that happens to have held?**

---

# Unresolved

Contradictions. No rule proposed for any of these — they need a decision.

## INV-02 — Tenant scope on every executor call *(reclassified: Class D)*

The rule as stated does not map onto this codebase, and the code does three incompatible things.

**First, the naming.** The only thing called an `Executor` is an LLM tool executor: [`internal/askg/toolexec.go:9`](../internal/askg/toolexec.go#L9), `Execute(ctx, ToolCall, CallerScope) ToolResult`. Its scope type is **not a tenant** — [`internal/askg/result.go:62`](../internal/askg/result.go#L62) is `CallerScope{ UserID string }`, and the doc comment above it says the underlying services return unscoped data and the field "enables future registrar-scoped filtering." It is used in exactly one `slog` line at [`internal/askg/tools.go:102-124`](../internal/askg/tools.go#L102) and is never passed into `executeDomain` / `executeTLD` / `executeKnowledgeSearch`. **The isolation gate described by this invariant does not exist at that layer.**

**Second, the service layer does it three ways:**

1. **Explicit, required, immediately after ctx — one vertical slice only.** Zone slaving / serial drift: [`internal/application/interfaces/zone_slaving_interface.go:25-43`](../internal/application/interfaces/zone_slaving_interface.go#L25) — all seven methods take `tenantID string`. It is genuinely enforced down to SQL: [`internal/infrastructure/db/postgres/serial_drift_repository.go:40`](../internal/infrastructure/db/postgres/serial_drift_repository.go#L40) and 11 more sites all carry `WHERE ... tenant_id = ?`, with a uniqueness index at [`zone_slaving.go:14`](../internal/infrastructure/db/postgres/zone_slaving.go#L14).
2. **No tenant at all — the entire core registry domain.** [`internal/application/interfaces/domain_interface.go:14-57`](../internal/application/interfaces/domain_interface.go#L14): not one method takes a scope. Same for [`registrar_interface.go:13-22`](../internal/application/interfaces/registrar_interface.go#L13). Note `ClID` here is the *operand's* primary key — which record you are acting on — not a caller scope; it should not be read as tenant threading.
3. **Derived from `context.Context` — but only for audit stamping, never authorization.** `trace_id` / `correlation_id` / `userid` are pulled from context and written onto events in all six producer services (e.g. [`domain_service.go:1761,1764,1780`](../internal/application/services/domain_service.go#L1761)). No query is filtered by them.

**Third, and worst: where the tenant comes from.** [`internal/interface/rest/zone_slaving_controller.go:32-38`](../internal/interface/rest/zone_slaving_controller.go#L32) reads it from an unauthenticated `X-Tenant-ID` header, falling back to a `tenant_id` query param. Handlers reject only the empty string ([`:66`](../internal/interface/rest/zone_slaving_controller.go#L66), `:104`, `:149`, `:197`, `:225`, `:264`). Nothing cross-checks it against the caller's identity. The one place tenant isolation *is* implemented is the one place a caller can choose their own tenant. The workflow trigger path is the same, via the request body: [`workflow_controller.go:498-502`](../internal/interface/rest/workflow_controller.go#L498).

**Note.** Tenant isolation is asserted in the agent eval suite — `CategoryTenantIsolation` ([`internal/askg/eval/eval.go:36`](../internal/askg/eval/eval.go#L36)), `ScoreTenantIsolation` ([`eval/scoring.go:24`](../internal/askg/eval/scoring.go#L24), which greps the model's answer text), `TestOrchestrator_TenantScopeThreading` ([`orchestrator_test.go:291`](../internal/askg/orchestrator_test.go#L291)). The tests describe an intent the service layer does not implement.

**What needs deciding:** whether the zone-slaving slice is the target shape the rest should converge on, or a local experiment — and, separately, whether `X-Tenant-ID` being self-asserted is known and accepted.

## UNR-01 — The domain layer imports outward, while the docs say it cannot

[`architecture.md:11`](../architecture.md#L11) states the domain layer "is dependency-free (no imports of database drivers, HTTP frameworks, etc.)", and [`.cursorrules`](../.cursorrules) makes that file binding on every agent working in this repo. 13 files in `pkg/domain/repositories/` import inward-layer packages — **but they are two different problems with two different fixes, and only one is a real layering violation.**

`pkg/domain/repositories/` is a genuine ports package: 26 interfaces, no adapters. (It does hold 4 mock implementations — [`MockDomainRepository`](../pkg/domain/repositories/domain_repository.go#L38), [`MockRegistrarRepository`](../pkg/domain/repositories/registrar_interface.go#L25), [`MockHostRepository`](../pkg/domain/repositories/host_repository.go#L28), [`MockHostAddressRepository`](../pkg/domain/repositories/hostAddress_repository.go#L17) — which is a separate, minor question about test doubles shipping in the port package.)

### Group 1 — misfiled value types, not a layering violation (11 files)

These import `internal/application/queries` purely for pagination and filter parameter types: `ListItemsQuery`, `ListDomainsFilter`, `ListContactsFilter`, `ListHostsFilter`, `ListTldsFilter`, `ListNndnsFilter`, `ListTombstonesFilter`, `ListRegistryOperatorsFilter`, `ActiveDomainsWithHostsQuery`.

Files: [`domain_repository.go:8`](../pkg/domain/repositories/domain_repository.go#L8), [`contact_repository.go:6`](../pkg/domain/repositories/contact_repository.go#L6), [`host_repository.go:6`](../pkg/domain/repositories/host_repository.go#L6), [`registrar_interface.go:6`](../pkg/domain/repositories/registrar_interface.go#L6), [`tld_interface.go:6`](../pkg/domain/repositories/tld_interface.go#L6), [`nndn_interface.go:6`](../pkg/domain/repositories/nndn_interface.go#L6), [`premiumLabel_repository.go:6`](../pkg/domain/repositories/premiumLabel_repository.go#L6), [`premiumList_repository.go:6`](../pkg/domain/repositories/premiumList_repository.go#L6), [`registryOperator_repository.go:6`](../pkg/domain/repositories/registryOperator_repository.go#L6), [`spec5Label_interface.go:6`](../pkg/domain/repositories/spec5Label_interface.go#L6), [`tombstone_interface.go:6`](../pkg/domain/repositories/tombstone_interface.go#L6).

**The dependency direction is not actually inverted here.** `go list -deps ./internal/application/queries` returns exactly one first-party package: `pkg/domain/entities`. No infrastructure, no GORM, no HTTP — its third-party deps are the same value-object libraries the entities use. These are domain-shaped value types that happen to be filed under `internal/application/`.

**Fix is a move, not a redesign:** relocate the query/filter types to `pkg/domain` (e.g. `pkg/domain/queries`), and these 11 imports become legal with no signature changes. Cheap.

### Group 2 — a real leak, and it is the expensive one (2 files)

[`fx_interface.go:6`](../pkg/domain/repositories/fx_interface.go#L6) and [`tldDNSRecord_repository.go:6`](../pkg/domain/repositories/tldDNSRecord_repository.go#L6) import `internal/infrastructure/db/postgres` and type their method signatures against GORM persistence models.

- **`FXRepository` is inconsistent within itself.** [`UpdateAll(ctx, fxs []*postgres.FX)`](pkg/domain/repositories/fx_interface.go#L13) takes the GORM model, while [`ListByBaseCurrency`](../pkg/domain/repositories/fx_interface.go#L14) returns `[]*entities.FX`. Writes speak Postgres, reads speak domain. And [`entities.FX` exists](../pkg/domain/entities/fx.go#L18) with real behaviour on it (a `Convert` method) — it is simply not used on the write path. [`postgres.FX`](../internal/infrastructure/db/postgres/fx.go#L10) is a different shape: `gorm:"primaryKey"` tags, `CreatedAt`/`UpdatedAt`, differently-named fields.
- **`TLDDNSRecordRepository` uses `postgres.TLDDNSRecord` in all three methods** ([`:11-13`](../pkg/domain/repositories/tldDNSRecord_repository.go#L11)) — and there is **no** `entities.TLDDNSRecord`. That concept was never modelled in the domain at all; the persistence model is its only representation.

**The concrete cost, measured:** `go list -deps` gives `pkg/domain/entities` **19** third-party transitive dependencies. `pkg/domain/repositories` has **76**, including `github.com/jackc/pgx/v5/pgconn`, `pgproto3`, and `lib/pq` — actual Postgres wire-protocol drivers, now reachable from the domain layer. **These two files alone cause that.** This is precisely the "no imports of database drivers" case `architecture.md:11` names.

**Fix is real work:** introduce `entities.TLDDNSRecord`, use `entities.FX` on the write path, and add mappers in the postgres adapter.

**What needs deciding:** confirm Group 1 is a package-placement mistake and schedule the move; and decide whether Group 2 gets modelled properly or the "dependency-free" claim is narrowed to `pkg/domain/entities` only — which is where it currently holds.

## UNR-02 — Two spellings of the correlation ID; the only reader is dead code with a passing test

The middleware writes `"correlation_id"`: [`internal/interface/rest/stream_middleware.go:62`](../internal/interface/rest/stream_middleware.go#L62). The six producer services read `"correlation_id"` and work.

But [`internal/application/helpers/helpers.go:12`](../internal/application/helpers/helpers.go#L12) reads `"correlationID"` — camelCase — and would therefore never find the value. It is moot only because `getCorrelationID` is unexported and called from nothing but its own test. That test passes because it sets the camelCase key itself: [`helpers_test.go:18`](../internal/application/helpers/helpers_test.go#L18). So the test certifies the spelling production never writes.

Related: all of these keys are **bare strings** used as `context.WithValue` keys, which is the `staticcheck` SA1029 pattern — currently unreported because lint is non-blocking in CI. The repo already has the typed-key idiom elsewhere: [`cmd/epp/eppServer.go:28`](../cmd/epp/eppServer.go#L28) defines `type contextKey string` and uses it properly.

**What needs deciding:** delete the dead helper, or fix its key and wire it up — and whether typed context keys become the standard.

## UNR-03 — `architecture.md` and `stack.md` assert things the code does not do

Both files are named by [`.cursorrules`](../.cursorrules) as authority every agent "must ALWAYS consult", so drift here propagates into agent behaviour.

1. [`architecture.md:22`](../architecture.md#L22) locates the domain layer at `internal/domain`, repeated at [`:24`](../architecture.md#L24), [`:25`](../architecture.md#L25), and in the repository rule at [`:51`](../architecture.md#L51). **That directory does not exist.** Entities and repositories are in `pkg/domain/`.
2. [`stack.md`](../stack.md) lists **RabbitMQ** as the message queue. `rg -i 'amqp|rabbitmq'` over all Go files returns nothing. There is no message broker: event delivery is the Postgres outbox plus S3 archive via Temporal (INV-01). Redis and MinIO, listed alongside it, *are* real (`go.mod:34`, `:28`).

**What needs deciding:** whether these files get corrected, superseded by this document, or retired. Deliberately not fixed here — out of scope for this ticket.

---

# Enforcement assessment

Whether each item could migrate from prose to a CI gate. **Assessment only — nothing implemented in this ticket.**

| ID | Enforceable | Mechanism | Effort | Notes |
|---|---|---|---|---|
| INV-01 | partial | import lint (`depguard`) | S | Ban `go.opentelemetry.io` from the relay/publisher packages. Cannot mechanically check "telemetry isn't load-bearing" in general — but the concrete rule is one deny-rule. |
| INV-01 defect 1 (non-transactional outbox) | no | code review | — | Requires knowing which writes belong together. |
| INV-01 defect 2 (swallowed publish) | yes | custom analyser | M | Flag `Publish(...)` whose error is only logged. Narrow enough to be low-noise. |
| INV-02 | partial | custom analyser | L | Could assert every exported method on a service interface takes `tenantID` after `ctx`. Only worth building once the target shape is decided — see Unresolved. |
| INV-03 | **yes** | import lint (`depguard`) | **S** | Highest value per unit of effort in this table. Allow vendor LLM SDKs only under `internal/askg/provider/**`; deny everywhere else. One config block. |
| INV-04 | yes | type change + analyser | M | Strongest fix is structural, not a lint: unexport `Result`'s fields and add `NewAnswer(...)`/`NewEscalation(...)` constructors that reject empty `Evidence` for `OutcomeAnswer`. A `Validate()` called at [`agent_controller.go:91`](../internal/interface/rest/agent_controller.go#L91) is the cheap interim. |
| INV-05 | partial | grep-level CI check | S | Cannot verify reconciliation happened. **Can** assert `entity_count_consistency` has `Severity: "error"` — i.e. pin the gate so it cannot be silently downgraded again. Genuine enforcement needs branching on `VerificationPassed`, which is a code fix, not a gate. |
| INV-06 | **yes** | architecture test | **S** | Assert no file under `internal/application/workflows/` imports `net/http`, `math/rand`, `crypto/rand`, or a uuid package, and contains no `time.Now(`. A ~30-line Go test using `go/parser`. Currently holds at zero violations, so it lands green on day one. |
| INV-07 | **yes** | lint (`errorlint`, `errorf: true`) | **S** | Verified against this SHA: `errorlint` is the linter that catches this — not `err113`/`wrapcheck`, which answer different questions. 49 offenders across 9 files to clear first; 25 are in one file. Enabling `errorlint`'s other two checks (`asserts`, `comparison`) raises the count to 87 — a separate cleanup with different risk, since `==` → `errors.Is` can change behaviour where a sentinel is compared by identity. Tracked in [#404](https://github.com/onasunnymorning/domain-os/issues/404). |
| PROP-01 | no | code review | — | Placement is a judgement call. |
| PROP-02 | partial | architecture test | M | Can assert every entity has a `New…` constructor returning `error`; cannot assert the validation is meaningful. |
| PROP-03 | no | code review | — | |
| PROP-05 | yes | architecture test | S | Parse `pkg/domain/repositories/`, assert param 0 is `context.Context`. 3 exceptions to allowlist or fix. |
| PROP-06 | partial | architecture test | M | Detectable by signature shape; naming-dependent, so somewhat brittle. |
| PROP-07 | no | code review | — | |
| PROP-08 | yes | import lint (`depguard`) | S | Ban `log` in chosen packages once the direction is confirmed. Blocked on PROP-08's answer. |
| PROP-09 | no | code review | — | |
| PROP-10 | no | code review | — | |
| UNR-01 grp 1 (11 files) | **yes** | import lint (`depguard`) | **S** | Gate is one deny-rule: no `internal/**` from `pkg/domain/**`. Blocked until the query/filter types move to `pkg/domain` — that move is itself S and mechanical. |
| UNR-01 grp 2 (2 files) | yes | same deny-rule | M | Same gate catches it, but the fix is modelling work (`entities.TLDDNSRecord`, `entities.FX` on writes, mappers) — not a rename. Do this before turning the rule on, or allowlist the two files meanwhile. |
| UNR-02 | partial | `staticcheck` SA1029 | S | Already detected by a linter that is already enabled — see the blocker below. |
| UNR-03 | no | code review | — | Doc-vs-code drift is not mechanically checkable. |

**Two blockers that gate this entire table:**

1. **CI lint is non-blocking.** [`.github/workflows/ci.yaml`](../.github/workflows/ci.yaml) runs golangci-lint with `--issues-exit-code=0`. **Every lint-based mechanism above is inert until that flag is removed.** That single change is the prerequisite for roughly half this table.
2. **No import-restriction linter is configured.** [`.golangci.yml`](../.golangci.yml) enables `errcheck, govet, staticcheck, unused, ineffassign, gocritic, gosec` — no `depguard`, no `importas`. Four items above (INV-01, INV-03, PROP-08, UNR-01) are `depguard` rules, so adding it once unlocks all four.

The only architecture-shaped tests that exist today are the env-var drift checks (`TestEnvRegistryDrift`, `TestContractDrift`, `TestCIImageMatrixMatchesContract`, run at the `envcheck` job) — a working precedent for the architecture-test mechanism proposed for INV-06 and PROP-05.

**Suggested first three, by value over effort:** INV-06 (architecture test, lands green immediately, protects the invariant that fails hardest in production) · INV-03 (one `depguard` block, also currently clean) · removing `--issues-exit-code=0` (unblocks everything else).
