# Architectural invariants — DRAFT for review

**Analysed at:** `0dccd6d59414984245a36eb3d9f2a77b440ca164`
**Date:** 2026-08-10
**Status:** Draft. Nothing here is settled. Expected review action is deletion and correction.

**Resolutions since the analysis SHA.** `INV-02` → Invariants, Class B, by [ADR-0006](adr/0006-tenancy-model.md) (2026-08-25). All ten `PROP` candidates are now answered (2026-08-10 and 2026-08-25): six promoted to `INV-07`…`INV-13`, four retired without promotion — see the disposition table. Evidence for anything resolved after the analysis SHA is cited at HEAD rather than at that SHA.

This document exists so architectural review can be a link to a section ID instead of an essay.

## How to read this

Every claim carries `file:line` evidence at the SHA above. Nothing is asserted without it.

| Class | Meaning |
|---|---|
| **A — Enforced** | Holds consistently everywhere it applies. |
| **B — Intended but leaky** | Holds in most places; every violation is cited individually. |
| **C — Observed pattern** | A convention exists; whether it is deliberate is unknown. Stated as a question. |
| **D — Contradiction** | Two parts of the repo do incompatible things. No rule proposed. |

**ID rules.** `INV-nn` are the confirmed invariants and are the only IDs citable as rules. `UNR-nn` are unresolved contradictions — no rule is proposed for them. `PROP-nn` was the review-time proposal space; **it is now fully retired**, and no `PROP` ID is live or citable. IDs are stable and are never reused after retirement.

## Index

| ID | Rule | Class | Section |
|---|---|---|---|
| INV-01 | Outbox is the path of record; telemetry is never load-bearing | A | Invariants |
| INV-02 | Two-sided tenancy: operator scope by TLD join, registrar scope by sponsorship | **B** | Invariants |
| INV-03 | No vendor LLM SDK outside `ModelProvider` | A | Invariants |
| INV-04 | Evidence provenance mandatory on terminal agent outcomes | **B** | Invariants |
| INV-05 | Reconciliation before record creation | **B** (process only) | Invariants |
| INV-06 | Determinism boundaries around workflows | A | Invariants |
| INV-07 | Errors are wrapped with `%w`, not `%v` | **B** | Invariants |
| INV-08 | Business validation lives in entity constructors | A | Invariants |
| INV-09 | Domain declares sentinel errors; outer layers wrap | A | Invariants |
| INV-10 | Repository methods take `ctx` first | **B** | Invariants |
| INV-11 | List endpoints paginate by cursor, never offset | A | Invariants |
| INV-12 | Temporal code logs through the Temporal logger | A | Invariants |
| INV-13 | `commands/` write inputs, `queries/` read filters | A | Invariants |
| ~~PROP-01…10~~ | All retired — six promoted, four dropped | — | Disposition table |
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

## INV-02 — Tenancy is two-sided: operator scope by TLD join, registrar scope by sponsorship

*Resolved 2026-08-25. Was Class D (Unresolved). The rule below is the decision recorded in [`docs/adr/0006-tenancy-model.md`](adr/0006-tenancy-model.md) — read that for the reasoning, the guardrails, and what is deferred.*

**Rule.** This registry is a two-sided marketplace, so there are two tenant kinds and each has its own key:

- **Operator scope** — a `RegistryOperator.RyID`. Administrative plane. Reaches TLDs and everything inside them *through the TLD join*, never through a denormalized column.
- **Registrar scope** — a `Registrar.ClID`. Transactional plane (EPP, future registrar portal). Reaches **only objects the registrar sponsors**; accreditation gates which TLDs it may transact in.
- **Staff/global is an explicit third kind**, never the absence of a filter.

Scope is a claim on the authenticated principal, is passed as a typed parameter immediately after `ctx`, and is enforced in repository SQL — `WHERE tld.ry_id = ?` on the operator side, `WHERE clid = ?` on the registrar side. Every EPP transactional verb checks sponsorship (info/update/delete/transfer → EPP `2201` on mismatch) or accreditation (create) in the command layer.

