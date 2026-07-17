# ADR 0003 — DNS zone production and publication

- **Status:** Proposed
- **Date:** 2026-07-16
- **Deciders:** Platform / DNS
- **Supersedes:** none

## Context

domain-os holds the authoritative registry state for its TLDs — domains,
hosts, DNSSEC delegation material, and EPP statuses — but has no component
that turns that state into a live DNS zone. Today the DNS-facing surface is
pull-based and partial:

- `TLDDNSService` (`internal/application/interfaces/tldDNS_interface.go`)
  exposes `GetNSRecordsPerTLD` and `GetGlueRecordsPerTLD`, served as REST
  endpoints on the TLD controller
  (`internal/interface/rest/tld_controller.go`). A caller can fetch
  delegations and glue as `dns.RR` values, but nothing assembles them into a
  zone.
- `entities.TLDHeader` (`pkg/domain/entities/tldHeader.go`) models the zone
  apex — SOA, NS, glue, DS, DNSKEY — with string rendering, but no producer
  or consumer of a complete zone exists.
- The **verification half of the loop is already built**:
  `CheckSerialDriftWorkflow`
  (`internal/application/workflows/serialDrift.go`) monitors SOA serial
  propagation across a master/slave nameserver fleet using RFC 1982
  arithmetic, and `ZoneSlavingService` manages the monitored configurations.
  We monitor propagation of zones we do not yet produce.

The missing piece is the **producing** component: generate the TLD zone from
registry state, get it signed, deliver it to the public authoritative fleet,
and confirm it propagated. For a registry this is the single most
availability-critical output — resolution of every registered domain depends
on it — and it is subject to well-known failure modes (publishing a truncated
or empty zone, serial regressions, signature expiry).

Relevant machinery already in the codebase that the design should reuse
rather than duplicate:

- **Temporal workflow discipline** — every long-running process is a
  documented workflow (`internal/application/workflows/*.doc.md`) with
  activities, retries, and Launchpad cards.
- **S3/MinIO streaming** — `takeSnapshot` streams multi-GB exports via
  `io.Pipe()` to `UploadStream()`; the same pattern suits zone files, which
  for a large TLD run to millions of records.
- **Domain event stream** — `eventRelay` drains domain events to S3 in
  batches; a future incremental-update path can consume the same events.
- **EPP status semantics** — `docs/domain_status_overview.md` defines which
  statuses (e.g. `serverHold`, `clientHold`, `inactive`) exclude a domain
  from the zone.
- **DNSSEC material management** — `dnssec_service.go` manages DS/DNSKEY
  data at the registry level.

### Constraints

- **The public DNS must not depend on domain-os availability.** Nameservers
  keep answering during app deploys, database failovers, and outages.
  Registries are audited on this property.
- **Never publish a broken zone.** A validation gate must be able to block
  publication; a bad publish must be reversible in minutes, not hours.
- Must fit the hexagonal rules in `architecture.md`: ports in the domain
  layer, adapters in infrastructure, orchestration as a documented Temporal
  workflow.
- **Serials are sacred**: monotonic per zone (RFC 1982), allocated
  transactionally, never reused — `serialDrift` verification depends on it.
- Memory-bounded: zone generation must stream (cursor reads, piped writes),
  never materialize a full zone in memory.
- DNSSEC is required for gTLD operation, but building and operating a signer
  and key-rollover machinery in-app is out of scope for the first iteration.

## Decision

Build a **Temporal-orchestrated zone publication pipeline** that produces
versioned, validated zone artifacts from registry state and hands them to a
**hidden primary** nameserver (Knot DNS or BIND), which performs DNSSEC
signing and distributes to the public fleet via AXFR/IXFR + NOTIFY.
domain-os produces artifacts; it is never in the DNS query path.

