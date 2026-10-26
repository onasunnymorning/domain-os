# RDAP server: architecture

**Status:** target architecture plus a roadmap. **Nothing on this page exists yet except the pinned RST spec and its completeness test (Phase 0).** Each section says what holds today, and the [roadmap](#roadmap) says which phase builds the rest. Update this page in the same PR as each phase lands, so it never describes the system as intended rather than as built.

**Decisions:**
- [ADR 0020](../adr/0020-rdap-server-architecture.md): the server
- [ADR 0021](../adr/0021-rdap-publication-and-redaction.md): what a response may contain
- [ADR 0022](../adr/0022-rdap-access-policies.md): rate limits

**Every setting, its default and how to change it:** [POLICY_REFERENCE.md](POLICY_REFERENCE.md). **Which field each TLD publishes:** [PUBLICATION_REFERENCE.md](PUBLICATION_REFERENCE.md). **What to do when an alert fires:** [RUNBOOK.md](RUNBOOK.md).

**Code:** `cmd/rdap` will be the composition root. The server lives in `internal/interface/rdap`.

---

## Where it stands today

There is no RDAP server. The registry's only public lookup is `cmd/whois`, a port-43 server that answers domain queries only, with no metrics or limits. It stays as it is until the later phase that retires it or serves it from here.

`internal/interface/rdap/testdata/rst/` pins ICANN's RST v2.0 RDAP catalogue (release `v2026.08`), and `make rst-rdap` reports all twelve cases as pending, each with the phase below that delivers it.

---

## Principles

1. **Read-only.** RDAP is a view of authoritative registry state. Nothing it does changes registry state.
2. **The TLD's current policy decides what is shown, field by field, before serialisation.** Stored data never decides on its own, and neither does the registrar (ADR 0021).
3. **Cheap to run, hard to abuse.** The service earns nothing, so limits start low, are held as data and are raised from evidence (ADR 0022).
4. **Never fail the SLA monitors.** ICANN's probes and RST's clients have their own policy. A refusal is a 429, never a 503.
5. **Every decision is observable.** Every request, refusal and redaction is a metric. Personal data never goes in a metric or a log.
6. **Reuse what EPP proved.** Use the same process shape, TLS code, limiter, policy model, metrics conventions and RST scaffolding. Shared code is extracted, never copied.

## 1. Process and transport

- `cmd/rdap` is its own binary, image and deployment, and reads registry state through application services in-process (ADR 0020 decisions 1 and 2).
- An L4 load balancer passes TCP through with the PROXY protocol, trusted only from `RDAP_PROXY_TRUSTED_CIDRS`. TLS terminates on the node with the RFC 9325 cipher suites and a hot-reloaded certificate. That TLS code is shared with the EPP server.
- Health, readiness and `/metrics` are served on a separate ops port. On SIGTERM, `/readyz` answers 503 and the node drains.
- The service is dual-stack (IPv4 and IPv6) on port 443, with one base URL per TLD (`https://rdap.example/{tld}/`).

*Today:* nothing. Phase 1 builds it.

## 2. Protocol surface

- Supported lookups:
  - `GET` and `HEAD` on `domain/{name}`, `nameserver/{name}`, `entity/{handle}` and `help`.
  - Searches answer 501.
  - Other methods are refused.
- Every response:
  - is `application/rdap+json`;
  - carries `Access-Control-Allow-Origin: *`;
  - carries `rdapConformance` and the gTLD profile's notices.

  Errors are RDAP error objects.
- Nameserver resolution: when several registrars hold a host with the same name, the host sponsored by its superordinate domain's sponsor is served, otherwise the oldest (ADR 0020 decision 5).
- Entities: registrars are served by IANA ID. Contacts are served only inside a domain response, never by a direct lookup.

*Today:* nothing. Phase 1 builds the surface, Phase 2 domains, and Phase 3 nameservers and entities.

## 3. Publication and redaction

- Contacts are served only for roles that the TLD's current GA phase allows. A role it prohibits is never served, even if a contact is still linked.
- Within an allowed role, each field follows the TLD's publication policy (`publish`, `publish-if-disclosed`, `withhold`), and is narrowed further by the contact's RFC 5733 disclose flags. Withheld fields are listed in `redacted` (RFC 9537).
- `rdappolicy.Decide` is a pure function that every response path goes through. Every requester is `public` until the authenticated tier exists.

*Today:* nothing. Phase 2 builds `Decide` with a built-in default policy, and Phase 3 makes publication policies data assigned per TLD.

## 4. Rate limiting

- Each request takes from two GCRA buckets in Redis: the client's address (IPv4 /32 or IPv6 /64) and its aggregate (/24 or /48). A not-found lookup costs more.
- The limits are named policies assigned to CIDR ranges. `anonymous` is the default and `monitoring` covers ICANN's probes. Every node reloads them by polling.
- If Redis is unavailable the limiter fails open to per-node buckets.
- Node limits are environment settings: in-flight requests, connections and timeouts.

*Today:* nothing. Phase 4 builds it, on the limiter extracted from the EPP server.

## 5. Observability

- Metrics are served from a private Prometheus registry on the ops port, using the EPP conventions: label constants, `_total`, `_seconds`, and `_timestamp_seconds` for refreshes and expiries.
- Alert rules link to the runbook. A test fails if a rule or a dashboard panel names a metric the server does not serve.
- Request latency is measured against the SLA's 400 ms round-trip time. Availability is measured against the monthly downtime allowance.

*Today:* nothing. Phase 1 adds the base metrics, and Phase 5 the alerts, dashboard and runbook.

---

## Conformance

ICANN's RST v2.0 `StandardRDAP` catalogue is the acceptance test ([ADR 0015](../adr/0015-rst-conformance-test-suites.md)). `internal/interface/rdap/rst_rdap_test.go` accounts for every case. `make rst-rdap` prints which pass, which are pending and in which phase, and which are operational. RST grades rdap-01 to rdap-04 with ICANN's RDAP Conformance Tool, so from Phase 6 the suite runs the pinned tool against the server, as the EPP suite runs xmllint. Every phase below should move cases from pending to passing, and its PR should say which.

### Pre-test checklist (for an RST run)

- One `rdap.baseURLs` entry per TLD, with a trailing slash and port 443.
- `rdap.testDomains`, `rdap.testNameservers` and `rdap.testEntities` (registrars by IANA ID) all exist.
- `general.registryDataModel` per TLD: `maximum` if its current policy allows a registrant, `minimum` if not (ADR 0021 decision 7).
- `epp.hostModel` is `objects`.
- `rdap.profileVersion` is the February 2024 gTLD RDAP Profile.
- RST's `rdap.clientACL` networks are assigned the `monitoring` policy.
- A and AAAA records are published for the RDAP hostname, and the certificate covers it (rdap-91, rdap-92).
- Each gTLD's base URL is registered in IANA's RDAP bootstrap.

## Roadmap

Each phase is one or more PRs and can be shipped on its own.

| Phase | Scope | Status |
|---|---|---|
| 0 | Pin the RST v2.0 RDAP catalogue (`internal/interface/rdap/testdata/rst/`), its completeness test and `make rst-rdap`; ADRs 0020, 0021 and 0022; ADR 0006 amended with the public read plane; this page, the policy and publication references and the runbook as skeletons | **Done** |
| 1 | Server skeleton: `cmd/rdap`, `internal/config/rdap.go` with its variables in the env registry, the shared TLS package extracted from the EPP server, PROXY protocol, the ops port and drain, routing per TLD base URL, RDAP error objects, CORS, HEAD, `/help`, `rdapConformance` and notices, base metrics, `Dockerfile.rdap`, compose and Tilt. Moves rdap-04 (shape), 08, 09 and 10 to passing | Planned |
| 2 | Domain lookup through a new `RDAPService`: statuses (RFC 8056), events, the registrar and its abuse contact, nameservers, DS data, IDN names, ROID handles, reserved and deleted domains; `rdappolicy.Decide` with a built-in publication policy, the current GA phase's contact roles, disclose flags and `redacted` (RFC 9537); golden responses for thick, thin and thick-to-thin TLDs. Moves rdap-01 (shape) and 05 | Planned |
| 3 | Nameserver lookup (`HostRepository` lookup by name, the resolution rule), registrar entities by IANA ID; publication policies as data (`rdap_publication_policies`, assignments per TLD, built-ins `gtld-rdp-2024` and `cctld-minimal`), admin API under `rdap:policy:admin`. Moves rdap-02, 03, 06 and 07 | Planned |
| 4 | Rate limiting: the EPP GCRA limiter extracted to `internal/infrastructure/ratelimit`; access policies assigned to networks (`anonymous`, `monitoring`), per-address and per-aggregate buckets, the not-found cost, 429 with `Retry-After`, node limits, fail-open, admin API. **Must land before any public deployment** | Planned |
| 5 | Alert rules (`prometheus/rules/rdap.rules.yml`, with promtool unit tests), the *RDAP server* Grafana dashboard, this runbook filled in, and the test that keeps rules and panels to served metrics; optionally ICANN's MoSAPI view of RDAP availability as an alert | Planned |
| 6 | `cmd/tools/rdapload`; the RDAP Conformance Tool and its RST config pinned and run by the suite (skipped locally without Java, required in CI), with seeded thick and thin TLDs. rdap-01 to 04 graded by the tool | Planned |
| 7 | Deployment: dual-stack endpoint, certificate, TLS and IPv4/IPv6 consistency checks against staging (rdap-91, 92), IANA bootstrap entries, an OT&E `StandardRDAPOnly` run | Planned: needs a deployment |
| 8a | RFC 9560 OpenID Connect: token validation, capability discovery in `/help`, tiers and grants as inputs to `Decide`, responses never shared-cached | Later |
| 8b | Disclosure requests: intake, review, approve or deny with reasons, configurable deadlines (Registration Data Policy §10, NIS2 Article 28) and escalation | Later |
| 8c | Searches | Later |
| 8d | Retire `cmd/whois`, or serve port 43 from this service | Later |
