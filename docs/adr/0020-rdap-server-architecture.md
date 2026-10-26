# ADR 0020 — RDAP server: a separate read-only service, TLS on the node, one answer per lookup

- **Status:** Accepted
- **Date:** 2026-10-26
- **Deciders:** Platform / Registry
- **Composes with:** [ADR 0013](0013-epp-server-architecture.md) (the EPP server, whose process shape this copies), [ADR 0015](0015-rst-conformance-test-suites.md) (the RST suite this is built against), [ADR 0017](0017-roid-repository-id.md) (handles), [ADR 0018](0018-where-configuration-lives.md) (where its settings go), [ADR 0021](0021-rdap-publication-and-redaction.md) (what a response may contain), [ADR 0022](0022-rdap-access-policies.md) (rate limits)
- **Roadmap:** [`docs/rdap/ARCHITECTURE.md`](../rdap/ARCHITECTURE.md)

## Context

The registry has no RDAP. The only public lookup is `cmd/whois`, a port-43 server that answers domain queries only, with no metrics and no limits. RDAP is the registration-data protocol for gTLDs: the February 2024 gTLD RDAP Profile has been mandatory since 2025-08-21, its availability and response time are SLA terms monitored by ICANN, and the `StandardRDAP` suite is part of every RST v2.0 plan an RSP has to pass (Pre-Delegation, RSP Change, RSP Evaluation, and `StandardRDAPOnly`).

RDAP earns nothing, and it is public. It has to be cheap to run, hard to abuse, and observable enough to defend the SLA. The EPP server (ADR 0013, 0019) has already worked out most of how this repository runs a protocol server: a separate process, application services called in-process, limits held as data and reloaded by every node, a fail-open limiter, a private metrics registry with alerts that link to a runbook, and an RST scoreboard.

RST grades RDAP as follows (`internal/interface/rdap/testdata/rst/`):

- rdap-01 to rdap-04 run ICANN's RDAP Conformance Tool against domain, nameserver, registrar and `/help` responses.
- rdap-05 to rdap-07 check that HEAD answers 200 with a CORS header and an empty body.
- rdap-08 to rdap-10 check that random non-existent objects get a 404.
- rdap-91 checks TLS on every service port.
- rdap-92 checks that the IPv4 and IPv6 endpoints give the same answers.

## Decision

1. **RDAP is its own binary, `cmd/rdap`, deployed like `cmd/epp`.** `main.go` is the composition root only, and the server lives in `internal/interface/rdap/`. It reads registry state through application services in-process, never through the admin API, the same as ADR 0013 decision 8. It is read-only: nothing it does changes registry state. A separate process keeps a public, unauthenticated, SLA-bound service from sharing failure, capacity or middleware with the admin API, and lets it scale and be limited on its own.

2. **It is built on `net/http`, not Gin.** The surface is a handful of GET routes. HEAD must answer exactly as GET does without a body. Nothing should be inherited from the admin API's middleware stack (its auth, CORS and logging are wrong for a public protocol).

3. **TLS terminates on the node, as for EPP.**
   - The certificate loading, hot reload, expiry and hostname checks and the RFC 9325 cipher list are extracted from the EPP server into a shared package, and both servers use it.
   - An L4 load balancer passes TCP through with the PROXY protocol, which is trusted only from `RDAP_PROXY_TRUSTED_CIDRS`, so the server sees the client's real address for rate limiting (ADR 0022).
   - Keeping TLS in our code puts what rdap-91 checks under test, rather than in load-balancer configuration outside the repository.
   - Health, readiness and metrics are served on a separate plain-HTTP ops port, and a SIGTERM drains the node: `/readyz` answers 503 first.

4. **Every response is RDAP, including errors.**
   - The content type is `application/rdap+json`, and `Access-Control-Allow-Origin: *` is on every response, including errors and HEAD.
   - Errors (400, 404, 429, 501, 503) carry an RDAP error object.
   - The service is served under a base URL per TLD (`https://rdap.example/{tld}/`), as RST's `rdap.baseURLs` expects.
   - A request for something the server does not offer, such as a search, answers 501 rather than 404, so a client can tell "not supported" from "not found".

5. **One answer per lookup.**
   - **Domains** are looked up by name. A tombstoned (deleted) domain is a 404. A reserved domain is served like any other domain, as ADR 0011 says.
   - **Nameservers**: host names are unique per registrar, not across the registry (`entities.ErrHostAlreadyExists`), so a name can match several hosts. RFC 9082 has no "several" answer for an exact lookup, so the server picks one by a fixed rule:
     - first, the host sponsored by the sponsor of its superordinate domain, when the host is internal (that is the host RFC 5732 says the name belongs to, and the one whose glue is published);
     - otherwise, the oldest host with that name.
     - An ambiguous match is counted in a metric, so the rule's effect can be seen.
   - **Entities**: only registrars are served directly, by IANA ID, which is what the gTLD profile uses as a registrar's handle and what rdap-03 queries. A direct lookup of a contact is a 404. A contact has no TLD (ADR 0006 guardrail 2), so there is no one publication policy to apply to it (ADR 0021). Contacts appear only inside the response for a domain, under that domain's TLD's policy.

6. **Handles are ROIDs with the repository suffix**, appended by the repositories on read (ADR 0017), so RDAP, EPP and escrow name the same object the same way.

7. **Caching.**
   - Public responses carry `Cache-Control: public, max-age=60` and `Vary: Authorization`. Nothing is per-requester yet. The header is set now so that a future authenticated tier ([ADR 0021](0021-rdap-publication-and-redaction.md) decision 6) cannot leak into a cache that was set up before it existed.
   - The server keeps no shared response cache.
   - An in-process cache of public domain responses with a TTL of 30s at most may be added behind a setting, off by default, if load testing shows the database is the bottleneck.

8. **Not in scope until RST passes:**
   - authenticated access (RFC 9560 OpenID Connect) and a disclosure-request workflow;
   - searches;
   - retiring `cmd/whois` or serving port 43 from this service.

   Each is a named later phase in the roadmap.

## Consequences

- A new image, compose service, Tilt entry and deployment contract entry. Its settings are environment variables registered in `internal/config/env_registry.go` (INV-17), except the limits, which are policy (ADR 0022).
- The EPP server's TLS code moves to a shared package with no change in behaviour, and its tests move with it.
- `HostRepository` gains a lookup of every host with a name. `RegistrarRepository.GetByGurID` already exists.
- The nameserver rule can serve a host that a registrar other than the asker expected. The gTLD profile's handle in the response identifies exactly which object was served.
- rdap-91 and rdap-92 also depend on the deployment: the certificate installed, and published IPv4 and IPv6 addresses. The suite marks them operational, and the roadmap carries a pre-test checklist for them.