```
              ┌────────────── Temporal: GenerateZoneWorkflow (per TLD) ──────────────┐
              │                                                                      │
 Postgres ──▶ 1. Build           2. Validate          3. Publish        4. Verify    │
 (domains,      stream apex +      kzonecheck +         S3 artifact +     NOTIFY,    │
  hosts, DS,    delegations →      delta guardrails     load on hidden    serialDrift│
  statuses)     zone artifact                           primary           converges  │
              └──────────────────────────────────────────────────────────────────────┘
                                                            │
                                     Hidden primary (Knot/BIND, inline DNSSEC signing)
                                                            │  AXFR/IXFR + NOTIFY
                                     Public authoritative fleet / anycast provider
```

### 1. Zone Builder — full regeneration, streaming

A `GenerateZoneWorkflow` (queue `data-pipeline`, documented per the
workflow template) runs per TLD on a schedule (5–15 min) or on demand:

- Streams eligible delegations with cursor-based reads: NS records per
  domain, glue (A/AAAA) for in-bailiwick hosts, DS records for secure
  delegations. Eligibility is derived from EPP statuses — domains with
  `serverHold`/`clientHold` or no linked hosts are excluded.
- Prepends the apex from `entities.TLDHeader`.
- Writes a canonical, deterministically ordered zone file streamed to
  S3/MinIO (reusing the `UploadStream` pattern), alongside a manifest
  (record counts, checksum, serial, generation parameters).
- Allocates the serial from a DB-backed monotonic counter in the same
  transaction that records the `zone_versions` row.

Full regeneration is the only write path initially: it is deterministic,
idempotent, trivially auditable, and rollback-free. Incremental updates are
an explicit later addition (see Option C), never a replacement — periodic
full regeneration remains the reconciliation backstop, which is the standard
gTLD registry pattern.

### 2. Validation gate

No artifact is published without passing, as a distinct workflow step:

- **Syntactic**: `kzonecheck`/`named-checkzone` against the generated file.
- **Structural**: apex completeness (SOA + NS present), glue consistency
  (every in-bailiwick NS has glue; no orphan glue).
- **Delta guardrails**: compared to the previous published version, refuse
  to publish if the zone shrank by more than a configurable percentage or
  if delegation count dropped below an absolute floor. Guardrail failures
  park the workflow pending a human signal (the existing HITL
  signal-to-confirm pattern), because a legitimate mass-deletion must be
  approvable.

This step exists to make "empty zone published to the world" structurally
impossible rather than procedurally unlikely.

### 3. Versioned artifacts, hidden-primary publication

- Every generated zone is an **immutable S3 object** keyed by
  `{tld}/{serial}`; `zone_versions` rows record lineage. Rollback =
  re-publish the previous version with a fresh (higher) serial.
- Publication delivers the artifact to the hidden primary and reloads
  (`knotc zone-reload` / `rndc`). The hidden primary is the DNSSEC signing
  point (Knot inline signing or BIND `dnssec-policy`): key generation,
  rollovers, and re-signing are the DNS software's problem, which is mature
  at exactly this job. domain-os keeps *managing* DS/DNSKEY registry data
  (`dnssec_service`) and holds the signed-zone DNSKEY for the apex; it does
  not sign.
- The domain layer gains a `ZonePublisher` port; the hidden-primary adapter
  lives in infrastructure. Swapping publication mechanics (file drop +
  reload today; serving AXFR directly via `miekg/dns`, or a managed-DNS API
  push, later) changes only the adapter.

### 4. Verification closes the loop

After the hidden primary loads the new serial, the workflow's final step
confirms propagation using the existing `serialDrift` machinery: a
publication is `converged` only when the public fleet serves the new serial
within the SOA-derived grace window. A `ZoneSlaving` config per production
zone becomes mandatory, created alongside the zone's first publication.

### 5. Placement (hexagonal)

| Piece | Location |
|-------|----------|
| `Zone`, `ZoneVersion` entities; serial arithmetic | `pkg/domain/entities` |
| `ZoneRepository`, `ZonePublisher`, delegation-stream port | `pkg/domain/repositories` |
| `GenerateZoneWorkflow` + activities + `.doc.md` | `internal/application/workflows`, `activities` |
| `ZoneGenerationService` (trigger, list versions, rollback) | `internal/application/services` |
| Postgres delegation cursor, S3 artifact store, hidden-primary adapter | `internal/infrastructure` |
| Trigger/inspect/rollback endpoints; Launchpad card | `internal/interface/rest` |

