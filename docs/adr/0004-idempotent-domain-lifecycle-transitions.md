# ADR 0004 — Idempotent, set-based domain lifecycle transitions

- **Status:** Accepted
- **Date:** 2026-07-16
- **Deciders:** Platform / Registry
- **Supersedes:** none

## Context

The domain lifecycle (expiry → auto-renew or pendingDelete → redemption →
purge, plus restore completion) is driven by three Temporal workflows on the
`lifecycle` queue, created and reconciled at worker startup by
`internal/infrastructure/bootstrap/ensure.go`:

| Schedule | Workflow | Interval |
|----------|----------|----------|
| `expiry-loop` | `ExpiryLoop` | hourly |
| `purge-loop` | `PurgeLoop` | hourly, +30 min offset |
| `restore-loop` | `RestoreWorkflow` | every 4 hours |

An investigation of this system (July 2026) confirmed the architecture —
**scheduled sweeps over indexed DB state, with Temporal as scheduler, retry
harness, and observability surface** — but found five defects in the
implementation:

1. **Non-idempotent batch writes under Temporal retries.** The batch
   activities run with `MaximumAttempts: 3` and 30-minute timeouts, and their
   heartbeats do not checkpoint progress. A retry after a partially completed
   attempt re-processed the whole name list. `Domain.Renew()` never checks
   that a domain is actually expired, so already-renewed domains received a
   second `ExpiryDate += 1 year` **and a second `domain.auto_renewed` event
   with a Quote attached** — a billing-integrity bug. Restore had the same
   exposure via `ForceRenew`, and purge retries mislabelled already-purged
   domains as failures.
2. **Purge activities dropped their query.** `ListPurgeableDomains` and
   `GetPurgeableDomainCount` accepted a `PurgeableDomainsQuery` but never
   serialized it; the server defaulted the cutoff to its own `time.Now()` at
   two different instants. The workflow's "locked reference time" was a
   no-op for purge, `ReferenceTimeOverride` and dry-run previews silently did
   nothing, and the TOCTOU race the code claimed to eliminate persisted. A
   related latent bug: the postgres `ListPurgeableDomains` implementation
   declared `(clid, cursor, tld)` while the repository interface declared
   `(clid, tld, cursor)` — the TLD filter landed in the cursor argument and
   was never applied in SQL.
3. **Per-domain HTTP eligibility fan-out.** `CheckDomainsCanAutoRenew`
   issued up to 1,000 individual `GET /domains/{name}/canautorenew` requests
   at 20-way concurrency inside a single activity with a **1-minute**
   StartToCloseTimeout and no heartbeat. The same policy checks were then
   re-evaluated (with different failure semantics) inside
   `BatchAutoRenewDomains`.
4. **Continue-as-new hot loop.** Both loops re-ran with identical params when
   `listed < count`. Failed domains stay in the query set, so a batch of
   persistently failing domains (> batch size) spun the workflow in an
   unbounded zero-delay continue-as-new chain, while the schedule's
   `OVERLAP_SKIP` policy suppressed all subsequent scheduled runs.
5. **Dead configuration and misleading names.** `BatchSize` and
   `ConcurrencyLimit` params were documented but ignored;
   `PurgeableDomainsQuery.After` actually meant `purge_date <= X`;
   `RestoreWorkflow` processed only the first page every 4 hours and
   completed restores with two non-transactional writes plus manual
   compensation.

## Decision

Keep the sweep-based architecture. Alternatives considered and rejected:

- **One Temporal workflow per domain lifetime (durable timers).** Exact-time
  transitions, but at registry scale it means millions of long-lived
  workflows, painful code migrations for in-flight executions on every policy
  change, and a standing reconciliation problem between workflow timers and
  the authoritative `expiry_date`/`purge_date` columns the DB already
  indexes.
- **Plain cron / pg_cron.** Loses retries with backoff, dry-runs, progress
  queries, manual triggering, and run history that Temporal provides for
  free.

Within that architecture, make the following changes (implemented together
with this ADR):