**Why.** The original formulation asked for "tenant scope on every executor call" and could not be satisfied, because "tenant" had never been defined here. Defining it dissolves the problem that made this a contradiction: contacts and hosts have no TLD and so cannot carry operator scope, but they are *sponsored*, and sponsorship is already in the schema — `ClID` on [`domain.go:59`](../pkg/domain/entities/domain.go#L59), [`contact.go:66`](../pkg/domain/entities/contact.go#L66), [`host.go:42`](../pkg/domain/entities/host.go#L42). The consumer side needs no schema change; the supply side needs no denormalization. Getting this wrong in the other direction is expensive and quiet: a `ry_id` column on `contacts` would work until the first TLD reassignment.

**Class: B — the rule holds wherever tenancy is implemented today; the surfaces it does not yet reach are enumerated below, each with a stated reason.**

**Evidence.**

Scope types: [`pkg/domain/entities/scope.go:38`](../pkg/domain/entities/scope.go#L38) (`OperatorID`) and [`:80`](../pkg/domain/entities/scope.go#L80) (`RegistrarClID`) — distinct defined types over `ClIDType`, not aliases, so the two scope kinds and a plain object ClID cannot be interchanged.

Single admin-plane derivation point: [`internal/interface/rest/tenant.go:39`](../internal/interface/rest/tenant.go#L39). It is the only reader of `X-Tenant-ID` in `internal/` — the six per-handler `getTenantID` calls, and the `?tenant_id=` query fallback that made scope forgeable from an address bar, are gone.

Operator scope threaded and enforced, zone-slaving slice: interface [`zone_slaving_interface.go:27`](../internal/application/interfaces/zone_slaving_interface.go#L27) (all seven methods `(ctx, scope entities.OperatorID, ...)`), port [`serial_drift_repository.go:15`](../pkg/domain/repositories/serial_drift_repository.go#L15), SQL enforcement at [`internal/infrastructure/db/postgres/serial_drift_repository.go:43`](../internal/infrastructure/db/postgres/serial_drift_repository.go#L43) and 12 further sites — 13 `tenant_id = ?` predicates in that file, one for every scoped query — with the uniqueness index at [`zone_slaving.go:14`](../internal/infrastructure/db/postgres/zone_slaving.go#L14).

The consumer-side chain is already in the schema and needs nothing: sponsorship columns above, accreditation as the bridge at [`accreditation_service.go:40`](../internal/application/services/accreditation_service.go#L40), and both filter hooks already present at [`listDomainsFilter.go:16,18`](../pkg/domain/queries/listDomainsFilter.go#L16) (`TldEquals`, `ClidEquals`).

**Where the rule does not yet reach, individually:**

1. **The EPP transactional surface does not exist, so the rule is pre-established rather than held.** The server authenticates per registrar — `registrarIDKey` stamped at [`cmd/epp/eppServer.go:319`](../cmd/epp/eppServer.go#L319) with a typed context key ([`:28`](../cmd/epp/eppServer.go#L28)) — but binds only greeting, login, logout and domain-check ([`:100-118`](../cmd/epp/eppServer.go#L100); contact-info commented out), and that identity currently feeds rate limiting only. This is the favourable case, not the unfavourable one: the invariant is being written before the surface it governs. To be put on the "Architectural invariants" board as a follow-on issue, so the EPP buildout works from it rather than from a document alone.
2. **The core registry services are intentionally global.** Not one method across the 25 interfaces in `internal/application/interfaces/` takes a scope — e.g. [`domain_interface.go:14-57`](../internal/application/interfaces/domain_interface.go#L14), [`registrar_interface.go:13-22`](../internal/application/interfaces/registrar_interface.go#L13). Under a single operator this is deliberate deferral, recorded in ADR-0006, not drift. (`ClID` in those signatures is the *operand's* key — which record is being acted on — not a caller scope.)
3. **The workflow launchpad still takes operator scope from the request body.** [`workflow_controller.go`](../internal/interface/rest/workflow_controller.go), `serial-drift` case: it is now typed and validated, so an unusable scope is rejected before a schedule is created against it, but it is self-asserted rather than claimed. Closes with the Auth0 tenant claim.
4. **Context-derived identifiers remain audit-only.** `trace_id` / `correlation_id` / `userid` are stamped onto events in all six producer services (e.g. [`domain_service.go:1761,1764,1780`](../internal/application/services/domain_service.go#L1761)) and no query is filtered by them. That is correct under this rule — scope is a parameter, not context state — and is listed so it is not mistaken for tenancy.
5. **Writes carry scope inside the entity, not as a parameter.** `CreateSlaving`, `CreateRun` and `CreateObservations` on the port take no `scope` argument — the value is already a field on the record being written, and a second copy in the signature would be two sources of truth for one column. Reads and status changes, where the scope is a filter rather than a value, all take it explicitly. Deliberate, and stated here so it is not read as an omission.
6. **`askg`'s `CallerScope` is not a tenant.** [`internal/askg/result.go:62`](../internal/askg/result.go#L62) is `CallerScope{ UserID string }`, threaded into no tool execution; the `Executor` at [`toolexec.go:9`](../internal/askg/toolexec.go#L9) that the original wording named is an LLM tool executor, not a tenancy boundary. The agent eval suite asserts tenant isolation anyway — `CategoryTenantIsolation` ([`eval/eval.go:36`](../internal/askg/eval/eval.go#L36)), `ScoreTenantIsolation` ([`eval/scoring.go:24`](../internal/askg/eval/scoring.go#L24), which greps answer text), `TestOrchestrator_TenantScopeThreading` ([`orchestrator_test.go:291`](../internal/askg/orchestrator_test.go#L291)) — so those tests describe an intent that layer does not implement. Out of scope for ADR-0006.

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

## INV-08 — Business validation lives in entity constructors, not at the edge

**Rule.** Business rules are enforced by the entity constructor or its `Validate()` method. Handlers do not re-implement them.

**Why.** One definition of "valid" means one place to change it and one place to test it. When the edge also enforces business rules, the two copies drift, and the domain's guarantee quietly becomes "valid *if* it arrived through the handler that checks".

**Class: A.**

**Evidence.** 33 `New<X>(...) (*X, error)` constructors and 26 `Validate()`/`IsValid()` methods across 75 files in `pkg/domain/entities/` — e.g. [`registrar.go:124`](../pkg/domain/entities/registrar.go#L124), [`contact.go:147`](../pkg/domain/entities/contact.go#L147), [`tld.go:65`](../pkg/domain/entities/tld.go#L65). Against that, the entire REST layer carries only 18 `binding:"..."` tags. The edge trusts the domain rather than duplicating it.

**Explicit carve-out — not a violation.** Gin `binding` tags for *presence and shape* (required field, parseable type) are permitted and expected. Rejecting a malformed request before it reaches the domain is a distinct concern from enforcing a business rule. This invariant governs business rules only; the 18 existing tags are not counted against it.

---

## INV-09 — The domain declares sentinel errors; outer layers wrap rather than declare

**Rule.** `Err… = errors.New(...)` sentinels are domain vocabulary and live in `pkg/domain/entities`. Outer layers wrap them; they do not mint their own parallel set.

**Why.** A sentinel is only useful if the caller can match it, and callers sit above the domain. Defining a second sentinel for the same condition in a service gives two ways to spell one failure, and the outer one is invisible to anyone holding the domain's. Together with [INV-07](#inv-07--errors-are-wrapped-with-w-not-v) this is one mechanism, not two: the domain names the failure, `%w` carries the name upward, and `errors.Is` works end to end. Documenting either half alone leaves the other unexplained.

**Class: A.**

**Evidence.** 165 sentinel declarations in non-test code, 37 files of them in `pkg/domain/entities`. Outer layers declare almost none: `internal/application/services` 4, `internal/interface/rest` 2, `pkg/domain/queries` 2, `internal/application/commands` 2, and one apiece in four `internal/infrastructure` packages.

---

## INV-10 — Repository methods take `ctx` first

**Rule.** Every method on a port in `pkg/domain/repositories/` takes `ctx context.Context` as its first parameter.

**Why.** A repository call is the point where work leaves the process — for the database or the network. Without a context the caller cannot cancel it, time it out, or carry a deadline into it, so one slow dependency becomes an unbounded wait with nothing able to interrupt it.

**Class: B** — 121 of 124 methods hold; one genuine violation, two legitimate exceptions.

**Violation, cited individually:**

- [`pkg/domain/repositories/iana_interface.go:8`](../pkg/domain/repositories/iana_interface.go#L8) — `ListRegistrars() ([]*entities.IANARegistrar, error)`. The implementation performs an HTTP fetch of the IANA XML registry: [`internal/infrastructure/web/ianaregistrars/iana_repository.go:33`](../internal/infrastructure/web/ianaregistrars/iana_repository.go#L33) calls `client.Get(repo.XMLRegistrarURL)`. With no context on the signature, no caller can cancel or bound it. This is exactly the case the rule exists for.

**Explicit exceptions — not violations.** [`idGenerator_interface.go:5`](../pkg/domain/repositories/idGenerator_interface.go#L5) and [`:6`](../pkg/domain/repositories/idGenerator_interface.go#L6) — `GenerateID()` and `ListNode()` are pure in-memory snowflake operations ([`internal/infrastructure/snowflakeidgenerator/snowflake_idgenerator.go:20`](../internal/infrastructure/snowflakeidgenerator/snowflake_idgenerator.go#L20)). They perform no IO, cannot block, and threading a context through them would be churn with no benefit. They are allowlisted deliberately.

---

## INV-11 — List endpoints paginate by cursor, never by offset

**Rule.** Pagination is expressed as `pageSize` plus an opaque `pageCursor`. No port or repository takes an offset.

**Why.** Offset pagination re-scans every skipped row, so cost grows with depth — and it silently skips or repeats rows when the underlying set changes between pages. A registry accumulates large, actively-mutating tables, which is the case where both failures bite hardest.

**Class: A** — holds universally; no violations found.

**Evidence.** The shared shape is `ListItemsQuery{PageSize int, PageCursor string, Filter ListItemsFilter}` at [`pkg/domain/queries/listItemsInterface.go:10`](../pkg/domain/queries/listItemsInterface.go#L10), consumed by 11 ports in `pkg/domain/repositories/`. Three further methods take the pair directly: [`ianaRegistrar_interface.go:12`](../pkg/domain/repositories/ianaRegistrar_interface.go#L12), [`phase_repository_interface.go:16`](../pkg/domain/repositories/phase_repository_interface.go#L16), [`:17`](../pkg/domain/repositories/phase_repository_interface.go#L17). A search for `Offset(` and `OFFSET` across `internal/infrastructure/db/postgres/` returns **zero** matches in non-test code.

*Note:* `ListItemsQuery` now lives at [`pkg/domain/queries`](../pkg/domain/queries) following [#399](https://github.com/onasunnymorning/domain-os/issues/399). The rule is about the shape, not the package, and was unaffected by that move.

---

## INV-12 — Temporal workflows and activities log through the Temporal logger

**Rule.** Code under `internal/application/workflows/` and `internal/application/activities/` logs via `workflow.GetLogger(ctx)` / `activity.GetLogger(ctx)`, not via the standard library or a package-global logger.

**Why.** The Temporal logger is replay-aware: it suppresses duplicate emission when a workflow re-executes its history, and it tags entries with workflow and run IDs. A `log.Printf` in workflow scope both loses that correlation and re-prints on every replay, turning one logical event into many identical lines during an incident.

**Class: A** — within the Temporal packages, which is the whole scope of this rule.

**Evidence.** `internal/application/workflows/`: 25 `workflow.GetLogger` calls, **0** `log.*`, **0** `slog.`. `internal/application/activities/`: 58 `activity.GetLogger` calls against 4 residual `log.*`.

**Scope limit — read this before citing it.** This invariant governs the Temporal packages *only*. The wider question — whether `slog` replaces the ~238 `log.*` calls elsewhere in the codebase, and what becomes of the injected `zap` loggers — is **not settled** and is not covered here. See the disposition table for `PROP-08`.

---

## INV-13 — `commands/` holds write inputs, `queries/` holds read filters

**Rule.** Input DTOs for state-changing operations live in `internal/application/commands/`; filter and read-shape types live in `pkg/domain/queries/`.

**Why.** The two kinds of type have different lifecycles — a command carries everything needed to validate and perform a write, a filter carries only what narrows a read — and keeping them in one package invites a type that quietly serves both and can no longer change for either.

**Class: A** — the split is consistently populated.

**This is a naming convention, not a CQRS commitment.** There is no separate read model, no read/write store split, and no event sourcing anywhere in this codebase. The package names resemble CQRS vocabulary; the architecture is not CQRS. Do not cite this invariant as authority for building a read model — that would be a new architectural decision needing its own ADR.

*Note:* the read filters now live at [`pkg/domain/queries`](../pkg/domain/queries) following [#399](https://github.com/onasunnymorning/domain-os/issues/399), while commands remain under `internal/application/commands/`. This rule is about the write/read separation, not the parent directory, and was unaffected by that move.

---

# Proposed — resolved 2026-08-25

**This section is closed.** All ten Class C candidates have been answered. Six were promoted to invariants, four were retired without promotion. **No `PROP-nn` ID is live**, none should be cited in review, and none will be reused.

| ID | Question asked | Outcome |
|---|---|---|
| `PROP-01` | Should transaction ownership sit at one named layer? | **Retired, not promoted.** No convention existed to confirm — 8 sites across 3 layers is an absence, not a pattern, and promoting one would have invented a rule rather than recorded one. The concrete finding it circled is `INV-01` defect 1 (the outbox write is not atomic with its business write), tracked in [#408](https://github.com/onasunnymorning/domain-os/issues/408). |
| `PROP-02` | Is validation the entity's job? | **Promoted → [INV-08](#inv-08--business-validation-lives-in-entity-constructors-not-at-the-edge)**, Class A. |
| `PROP-03` | Are sentinels domain vocabulary? | **Promoted → [INV-09](#inv-09--the-domain-declares-sentinel-errors-outer-layers-wrap-rather-than-declare)**, Class A. |
| `PROP-04` | Is `%w` the rule? | **Promoted → [INV-07](#inv-07--errors-are-wrapped-with-w-not-v)**, Class B (2026-08-10). |
| `PROP-05` | Is `ctx`-first a rule? | **Promoted → [INV-10](#inv-10--repository-methods-take-ctx-first)**, Class B — one real violation cited, two exceptions allowlisted. |
| `PROP-06` | Is cursor pagination a rule? | **Promoted → [INV-11](#inv-11--list-endpoints-paginate-by-cursor-never-by-offset)**, Class A. Evidence was stronger than first recorded: the original entry cited 3 methods; the real figure is a shared query type used by 11 ports and zero offset usage anywhere. |
| `PROP-07` | Keep the `preload…bool` idiom? | **Retired, not promoted.** Four sites naming four different relations is coincidence, not convention. Promoting it would have committed new code to a boolean-flag idiom that compounds badly as relations multiply. Dropped without replacement; revisit if it starts to hurt. |
| `PROP-08` | Is `slog` the direction of travel? | **Split.** The Temporal half was real and is promoted → [INV-12](#inv-12--temporal-workflows-and-activities-log-through-the-temporal-logger), Class A. The project-wide `slog`-vs-`log` question is a direction to choose, not a pattern to observe, and is deferred to [#409](https://github.com/onasunnymorning/domain-os/issues/409). |
| `PROP-09` | Is AutoMigrate-with-no-tool deliberate? | **Retired, not promoted.** This is a risk to decide on, not a convention to bless — writing it into the invariants would enshrine as intent something that cannot drop columns or indexes, cannot be reviewed as a diff, and has no down-path. Deferred to [#410](https://github.com/onasunnymorning/domain-os/issues/410), to settle before production traffic. |
| `PROP-10` | Is the `commands`/`queries` split a CQRS commitment? | **Promoted → [INV-13](#inv-13--commands-holds-write-inputs-queries-holds-read-filters)**, Class A — but explicitly as a naming convention, *not* as CQRS. |

---

# Unresolved

Contradictions. No rule proposed for any of these — they need a decision.

`INV-02` was here and has left: it is resolved by [ADR-0006](adr/0006-tenancy-model.md) and now sits in Invariants as Class B. The ID is unchanged.

## UNR-01 — The domain layer imports outward, while the docs say it cannot

[`architecture.md:11`](../architecture.md#L11) states the domain layer "is dependency-free (no imports of database drivers, HTTP frameworks, etc.)", and [`.cursorrules`](../.cursorrules) makes that file binding on every agent working in this repo. This started as 13 files in `pkg/domain/repositories/` importing inward-layer packages. They were two different problems with two different fixes, and only one was a real layering violation. **Group 1 (11 files) is now fixed; 2 remain.**

`pkg/domain/repositories/` is a genuine ports package: 26 interfaces, no adapters. (It does hold 4 mock implementations — [`MockDomainRepository`](../pkg/domain/repositories/domain_repository.go#L38), [`MockRegistrarRepository`](../pkg/domain/repositories/registrar_interface.go#L25), [`MockHostRepository`](../pkg/domain/repositories/host_repository.go#L28), [`MockHostAddressRepository`](../pkg/domain/repositories/hostAddress_repository.go#L17) — which is a separate, minor question about test doubles shipping in the port package.)

### ~~Group 1~~ — misfiled value types (11 files) — **RESOLVED 2026-08-25**

The 11 ports importing `internal/application/queries` were not an inverted dependency. `go list -deps` on that package returned exactly one first-party package — `pkg/domain/entities` — with no infrastructure, no GORM and no HTTP; its third-party deps were the same value-object libraries the entities already use. They were domain-shaped value types filed under `internal/application/`.

Fixed by moving the whole package to [`pkg/domain/queries`](../pkg/domain/queries) ([#399](https://github.com/onasunnymorning/domain-os/issues/399)). The package name is unchanged, so the change was an import-path rewrite across 103 files with **no signature changes and no logic changes**. Full suite green.

The move was safe as a whole-package move rather than a partial one because every type in it — the filters, `ListItemsQuery`, and value objects like `QuoteRequest` (which carries its own `Validate()`) — depends only on `pkg/domain/entities`.

### Group 2 — a real leak, and it is the expensive one (2 files, still open)

[`fx_interface.go:6`](../pkg/domain/repositories/fx_interface.go#L6) and [`tldDNSRecord_repository.go:6`](../pkg/domain/repositories/tldDNSRecord_repository.go#L6) import `internal/infrastructure/db/postgres` and type their method signatures against GORM persistence models.

- **`FXRepository` is inconsistent within itself.** [`UpdateAll(ctx, fxs []*postgres.FX)`](../pkg/domain/repositories/fx_interface.go#L13) takes the GORM model, while [`ListByBaseCurrency`](../pkg/domain/repositories/fx_interface.go#L14) returns `[]*entities.FX`. Writes speak Postgres, reads speak domain. And [`entities.FX` exists](../pkg/domain/entities/fx.go#L18) with real behaviour on it (a `Convert` method) — it is simply not used on the write path. [`postgres.FX`](../internal/infrastructure/db/postgres/fx.go#L10) is a different shape: `gorm:"primaryKey"` tags, `CreatedAt`/`UpdatedAt`, differently-named fields.
- **`TLDDNSRecordRepository` uses `postgres.TLDDNSRecord` in all three methods** ([`:11-13`](../pkg/domain/repositories/tldDNSRecord_repository.go#L11)) — and there is **no** `entities.TLDDNSRecord`. That concept was never modelled in the domain at all; the persistence model is its only representation.

**The concrete cost, measured:** `go list -deps` gives `pkg/domain/entities` **19** third-party transitive dependencies. `pkg/domain/repositories` has **76**, including `github.com/jackc/pgx/v5/pgconn`, `pgproto3`, and `lib/pq` — actual Postgres wire-protocol drivers, now reachable from the domain layer. **These two files alone cause that** — the Group 1 move did not shift the number, which is exactly what identifies Group 2 as the real leak. This is precisely the "no imports of database drivers" case `architecture.md:11` names.

**Fix is real work:** introduce `entities.TLDDNSRecord`, use `entities.FX` on the write path, and add mappers in the postgres adapter.

**What needs deciding:** whether Group 2 gets modelled properly, or the "dependency-free" claim in `architecture.md` is narrowed to `pkg/domain/entities` only — which is where it currently holds. [`fx_interface.go`](../pkg/domain/repositories/fx_interface.go) is being addressed in [#400](https://github.com/onasunnymorning/domain-os/issues/400); [`tldDNSRecord_repository.go`](../pkg/domain/repositories/tldDNSRecord_repository.go) needs the modelling decision in [#401](https://github.com/onasunnymorning/domain-os/issues/401) first. Once both land, the `depguard` rule in [#403](https://github.com/onasunnymorning/domain-os/issues/403) can be switched on without an allowlist and UNR-01 closes.

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
| INV-02 | partial | grep-level CI check + architecture test | S→L | Two cheap gates hold the ratchet today: assert the literal `"X-Tenant-ID"` appears exactly once in non-test `internal/` code — the `TenantIDHeader` const in the derivation point, and assert no `tenantID string` parameter survives in the zone-slaving slice. The full rule — every tenant-scoped service method takes a typed scope after `ctx`, every scoped query carries its predicate — needs a custom analyser and is only worth building once scope is threaded past the template slice. The EPP half is not checkable until the verbs exist. |
| INV-03 | **yes** | import lint (`depguard`) | **S** | Highest value per unit of effort in this table. Allow vendor LLM SDKs only under `internal/askg/provider/**`; deny everywhere else. One config block. |
| INV-04 | yes | type change + analyser | M | Strongest fix is structural, not a lint: unexport `Result`'s fields and add `NewAnswer(...)`/`NewEscalation(...)` constructors that reject empty `Evidence` for `OutcomeAnswer`. A `Validate()` called at [`agent_controller.go:91`](../internal/interface/rest/agent_controller.go#L91) is the cheap interim. |
| INV-05 | partial | grep-level CI check | S | Cannot verify reconciliation happened. **Can** assert `entity_count_consistency` has `Severity: "error"` — i.e. pin the gate so it cannot be silently downgraded again. Genuine enforcement needs branching on `VerificationPassed`, which is a code fix, not a gate. |
| INV-06 | **yes** | architecture test | **S** | Assert no file under `internal/application/workflows/` imports `net/http`, `math/rand`, `crypto/rand`, or a uuid package, and contains no `time.Now(`. A ~30-line Go test using `go/parser`. Currently holds at zero violations, so it lands green on day one. |
| INV-07 | **yes** | lint (`errorlint`, `errorf: true`) | **S** | Verified against this SHA: `errorlint` is the linter that catches this — not `err113`/`wrapcheck`, which answer different questions. 49 offenders across 9 files to clear first; 25 are in one file. Enabling `errorlint`'s other two checks (`asserts`, `comparison`) raises the count to 87 — a separate cleanup with different risk, since `==` → `errors.Is` can change behaviour where a sentinel is compared by identity. Tracked in [#404](https://github.com/onasunnymorning/domain-os/issues/404). |
| INV-08 | partial | architecture test | M | Can assert every entity type has a `New…` constructor returning `error`; cannot assert the validation inside it is meaningful, nor distinguish a business rule from a presence check at the edge. |
| INV-09 | partial | architecture test | M | Can assert no `Err… = errors.New` outside `pkg/domain/entities` (13 current outliers to fix or allowlist). Cannot assert an outer-layer error *should* have been a domain sentinel — that stays review. |
| INV-10 | **yes** | architecture test | **S** | Parse `pkg/domain/repositories/`, assert param 0 is `context.Context`. Allowlist the 2 `idGenerator` methods; the `iana_interface.go:8` violation should be fixed rather than allowlisted, since it is a live uncancellable HTTP call. |
| INV-11 | partial | grep-level CI check | S | Cheapest reliable form is a grep asserting no `Offset(`/`OFFSET` appears in `internal/infrastructure/db/postgres/` — currently zero, so it lands green. Asserting the positive (every list method takes a cursor) is signature-shape matching and brittle. |
| INV-12 | **yes** | import lint (`depguard`) | **S** | Deny `log` and `log/slog` from `internal/application/workflows/**` and `internal/application/activities/**`. Workflows are already clean; 4 residual `log.*` calls in activities to clear first. **Not blocked** — the project-wide logging direction is a separate, still-open question ([#409](https://github.com/onasunnymorning/domain-os/issues/409)) and does not gate this rule. |
| INV-13 | partial | architecture test | S | Can assert `commands/` and `queries/` contain no cross-imports and that no type is declared in both. Cannot assert a given type was filed in the right one. |
| ~~PROP-09~~ | — | — | — | Retired without promotion. Migration tooling is an open decision, not an invariant — [#410](https://github.com/onasunnymorning/domain-os/issues/410). |
| UNR-01 grp 1 (11 files) | **yes** | import lint (`depguard`) | **S** | Gate is one deny-rule: no `internal/**` from `pkg/domain/**`. Blocked until the query/filter types move to `pkg/domain` — that move is itself S and mechanical. |
| UNR-01 grp 2 (2 files) | yes | same deny-rule | M | Same gate catches it, but the fix is modelling work (`entities.TLDDNSRecord`, `entities.FX` on writes, mappers) — not a rename. Do this before turning the rule on, or allowlist the two files meanwhile. |
| UNR-02 | partial | `staticcheck` SA1029 | S | Already detected by a linter that is already enabled — see the blocker below. |
| UNR-03 | no | code review | — | Doc-vs-code drift is not mechanically checkable. |

**Two blockers that gate this entire table:**

1. **CI lint is non-blocking.** [`.github/workflows/ci.yaml`](../.github/workflows/ci.yaml) runs golangci-lint with `--issues-exit-code=0`. **Every lint-based mechanism above is inert until that flag is removed.** That single change is the prerequisite for roughly half this table.
2. **No import-restriction linter is configured.** [`.golangci.yml`](../.golangci.yml) enables `errcheck, govet, staticcheck, unused, ineffassign, gocritic, gosec` — no `depguard`, no `importas`. Four items above (INV-01, INV-03, INV-12, UNR-01) are `depguard` rules, so adding it once unlocks all four.

The only architecture-shaped tests that exist today are the env-var drift checks (`TestEnvRegistryDrift`, `TestContractDrift`, `TestCIImageMatrixMatchesContract`, run at the `envcheck` job) — a working precedent for the architecture-test mechanism proposed for INV-06, INV-10 and INV-13.

**Suggested first three, by value over effort:** INV-06 (architecture test, lands green immediately, protects the invariant that fails hardest in production) · INV-03 (one `depguard` block, also currently clean) · removing `--issues-exit-code=0` (unblocks everything else).