## Options Considered

### Option A: Keep pull-based REST; external tooling assembles the zone

| Dimension | Assessment |
|-----------|------------|
| Complexity | None in-repo; pushed to unversioned external scripts |
| Safety | Poor — no validation gate, no serial discipline, no audit trail |
| Availability coupling | Poor — zone freshness depends on an external cron nobody owns |
| Stack fit | n/a |

**Pros:** Zero work; endpoints already exist.
**Cons:** The most critical registry output lives outside the codebase, untested and unaudited; violates the "never publish a broken zone" constraint by having no gate at all.

### Option B: Full-regeneration pipeline → hidden primary (signing delegated) ⭐

| Dimension | Assessment |
|-----------|------------|
| Complexity | Medium — one workflow, one builder, adapters; no DNS serving code |
| Safety | Strong — validation gate, immutable versions, instant rollback, HITL guardrails |
| Propagation latency | Minutes (regeneration interval + transfer) |
| Stack fit | High — reuses Temporal, S3 streaming, serialDrift, HITL signal patterns |

**Pros:** Deterministic and idempotent; auditable (every published zone is an immutable artifact); app stays out of the query path; DNSSEC handled by software built for it; standard registry pattern.
**Cons:** Propagation latency floor of minutes; hidden primary is new infrastructure to operate; full regeneration cost grows with zone size (mitigated by streaming; a multi-million-record TLD regenerates in low minutes).

### Option C: Event-driven incremental updates (RFC 2136 dynamic update / IXFR from the event stream)

| Dimension | Assessment |
|-----------|------------|
| Complexity | High — ordering, exactly-once application, drift repair, reconciliation |
| Safety | Weaker — no natural whole-zone validation point; drift accumulates silently |
| Propagation latency | Seconds |
| Stack fit | Medium — event stream exists (`eventRelay`), but consumers/ordering do not |

**Pros:** Seconds-level propagation; near-zero marginal cost per change.
**Cons:** A dropped or reordered update silently diverges the zone from registry state; needs a full-regeneration backstop anyway, so it is an *addition to* Option B, not an alternative.

### Option D: domain-os serves DNS directly (in-path authoritative via `miekg/dns`)

| Dimension | Assessment |
|-----------|------------|
| Complexity | High — DNS server correctness, DNSSEC online signing, DoS surface |
| Safety | Poor — app availability becomes DNS availability |
| Propagation latency | Immediate |
| Stack fit | Low — nothing in the codebase serves network protocols at this criticality |

**Pros:** No zone materialization; always-fresh answers.
**Cons:** Directly violates the decoupling constraint; a deploy or DB failover becomes a resolution outage for every registered domain; in-app DNSSEC signing is a large, audit-sensitive surface.

### Option E: Managed DNS provider API sync (push records to e.g. Route 53/NS1) — rejected as primary

Pushing per-record diffs to a provider API inherits Option C's drift problems
plus provider rate limits and lock-in, and most providers cannot host a
signed TLD zone under registry policy requirements. Viable later as an
additional `ZonePublisher` adapter for secondary distribution, not as the
production mechanism.

## Trade-off Analysis

Weighted for the stated priorities (safety/rollback 35%, availability
decoupling 25%, stack fit 15%, complexity 15%, propagation latency 10%;
scores 1–5):

| Option | Safety | Decoupling | Stack fit | Complexity | Latency | **Weighted** |
|--------|--------|------------|-----------|------------|---------|--------------|
| A. Pull-only status quo | 1 | 2 | 3 | 5 | 1 | 2.2 |
| **B. Full regen → hidden primary** | 5 | 5 | 5 | 4 | 3 | **4.6** |
| C. Incremental event-driven | 3 | 5 | 3 | 2 | 5 | 3.6 |
| D. In-path serving | 2 | 1 | 2 | 2 | 5 | 2.1 |
| E. Provider API sync | 3 | 4 | 3 | 3 | 4 | 3.4 |