1. **Every batch lifecycle write is a guarded, skip-on-converged transition.**
   `BatchResult` gains a `Skipped` list. Before writing, each operation
   checks whether the domain is already in (or past) the target state and
   skips it:
   - auto-renew skips domains whose `ExpiryDate` is in the future,
   - expire skips domains already `pendingDelete`,
   - purge skips domains that no longer exist,
   - restore-completion skips domains no longer `pendingRestore`.
   Re-running any batch is therefore a no-op for already-processed domains:
   **activity retries cannot double-renew, double-bill, or double-expire.**
   "Domain not found" is treated as convergence (skip), not failure, in all
   four operations.
2. **Restore completion is one atomic write.** `BatchRestoreDomains` applies
   `UnSetStatus(pendingRestore)` and `ForceRenew(1y)` to the entity in memory
   and persists with a single `UpdateDomain` call (quote fetched before any
   mutation). The half-restored state and its compensation logic are gone.
   One `domain.renewed` event (with quote) is published per completion.
3. **Eligibility is partitioned set-based in the worker.**
   `DomainService.PartitionExpiredDomains` resolves auto-renew vs. expire for
   a whole batch with one domain fetch, one TLD+phase lookup per TLD and one
   registrar lookup per ClID, exposed as the
   `BatchCheckAutoRenewEligibility` activity (chunked, heartbeating,
   30-minute timeout). The per-domain HTTP activities
   (`CheckDomainCanAutoRenew`, `CheckDomainsCanAutoRenew`) are deprecated but
   stay registered so in-flight executions on the drain queues can complete.
4. **Continue-as-new is guarded.** A run only continues when it made progress
   (`succeeded + skipped > 0`) and the chain is capped at
   `maxContinuationRuns` (50). Otherwise the run completes with an
   explanatory note and the next scheduled run retries — a poison batch can
   no longer starve the schedule.
5. **Queries are serialized end-to-end and named for what they do.** The
   purge activities now send `before` (cutoff), `clid`, `tld` and `pagesize`;
   `PurgeableDomainsQuery.After` is renamed `Before`; the REST endpoints
   accept `before` and keep `after` as a deprecated alias; the postgres
   parameter-order bug is fixed and the TLD filter applied. `BatchSize` is
   wired through all three workflows (`PageSize` on the query structs);
   `ConcurrencyLimit` is removed. `RestoreWorkflow` takes optional params,
   paginates via guarded continue-as-new, and remains compatible with
   argument-less starts (Temporal pads missing inputs with zero values).

## Consequences

**Positive**

- Billing integrity: renewal/billing events are emitted at most once per
  domain transition, regardless of worker crashes, timeouts, or retries.
- Purge evaluates the workflow's locked reference time; dry-runs and
  `ReferenceTimeOverride` now mean what they say.
- Eligibility checking drops from O(N) HTTP requests to ~3 DB round-trips per
  200-domain chunk, inside a properly heartbeated 30-minute activity.
- Poison batches degrade gracefully (visible failure notes, hourly retries)
  instead of hot-looping and blocking scheduled runs.
- `Skipped` counters make retry no-ops observable in run results.

**Negative / accepted trade-offs**

- The deploy that ships this change breaks replay determinism for lifecycle
  workflow runs in flight at rollout (the loops are short-lived and hourly;
  an interrupted run is retried by the next schedule fire — acceptable).
- Eligibility semantics live in the worker's service layer; the
  `/domains/{name}/canautorenew` endpoint remains for ad-hoc use but is no
  longer on the lifecycle hot path.
- Failed (non-skippable) domains are retried every scheduled run
  indefinitely. A dead-letter mechanism (attempt counter + "needs attention"
  flag + alert) is the natural next step and is out of scope here, as is
  emitting Prometheus metrics per run.

**Deployment note**

- The renamed repository parameters and the REST `before` alias are
  backwards-compatible; external callers using `after` keep working.
- Existing Temporal schedules need no migration: `restore-loop` keeps
  starting the workflow without args (zero-value params apply), and the
  schedule reconciler updates nothing because intervals/queues are unchanged.
