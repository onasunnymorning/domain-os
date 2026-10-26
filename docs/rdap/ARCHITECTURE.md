# RDAP server: architecture

**Status:** target architecture plus a roadmap. The server skeleton exists (Phase 1); most of what this page describes does not yet. Each section says what holds today, and the [roadmap](#roadmap) says which phase builds the rest. Update this page in the same PR as each phase lands, so it never describes the system as intended rather than as built.

**Decisions:**
- [ADR 0020](../adr/0020-rdap-server-architecture.md): the server
- [ADR 0021](../adr/0021-rdap-publication-and-redaction.md): what a response may contain
- [ADR 0022](../adr/0022-rdap-access-policies.md): rate limits

**Every setting, its default and how to change it:** [POLICY_REFERENCE.md](POLICY_REFERENCE.md). **Which field each TLD publishes:** [PUBLICATION_REFERENCE.md](PUBLICATION_REFERENCE.md). **What to do when an alert fires:** [RUNBOOK.md](RUNBOOK.md).

**Code:** `cmd/rdap` is the composition root. The server lives in `internal/interface/rdap`, its reads in `services.RDAPService`, and the TLS it shares with the EPP server in `internal/infrastructure/tlsconfig`.

---

## Where it stands today

The server runs (Phase 1) and serves domains (Phase 2), but not yet nameservers or registrars:
- `cmd/rdap` terminates TLS on the node (TLS 1.2 and 1.3, the RFC 9325 suites, a hot-reloaded certificate), accepts PROXY protocol from trusted load balancers, and serves health, readiness and metrics on an ops port.
- It routes the registry-wide base URL and each TLD's (`{base}{tld}/`), answers `/help`, and gives RDAP error objects with CORS on every response.
- **Domain lookups answer with full domain objects** (Phase 2): statuses (RFC 8056), events, the registrar and its abuse contact, nameservers with glue, DS records, IDN names, ROID handles, the gTLD profile's notices, and contacts as the TLD's policy allows, redacted per RFC 9537.
- A nameserver or registrar that exists is answered **501** until Phase 3. Anything that does not exist is a 404. Rate limiting is Phase 4, so the server must not be deployed publicly before then.
- The TLD in a base URL is not yet checked against the TLDs the registry serves. Any label is accepted, and a domain outside that label is a 404.

`make rst-rdap` reports rdap-01 and rdap-04 (shape; the conformance tool grades them from Phase 6), rdap-05, 08, 09, 10 and 91 passing. rdap-91's CA and chain requirements are operational. The port-43 `cmd/whois` stays as it is until the later phase that retires it.

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

*Today:* built (Phase 1), except dual-stack publication, which is a deployment property (Phase 7). The certificate source, cipher suites and PROXY listener are the EPP server's, moved to `internal/infrastructure/tlsconfig`.

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

*Today:* the surface is built (Phase 1): routing, `/help`, errors, CORS, HEAD, 501 for searches, 405 for other methods, 400 for paths that are not RDAP queries, and IDN names accepted as U-labels or A-labels. Existing objects are answered 501 until Phase 2 (domains) and Phase 3 (nameservers and entities). The `/help` and error responses carry `rdapConformance`; the Terms of Use notice comes from `RDAP_TERMS_OF_SERVICE_URL` until Phase 3 moves it into each TLD's policy.

## 3. Publication and redaction

- Contacts are served only for roles that the TLD's current GA phase allows. A role it prohibits is never served, even if a contact is still linked.
- Within an allowed role, each field follows the TLD's publication policy (`publish`, `publish-if-disclosed`, `withhold`), and is narrowed further by the contact's RFC 5733 disclose flags. Withheld fields are listed in `redacted` (RFC 9537).
- `rdappolicy.Decide` is a pure function that every response path goes through. Every requester is `public` until the authenticated tier exists.

*Today:* built (Phase 2) with built-in policies: `rdappolicy.Decide` decides each contact field, and the response builders in `internal/interface/rdap` (`domain.go`, `contact.go`) only carry the decision out. `services.RDAPService` reads a contact only for a role the current GA phase allows and the publication policy publishes, so a contact in any other role is never read. A generic TLD has `gtld-rdp-2024` and any other `cctld-minimal` until Phase 3 makes publication policies data, assigned per TLD. Golden responses for a thick gTLD, a gTLD gone thin and a ccTLD are in `internal/interface/rdap/testdata/golden/`.

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

*Today:* the base metrics are served (Phase 1): `rdap_requests_total{route,code}`, `rdap_request_duration_seconds{route}`, `rdap_requests_in_flight`, `rdap_lookup_errors_total{route}`, `rdap_server_cert_not_after_timestamp_seconds`, `rdap_server_cert_reload_errors_total`, `rdap_draining` and `rdap_redactions_total{tld,field,method}` (never the values), plus the Go and process collectors. The `route` label is a fixed set, so a request path cannot mint a series. A `tld` label waits until the TLD in a path is checked against the TLDs served. Alerts, dashboard and runbook are Phase 5.

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
| 1 | Server skeleton: `cmd/rdap`, `internal/config/rdap.go` with its variables in the env registry and deployment contract, the EPP server's TLS moved to `internal/infrastructure/tlsconfig` and shared, PROXY protocol, the ops port and drain, routing per TLD base URL, RDAP error objects, CORS, HEAD, `/help`, `rdapConformance` and the Terms of Use notice, base metrics, `Dockerfile.rdap` in the CI image matrix, compose and Tilt | **Done**. rdap-04 (shape), 08, 09, 10 and 91 pass. Existing objects are answered 501 until Phases 2 and 3 |
| 2 | Domain lookup through `RDAPService`: statuses (RFC 8056), events, the registrar and its abuse contact, nameservers with glue, DS data, IDN names, ROID handles, the gTLD notices; `rdappolicy.Decide` with built-in publication policies by TLD type, the current GA phase's contact roles, disclose flags and `redacted` (RFC 9537); golden responses for thick, thin and thick-to-thin TLDs | **Done**. rdap-01 (shape) and rdap-05 pass. Reserved and blocked names are served as stored domains; there is no special case |
| 3 | Nameserver lookup (`HostRepository` lookup by name, the resolution rule), registrar entities by IANA ID; publication policies as data (`rdap_publication_policies`, assignments per TLD, built-ins `gtld-rdp-2024` and `cctld-minimal`), admin API under `rdap:policy:admin`. Moves rdap-02, 03, 06 and 07 | Planned |
| 4 | Rate limiting: the EPP GCRA limiter extracted to `internal/infrastructure/ratelimit`; access policies assigned to networks (`anonymous`, `monitoring`), per-address and per-aggregate buckets, the not-found cost, 429 with `Retry-After`, node limits, fail-open, admin API. **Must land before any public deployment** | Planned |
| 5 | Alert rules (`prometheus/rules/rdap.rules.yml`, with promtool unit tests), the *RDAP server* Grafana dashboard, this runbook filled in, and the test that keeps rules and panels to served metrics; optionally ICANN's MoSAPI view of RDAP availability as an alert | Planned |
| 6 | `cmd/tools/rdapload`; the RDAP Conformance Tool and its RST config pinned and run by the suite (skipped locally without Java, required in CI), with seeded thick and thin TLDs. rdap-01 to 04 graded by the tool | Planned |
| 7 | Deployment: dual-stack endpoint, certificate, TLS and IPv4/IPv6 consistency checks against staging (rdap-91, 92), IANA bootstrap entries, an OT&E `StandardRDAPOnly` run | Planned: needs a deployment |
| 8a | RFC 9560 OpenID Connect: token validation, capability discovery in `/help`, tiers and grants as inputs to `Decide`, responses never shared-cached | Later |
| 8b | Disclosure requests: intake, review, approve or deny with reasons, configurable deadlines (Registration Data Policy §10, NIS2 Article 28) and escalation | Later |
| 8c | Searches | Later |
| 8d | Retire `cmd/whois`, or serve port 43 from this service | Later |
