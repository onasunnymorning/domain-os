# ADR 0003 — DNS zone production and publication

- **Status:** Proposed
- **Date:** 2026-07-16; revised 2026-11-02
- **Deciders:** Platform / DNS
- **Supersedes:** none
- **Composes with:** [ADR 0013](0013-epp-server-architecture.md) and [ADR 0020](0020-rdap-server-architecture.md) (the separate-process shape the zone-transfer service copies), [ADR 0015](0015-rst-conformance-test-suites.md) (how its conformance tests are built), [ADR 0018](0018-where-configuration-lives.md) (where its settings go), INV-01, INV-05

**Revision 2026-11-02.** The July draft settled how a zone is *produced* but left the *seam* to downstream DNS as "file drop plus `knotc` reload". This revision decides that seam:
- the zone reaches downstream DNS over standard zone transfer, from a separate domain-os service;
- publication targets are data, owned by DNS engineers;
- the file drop becomes the first step of a migration path, not the end state.

It also records what the code does today and the prerequisites the July draft did not name, and removes the unsafe `scripts/pull-zone.sh`.

## Context

domain-os holds the authoritative registry state for its TLDs: domains, hosts, DNSSEC delegation material and EPP statuses. No component turns that state into a live DNS zone. What exists:

- **Read endpoints.** The TLD controller serves the apex, delegations, glue and DS records as `dns.RR` values or zone-file text, cursor-paginated (INV-11): `/tlds/:tld/dns/{resource-records,domain-delegations,glue-records,ds-records}` ([`internal/interface/rest/tld_controller.go:39`](../../internal/interface/rest/tld_controller.go#L39)).
- **The apex.** `entities.TLDHeader` ([`pkg/domain/entities/tldHeader.go`](../../pkg/domain/entities/tldHeader.go)) models SOA, NS, glue, DS and DNSKEY. `TLDService.GetTLDHeader` ([`internal/application/services/tld_service.go:140`](../../internal/application/services/tld_service.go#L140)) builds it from stored `tld_dns_records` rows. **The SOA is a stored row: nothing allocates or increments a serial.**
- **A sketch consumer, removed in this revision.** `scripts/pull-zone.sh` curled the apex and delegations into a file and ran `named-checkzone`. It called a delegations path that no longer exists, fetched a paginated endpoint once, and omitted glue and DS. It therefore produced a **truncated zone that passes a syntax check**, which is INV-05's failure mode applied to the registry's most critical output.
- **The verification half.** `CheckSerialDrift` ([`internal/application/workflows/serialDrift.go`](../../internal/application/workflows/serialDrift.go)) monitors SOA serial propagation across a nameserver fleet with RFC 1982 arithmetic, and `ZoneSlavingService` manages what is monitored. We monitor propagation of zones we do not yet produce. INV-05 violation 5 applies: a drift run is stored however many nameservers answered.

Machinery the design reuses:

- **Temporal workflow discipline**: documented workflows with activities, retries and Launchpad cards.
- **S3/MinIO streaming**: the `io.Pipe()` → `UploadStream()` pattern streams multi-GB exports and suits zone files of millions of records.
- **The event outbox**: `eventRelay` drains domain events. A later incremental path would consume them; see the prerequisites.
- **EPP status semantics**: [`docs/domain_status_overview.md`](../domain_status_overview.md) defines which statuses exclude a domain from the zone.
- **DNSSEC material**: `dnssec_service.go` manages DS/DNSKEY data at the registry level.

### Two audiences

The seam to downstream DNS is used by two groups who need different things from it.

| | Owns | Touches |
|---|---|---|
| **Registry operator** | Eligibility policy, publication schedule, guardrail thresholds, approving a mass deletion, rollback, "is it live?" | The admin API and UI and an audit trail. Never TSIG keys or nameserver configuration. |
| **DNS engineer** | The signer, key rollovers, transport, the secondary fleet, anycast, vendor choice | Standard DNS tooling and declarative configuration. Never the registry database or REST pagination. |

A design that serves only one of them pushes the other into workarounds. Examples are an operator filing a ticket to roll back, or a DNS engineer writing a poller against a paginated API.

### Constraints

- **Public DNS must not depend on domain-os availability.** Nameservers keep answering through app deploys, database failovers and outages. Registries are audited on this.
- **Never publish a broken zone.** A validation gate must be able to block publication, and a bad publication must be reversible in minutes.
- **Serials are sacred.** They are monotonic per zone (RFC 1982), allocated transactionally and never reused, because verification depends on them.
- **Memory-bounded.** Generation streams with cursor reads and piped writes, and never holds a whole zone in memory.
- **DNSSEC is required** for gTLD operation. Signing and key rollover are done by DNS software built for it, not in domain-os.
- **Hexagonal placement.** Ports go in the domain layer and adapters in infrastructure (INV-14). Orchestration is a documented Temporal workflow (INV-06).

## Decision

domain-os **compiles** each TLD zone into immutable, validated, versioned artifacts. It **delivers** them through a `ZonePublisher` port to publication targets that DNS engineers declare. The primary delivery contract is **standard DNS zone transfer** (AXFR/IXFR with NOTIFY and TSIG), served by a separate domain-os process to a signer that DNS engineers run. domain-os is never in the public query path and never signs.

```
 Registry state (Postgres)
        │  cursor reads
        ▼
 ┌─ GenerateZoneWorkflow (Temporal, per TLD) ────────────────────┐
 │ 1 build → 2 validate → 3 allocate serial + store version      │  ← operator policy:
 └───────────────────────────────┬───────────────────────────────┘    eligibility, schedule,
                                 │ zone_versions + S3 {tld}/{serial}    thresholds, approval,
                                 │ (zone, manifest, diff to previous)   rollback
              ┌──────────────────┼────────────────────┐
              ▼                  ▼                    ▼
       zone transfer       artifact drop        provider sync
       (cmd/zonexfr)       (file + reload)      (secondary only)
       AXFR/IXFR, NOTIFY,                                   ← DNS-engineer config:
       TSIG, catalog zone                                     publication targets
              │
              ▼
       Signer (Knot / BIND / PowerDNS / OpenDNSSEC), inline signing
              │  AXFR/IXFR + NOTIFY
              ▼
       Public authoritative fleet / anycast
              │
              ▼
 4 verify: serialDrift → version "converged"
```

### 1. Build: full regeneration, streaming

`GenerateZoneWorkflow` runs per TLD on a schedule (5–15 minutes) or on demand, on the `data-pipeline` queue, and is documented per the workflow template.
- It streams eligible delegations with cursor reads: NS records per domain, glue (A/AAAA) for in-bailiwick hosts, and DS records for secure delegations. Eligibility comes from EPP statuses: a domain on `serverHold` or `clientHold`, or with no linked hosts, is excluded.
- It prepends the apex from `TLDHeader`, with the SOA serial supplied by step 3 rather than the stored row.
- It writes a canonical, deterministically ordered zone file to S3/MinIO with `UploadStream`. A manifest goes alongside: record counts, checksum, serial and generation parameters.

Full regeneration is the only write path. It is deterministic, idempotent and auditable, and it stays as the reconciler if an incremental path is ever added (see Options, pattern 3).

### 2. Validation gate

No version is stored as publishable without passing these checks, as a distinct workflow step:
- **Syntactic:** `kzonecheck` or `named-checkzone` on the generated file.
- **Structural:**
  - the apex is complete (SOA and NS present);
  - glue is consistent (every in-bailiwick NS has glue, and there is no orphan glue);
  - the record counts in the manifest match the counts the cursor read. This is the INV-05 check: a truncated read must fail here, not pass `named-checkzone`.
- **Delta guardrails:** the workflow compares against the previous version. It refuses if the zone shrank by more than a configured percentage, or if the delegation count fell below an absolute floor. A breach parks the workflow on the existing HITL signal-to-confirm pattern, because a legitimate mass deletion must be approvable.

This step makes "empty zone published to the world" structurally impossible rather than procedurally unlikely.

### 3. Versions and serials

- The serial is allocated from a DB-backed monotonic counter, in the same transaction that writes the `zone_versions` row. **domain-os owns the unsigned zone's serial**, and nothing downstream edits it.
- Every version is an immutable S3 object keyed `{tld}/{serial}`. It is stored with its manifest and with **the diff to the previous version** (`{tld}/{from}-{to}.diff`). The operator's diff view and the delta guardrail need the diff now, and IXFR serves it later.
- Rollback republishes an earlier version's content under a fresh, higher serial. Serials never go backwards.
- A version has three states, recorded separately:
  - **published** (accepted and current);
  - **delivered** (per target: the target has fetched or loaded it);
  - **converged** (the public fleet serves it; see step 4).

### 4. Delivery: `ZonePublisher` and publication targets

A `ZonePublisher` port in the domain layer takes a published version and a target, and returns a delivery receipt. Infrastructure has three adapters:

| Adapter | Contract with downstream | Use |
|---|---|---|
| **Zone transfer** (`cmd/zonexfr`) | AXFR/IXFR + NOTIFY + TSIG, plus an RFC 9432 catalog zone listing the TLDs | **Primary.** Any RFC-compliant signer or secondary consumes it with configuration alone. |
| **Artifact drop** | Zone file + manifest, plus a reload (`knotc zone-reload`, `rndc reload`) or a signer-side puller | First delivery step (see the roadmap). After that: break-glass and consumers with no transfer path. |
| **Provider sync** | Vendor API (PowerDNS, Route 53, NS1, …) | Secondary distribution and small infrastructure zones only. Never the TLD primary path. |

**Publication targets are data.** Under ADR 0018 they vary per TLD and need history, so they are a typed table with a validating entity, edited through the admin API. A row holds:
- the adapter;
- the endpoint or allow-list;
- the TLDs it receives;
- a **reference** to a TSIG key in the secret store, never the key itself.

DNS engineers own these rows. To keep their configuration as code, they apply a file through the admin API; they never write to the database by hand. Process settings for `cmd/zonexfr` (listen address, artifact cache path, S3 location) are environment settings under ADR 0018.

**The zone-transfer service** follows the shape ADR 0013 and ADR 0020 set for the EPP and RDAP servers: a separate process with its own health, metrics and deployment.
- It serves only published versions, and reads only the artifact store and a local disk cache. **It never reads Postgres**, so it keeps serving the last good version through database and app outages.
- It answers only TSIG-authenticated clients on an allow-list, so it is not in the public query path.
- It sends NOTIFY when a version is published, and records each transfer as a delivery receipt.
- It uses `miekg/dns`, which is already a dependency.

**The signer is a "bump-in-the-wire" secondary.** It transfers the unsigned zone from `cmd/zonexfr`, signs it inline, and serves the signed zone to the public fleet. Key generation, rollovers and re-signing stay with the DNS software. domain-os keeps *managing* DS/DNSKEY registry data (`dnssec_service`) and does not sign. This corrects the July draft, which assumed that serving transfers from domain-os meant moving signing in-app. It does not.

### 5. Verification closes the loop

A delivered version becomes **converged** when serialDrift sees the public fleet serving it within the SOA-derived grace window. A `ZoneSlaving` configuration per production zone becomes mandatory, created with the zone's first publication.

Inline signers can serve their own serial: Knot's `serial-policy`, and BIND's separate serials for the raw and signed zone. Verification therefore compares against **the signed serial**, using the mapping from unsigned to signed serial that the signer reports at delivery. Signer configuration either preserves the incoming serial or the mapping is recorded; which one is settled with the first signer deployment, and the answer holds for every delivery adapter.

### 6. Placement

| Piece | Location |
|---|---|
| `Zone`, `ZoneVersion`, `PublicationTarget` entities; serial arithmetic | `pkg/domain/entities` |
| `ZoneRepository`, `ZonePublisher`, delegation-stream port | `pkg/domain/repositories` |
| `GenerateZoneWorkflow` + activities + `.doc.md` | `internal/application/workflows`, `activities` |
| `ZoneGenerationService` (trigger, versions, diff, rollback); `PublicationTargetService` | `internal/application/services` |
| Postgres delegation cursor, S3 artifact store, publisher adapters | `internal/infrastructure` |
| Zone-transfer server | `internal/interface/zonexfr`, `cmd/zonexfr` |
| Trigger, inspect, diff, rollback and target endpoints; Launchpad card | `internal/interface/rest` |

## Delivery roadmap: artifact drop → zone transfer

Zone transfer is the target contract, but it is new protocol code on the availability-critical path. Delivery therefore starts with the artifact drop and moves to zone transfer one TLD at a time. Only the domain-os → signer hop changes. The signer → public fleet hop is the same throughout.

The move is cheap only if the first step holds to these rules from day one:

1. domain-os owns the unsigned serial. The signer loads the whole file (Knot `zonefile-load: whole`) and does not bump it.
2. The signer is deployed as the signer, with TSIG on its outbound side and the public fleet as its secondaries. It is not deployed as a host domain-os writes files to.
3. Delivery goes through `ZonePublisher` with a per-target receipt, even when there is one target.
4. Every version stores its diff to the previous one.
5. Verification uses the signed-serial mapping (Decision 5).

| Step | Change | Rollback |
|---|---|---|
| 1 | Artifact drop live: file + reload. The signer is primary-from-file. | Republish an earlier version. |
| 2 | `cmd/zonexfr` in **shadow**, AXFR only. A shadow secondary transfers every publication, and its zone is compared with the file-drop zone; they must be identical. Conformance tests per ADR 0015 cover AXFR, TSIG, NOTIFY retry and SOA queries. | Stop the shadow. |
| 3 | Cut over per TLD: the signer's zone changes from file to secondary of `cmd/zonexfr`. Continuity holds because the next serial comes from the same counter and is strictly higher. | Point the signer back at the file; the drop adapter is still running. |
| 4 | IXFR, served from the stored diffs with fallback to AXFR. Then the catalog zone. | Disable IXFR; AXFR still works. |
| 5 | Keep the artifact drop as break-glass and for consumers without a transfer path. | — |

If step 2 is far off, an intermediate step 1.5 moves the reload trigger to the signer side: a puller next to the signer fetches the newest version from S3 when notified. That removes domain-os's need to execute on the signer host before any protocol code exists.

## Options considered

The options differ mostly in the seam to downstream DNS. Patterns 1 and 2 share the compiler, versions, serial counter and validation gate, and differ only in delivery.

### Pattern 1: Compiler + artifact drop

The July draft's decision. Build, validate, version, then push a file to the hidden primary and reload it.

**Pros:**
- deterministic, auditable and instant to roll back;
- every tool reads an RFC 1035 file;
- reuses Temporal, `UploadStream` and the HITL signal.

**Cons:**
- the reload step is vendor-specific and makes domain-os execute on, or be polled by, the DNS team's host, so it is the least portable part of the design;
- the DNS engineer's contract is "our script", not a protocol.

**Kept as the first roadmap step and as break-glass.**

### Pattern 2: domain-os as hidden primary over zone transfer ⭐

Pattern 1's core, delivered over AXFR/IXFR + NOTIFY + TSIG and a catalog zone, by a separate process that reads only artifacts.

**Pros:**
- the seam is a protocol every DNS engineer already operates;
- vendor choice, signer migrations, multi-signer (RFC 8901) and extra secondaries are configuration changes on their side;
- IXFR keeps small changes cheap;
- a catalog zone provisions new TLDs on secondaries automatically.

**Cons:**
- a new availability-critical network service with its own correctness and conformance burden: transfer edge cases, TSIG, NOTIFY retries;
- it needs HA and an artifact cache.

**Chosen as the target contract.**

### Pattern 3: Event-streamed incremental updates

Outbox events → applier → RFC 2136 dynamic updates or micro-versions served over IXFR, with full regeneration as the reconciler.

**Pros:** propagation in seconds, and near-zero cost per change.

**Cons:**
- there is no whole-zone validation point;
- a dropped or reordered event silently diverges the zone;
- it is always an addition to pattern 1 or 2, never a replacement;
- **it is blocked today by INV-01's documented defects:** the outbox insert does not share the business transaction, and all six producers swallow publish errors. Zone correctness cannot rest on a stream that can lose events.

**Deferred.** The trigger and prerequisites are below.

### Pattern 4: Desired-state reconciler with provider plugins

OctoDNS/DNSControl style: compute the desired record set, and have a plugin per provider diff it against the provider's actual state and converge.

**Pros:**
- first-class multi-provider fan-out;
- per-target health;
- a model platform teams know.

**Cons:**
- record-level API sync of a multi-million-record zone hits rate limits and leaves partial-apply states;
- most managed providers cannot host a registry-signed TLD zone under policy;
- domain-os ends up owning one integration per vendor.

**Kept only as the provider-sync adapter for secondaries and small zones.** Its *target model* (declared targets with per-target state) is adopted as publication targets.

### Rejected: domain-os serves public DNS

An in-path authoritative server built on `miekg/dns`. It directly violates the decoupling constraint: a deploy or database failover becomes a resolution outage for every registered domain, and online DNSSEC signing in-app is a large, audit-sensitive surface. This is unlike pattern 2, which answers only a few TSIG'd transfer clients from static artifacts.

## Trade-off analysis

Scores are 1–5, higher is better.

| Dimension | 1. Artifact drop | **2. Zone transfer** | 3. Incremental | 4. Reconciler |
|---|---|---|---|---|
| Validation point (never publish broken) | 5 | 5 | 2 | 3 |
| Rollback | 5 | 5 | 2 | 3 |
| Downstream vendor flexibility | 3 | 5 | 3 | 4 |
| Familiar to DNS engineers | 3 | 5 | 2 | 3 |
| Operator experience | 5 | 5 | 4 | 4 |
| Propagation latency | 3 | 3–4 | 5 | 3 |
| Decoupled from domain-os availability | 5 | 4 | 4 | 4 |
| Scale to millions of records | 4 | 5 | 4 | 2 |
| DNSSEC under registry policy | 5 | 5 | 3 | 2 |
| New code and operating burden (5 = least) | 4 | 3 | 1 | 2 |
| Fit with existing code and invariants | 5 | 4 | 2 | 3 |

Patterns 1 and 2 lead on the dimensions that matter most for a registry: safety, rollback and decoupling. Pattern 2 is the better contract with downstream, and the roadmap reaches it through pattern 1 at low risk. Pattern 3's only advantage, latency, is not yet a requirement.

## Prerequisites

- **Before verification gates "converged":** serialDrift runs must be quorum-aware. A run in which too few nameservers answered is inconclusive, not a pass (INV-05 violation 5).
- **Before any incremental path (pattern 3):**
  - fix both INV-01 defects: the outbox insert shares the business transaction, and no producer swallows a publish error;
  - land a contractual or product requirement for sub-minute propagation. Until then, incremental machinery is pure operating risk.

## Consequences

**Easier:**

- Registry state becomes resolvable. Every published zone traces to an immutable artifact, a serial and a workflow run.
- Rollback is one action: republish an earlier version.
- Operators and DNS engineers each own their half. Operators change policy and approve versions through the admin API. DNS engineers change signers, vendors and secondaries without a domain-os change.
- Adding a TLD reaches every secondary through the catalog zone, with no per-TLD ticket.
- The existing serialDrift investment becomes the acceptance test for every publication.
- DNSSEC key rollovers run on standard DNS tooling and runbooks.

**Harder:**

- Three new things to operate: `cmd/zonexfr`, a signer, and a public fleet or anycast contract.
- Zone eligibility rules (status combinations, `inactive`, IDN and edge-case host names) become externally visible behavior and must be specified exactly.
- The generation schedule sets a latency floor that support and registrars must understand: "changes are live within N minutes".
- A legitimate mass deletion needs a human approval. This friction is intended.

**To revisit:**

- An incremental path, when the prerequisites above are met.
- Whether the artifact drop is retired after step 5 or kept as break-glass.
- Per-tenant zones: the schema and S3 keys assume multiple TLDs across tenants from day one (ADR 0006), even if the first deployment runs one.

## Action items

1. [ ] `Zone`, `ZoneVersion` and `PublicationTarget` entities, and the DB-backed monotonic serial counter (RFC 1982), in the domain layer.
2. [ ] `ZoneRepository`, `ZonePublisher` and a cursor-based delegation-stream port in `pkg/domain/repositories`.
3. [ ] Zone-eligibility rules from EPP statuses, specified in `docs/domain_status_overview.md` and encoded in the delegation query.
4. [ ] Streaming builder activity: Postgres cursor → canonical zone file → S3, with manifest and diff to the previous version.
5. [ ] Validation activity: `kzonecheck`, apex/glue and count checks, delta guardrails with a HITL signal on breach.
6. [ ] `GenerateZoneWorkflow` + `.doc.md`, registered with a Launchpad card.
7. [ ] Artifact-drop adapter, and a Knot signer with inline signing in `deploy/` (roadmap step 1). Settle the signed-serial mapping.
8. [ ] Quorum-aware serialDrift runs. Then wire verification to mark versions converged, and create a `ZoneSlaving` configuration on a zone's first publication.
9. [ ] REST endpoints: trigger, list versions, inspect manifests, diff, rollback, and manage publication targets.
10. [ ] `cmd/zonexfr`: AXFR + TSIG + NOTIFY in shadow, with conformance tests (roadmap step 2); then cutover, IXFR and the catalog zone (steps 3–4).
11. [ ] Runbooks: bad-publication rollback, guardrail-breach approval, signer failure and rebuild, and cutover and fallback per TLD.
12. [x] Remove `scripts/pull-zone.sh`, which built truncated zones (done in this revision).
