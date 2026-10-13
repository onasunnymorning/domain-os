# EPP server: architecture

**Status:** target architecture plus a roadmap. Most of what this page describes **does not exist yet**. Each section says what holds today, and the [roadmap](#roadmap) says which phase builds the rest. Update this page in the same PR as each phase lands, so it never describes the system as intended rather than as built.

**Decisions:** [ADR 0013](../adr/0013-epp-server-architecture.md). **Every setting, its default and how to change it:** [POLICY_REFERENCE.md](POLICY_REFERENCE.md). This page replaces `docs/EPP_PRODUCTION_ARCHITECTURE.md`, which described an aspirational design (anycast, Vault, RabbitMQ) and was removed rather than kept in sync.

**Code:** `cmd/epp` is the composition root. The server lives in `internal/interface/epp`. The current Redis limiter is `internal/infrastructure/epp/middleware`, which is due to be replaced in Phases 3 and 5.

---

## Where it stands today

The server is a **protocol stub**:
- It binds greeting, `<hello>`, login, logout and a placeholder `domain:check` that ignores the names it is asked about.
- **Login is not authenticated.** Any request carrying a clID and a pw succeeds.
- The client certificate is requested but **not verified** (`tls.RequireAnyClientCert`).
- The server certificate is self-signed, generated at boot and valid for one day.
- Transaction IDs are fixed placeholders.

The Redis limiter has known structural flaws, all fixed by redesign rather than patching:
- **Connection counts drift.** They use INCR/DECR with a 5-minute TTL, so a session open longer than that drops out of the count, and a crashed node leaks counts until the TTL expires.
- **Check-then-increment is not atomic**, so concurrent connects can overshoot a cap.
- **Per-registrar caps never fire**, because the registrar ID is never recorded on the session.
- **`CheckRequestRate` is never called.**
- **Lockout is keyed on clID alone**, so anyone who knows a registrar's clID can lock it out.

---

## Principles

1. **Defence in layers, cheapest check first:** network, then TLS, then identity, then session, then frame, then policy, then business logic. Reject as early as possible.
2. **Policy is data, not code.** Limits are versioned and tiered, overridable per registrar and per registrar *family*, and change without a restart.
3. **Every decision is observable:** every accept, reject, throttle and command is a metric and an audit record.
4. **Stateless nodes, shared counters.** Any node serves any registrar. Global limits live in an atomic shared store, and each node enforces local caps as a backstop.
5. **Transport policy here, business rules in the domain.** This layer caps *volume and shape* (how many names in a check, how many sessions). Whether a given name or contact is valid stays in entity constructors (INV-08).

## 1. Network & transport

- An L4 load balancer passes TCP through, so TLS terminates **on the EPP node** and the client certificate stays visible.
- The load balancer sends **PROXY protocol v2** so the real client IP survives. Without it, every per-IP cap is a global cap. The header is trusted only from configured load-balancer CIDRs.
- **Per-registrar source-IP allowlists.** A connection from an IP on no allowlist is dropped before the TLS handshake.
- TLS 1.2 minimum (1.3 preferred) with modern cipher suites, and a handshake timeout.
- Each node caps total connections and concurrent handshakes.
- On SIGTERM the node stops accepting, finishes in-flight commands and closes sessions within a drain timeout.
- Health, readiness and `/metrics` are served on a separate ops port.

*Today:* plain TCP listener, TLS 1.2 minimum, no PROXY protocol, no allowlist, no drain, no ops port.

## 2. Identity & credentials

A login must pass three checks:
1. **Source IP** is in the registrar's allowlist.
2. **Client certificate** is pinned by SPKI SHA-256 fingerprint to a specific *EPP login*. Registrars may upload certificates from a public CA or self-signed ones. Each login has up to two active certificates so rotation can overlap. Revocation takes effect immediately, and expiry is tracked and alerted.
3. **Password** is hashed with Argon2id and can be changed with `<newPW>`. RFC 8807 (login security) is a follow-up item.

Other rules:
- **A registrar can have several EPP logins.** Each has a role (`transform` or `readonly`), its own session cap and its own audit trail.
- **Lockout is keyed on (login, source IP)** with exponential backoff, and is applied only after the IP and certificate checks pass. A stranger therefore cannot lock a registrar out.
- The server certificate is loaded from file and hot-reloaded, so rotation does not drop sessions.

*Today:* none of this exists. `Registrar` has no EPP logins, certificates or allowlist.

## 3. Session management

- **State machine:** CONNECTED → GREETED → AUTHENTICATED → CLOSED. A command before login, or a second login, is answered with 2002. A session runs one command at a time.
- **Timeouts:** a login timeout, an idle timeout, a maximum session lifetime and a per-frame read timeout.
- **Distributed session registry.** Each session is a lease in Redis, refreshed by heartbeat, carrying: session ID, clID, login, IP, certificate fingerprint, node, connected-at, last-command-at and command count.
  - Counts are derived from live leases, not from counters, so they cannot drift.
  - A crashed node's sessions expire with their leases.
  - Acquiring a session slot is a single atomic operation, so concurrent connects cannot overshoot a cap.
- **Caps** apply per registrar, per login, per family, per source IP and per node. Over the cap, the server answers 2502 and closes.
- **Operators can list live sessions and kill one** through the admin API.

*Today:* epp-lib's idle, read, write and total timeouts apply. There is no state machine, and counting uses the drifting counters described above.

## 4. Frame & protocol hygiene

- A **maximum frame size** is enforced from the 4-byte length header, before the body is read.
- XML is hardened: `<!DOCTYPE` is rejected, and depth and element count are capped.
- **XSD validation** runs against the object and extension URIs negotiated at login. A schema failure returns 2001, and an extension that was not announced is rejected.
- **Per-command object caps** (names per check, nameservers, DS records). A breach returns 2306.
- `clTRID` is echoed. `svTRID` is globally unique and doubles as the correlation ID in logs and audit.

*Today:* the maximum frame is 1000 bytes, which is too small for a real create with DNSSEC. There is no XSD validation and there are no caps.

## 5. Policy enforcement

A `PolicyEnforcer` sits between parse and dispatch: `Decide(session, command)` returns allow, delay, reject(code) or close(2502). It checks, in order:
- **Command allowlist** for the login's role.
- **Per-command object caps.**
- **Token buckets (GCRA)** per (registrar, command class) and per (family, command class). The classes are `check`, `info`, `transform` and `poll`. A `check` costs one token **per name**, so the batch size is about efficiency, not capacity. Every other class costs one token per command.
- **Quotas,** such as daily creates for a probationary tier.

Policies are versioned and tiered (default, probation, custom). They are assigned per registrar with optional overrides, and are hot-reloaded by every node. If Redis is unavailable, enforcement **fails open** to node-local caps and buckets, and raises a metric and an alert.

### Anti-dropcatch and fairness

1. **Failed-create budget.** Count `domain:create` results of 2302 (object exists) per registrar and per family over a sliding window, and escalate: tarpit, then 2502, then temporary suspension of the login with an alert. Daily per-registrar counts are kept so that an excessive-activity fee can be added once the policy is published.
2. **Tarpitting before rejecting.** Response delay grows with the failure rate. This breaks tight loops without breaking legitimate clients.
3. **Family caps.** Dropcatchers operate many affiliated accreditations. Session and rate caps also apply to the corporate family, not only to each clID.
4. **Fair queueing on the create path.** A per-registrar concurrency limit stops one burst from starving others, and a per-node admission cap protects the database.
5. **Randomised release.** Names leaving pendingDelete are purged in random order, at an unpublished offset within a window.
6. **Check→create correlation.** Detect bursts of checks followed by an immediate create on the same name. This ships behind a flag and is enabled once we have data.

*Today:* none of this exists.

## 6. Observability

- **Prometheus metrics:**
  - `epp_sessions_active{registrar,login,node}`
  - `epp_connections_total{outcome}`
  - `epp_tls_handshake_failures_total{reason}`
  - `epp_logins_total{result}`
  - `epp_commands_total{registrar,command,object,code}`
  - `epp_command_duration_seconds`
  - `epp_frame_bytes`
  - `epp_policy_decisions_total{rule,action}`
  - `epp_failed_creates_total{registrar}`
  - `epp_client_cert_expiry_seconds{clid,login}`
  - `epp_policy_store_degraded`
- **"Who is connected"** comes from the session registry through the admin API.
- **Alerts** cover cap saturation, failed-create spikes, lockout storms, certificate expiry and a degraded policy store.
- **Audit log** of every command and response, keyed by svTRID and clTRID, with `pw`, `newPW` and `authInfo` redacted. It is written durably (not telemetry, per INV-01) with configurable retention.
- **Structured `slog` only.**

*Today:* structured JSON `slog`. No metrics, no audit log.

---

## Roadmap

Each phase is one or more PRs and can be shipped on its own.

| Phase | Scope | Status |
|---|---|---|
| 0 | This document, ADR 0013, policy reference skeleton. Server moved to `internal/interface/epp`. Typed config in `internal/config/epp.go`. `slog` throughout. One end-to-end TLS session test | **Done** |
| 1 | Server certificate from file with hot reload, PROXY protocol v2, per-node caps, session state machine, clTRID/svTRID, configurable frame size and XML hardening, graceful drain, ops port with base metrics, TLS integration test harness | Planned |
| 2 | EPP logins, pinned certificates and IP allowlists (domain, persistence, admin API). Real authentication. Lockout keyed on (login, IP). Certificate-expiry metric | Planned |
| 3 | Lease-based distributed session registry (replaces the middleware limiter). Session caps, fail-open, admin list and kill | Planned |
| 4 | Command pipeline: XSD validation, policy hook, in-process dispatch to application services, real domain check/info/create with sponsorship checks (ADR 0006), audit log | Planned |
| 5 | Policy engine: versioned tiered policies, registrar families, token buckets, object caps, quotas, hot reload | Planned |
| 6 | Anti-dropcatch: failed-create budget and escalation, fair queueing, randomised release in `purgeLoop` | Planned |
| 7 | Alert rules, dashboards, load and drop-time simulation, fuzzing, RST v2.0 alignment, operator runbook | Planned |