B wins decisively on the criteria that matter for a registry. C is the only
option whose extra capability (seconds-level propagation) is foreseeably
needed; the decision is therefore **B with a designed extension path to
B+C**, not B alone.

### Extension path B → B+C

1. **The event stream already exists.** `eventRelay` publishes ordered
   domain events; an incremental applier consumes host/delegation/DS change
   events and applies them to the hidden primary as dynamic updates,
   allocating serials from the same DB counter.
2. **Full regeneration becomes the reconciler.** The scheduled
   `GenerateZoneWorkflow` keeps running; its validation step gains a
   drift-check comparing the incrementally maintained zone against the
   freshly generated artifact, repairing divergence by republishing.
3. **Ports don't change.** The applier is a second implementation behind
   `ZonePublisher`; workflow, entities, and audit schema are untouched.

**Trigger for the extension:** a contractual or product requirement for
sub-minute propagation (e.g. rapid-update SLAs). Until then, incremental
machinery is pure operating risk.

## Consequences

**Easier:**

- Registry state becomes *resolvable* — the core product output finally
  exists, with every published zone traceable to an immutable artifact,
  a serial, and a workflow run.
- Rollback of a bad publish is a one-action republish of the previous
  version.
- The existing `serialDrift`/`ZoneSlaving` investment becomes the automatic
  acceptance test for every publication.
- DNSSEC key rollovers are operated with standard DNS tooling and runbooks,
  not custom code.

**Harder:**

- A hidden primary (Knot/BIND) plus a public fleet or anycast secondary
  contract is new infrastructure to provision, monitor, and patch.
- Zone eligibility rules (status combinations, `inactive` semantics,
  IDN/edge-case host names) must be specified exactly — they are now
  externally visible behavior, not internal bookkeeping.
- The generation schedule creates a propagation-latency floor that support
  and registrars must understand ("changes are live within N minutes").
- The delta guardrail's HITL gate means a legitimate mass deletion needs a
  human approval step — this is intentional friction.

**To revisit:**

- Incremental path (B → B+C) when a sub-minute propagation requirement
  lands.
- Whether the hidden primary should be replaced by domain-os serving
  AXFR/IXFR directly (`miekg/dns` is vendored) once the pipeline is proven —
  removes a box from the diagram but moves signing in-app.
- Multi-provider secondary distribution (Option E as an additional
  `ZonePublisher` adapter) for fleet diversity.
- Per-tenant zones: the schema and S3 keying should assume multiple TLDs
  across tenants from day one, even if the first deployment runs one.

## Action Items

1. [ ] Define `Zone`/`ZoneVersion` entities and the DB-backed monotonic serial counter (RFC 1982) in the domain layer.
2. [ ] Add `ZoneRepository`, `ZonePublisher`, and a cursor-based delegation-stream port to `pkg/domain/repositories`.
3. [ ] Specify zone-eligibility rules from EPP statuses (extend `docs/domain_status_overview.md`) and encode them in the delegation query.
4. [ ] Implement the streaming zone builder activity (Postgres cursor → canonical zone file → S3 `UploadStream`, manifest with checksum/counts).
5. [ ] Implement the validation activity: `kzonecheck`, apex/glue structural checks, delta guardrails with HITL signal on breach.
6. [ ] Implement the hidden-primary publisher adapter (artifact fetch + `knotc` reload) and stand up Knot with inline signing in `deploy/`.
7. [ ] Write `GenerateZoneWorkflow` + `.doc.md` per the workflow template; register it in the workflow registry with a Launchpad card.
8. [ ] Wire the post-publish verification step to `serialDrift`; auto-create a `ZoneSlaving` config on first publication of a zone.
9. [ ] REST endpoints: trigger generation, list versions, inspect manifests, rollback to version.
10. [ ] Runbook: bad-publish rollback, guardrail-breach approval, hidden-primary failure/rebuild.
