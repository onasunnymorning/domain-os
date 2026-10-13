# EPP server: architecture

**Status:** target architecture plus a roadmap. Most of what this page describes **does not exist yet**. Each section says what holds today, and the [roadmap](#roadmap) says which phase builds the rest. Update this page in the same PR as each phase lands, so it never describes the system as intended rather than as built.

**Decisions:** [ADR 0013](../adr/0013-epp-server-architecture.md), with credential rotation amended by [ADR 0014](../adr/0014-epp-credential-rotation.md). **Every setting, its default and how to change it:** [POLICY_REFERENCE.md](POLICY_REFERENCE.md). This page replaces `docs/EPP_PRODUCTION_ARCHITECTURE.md`, which described an aspirational design (anycast, Vault, RabbitMQ) and was removed rather than kept in sync.

**Code:** `cmd/epp` is the composition root. The server lives in `internal/interface/epp`. The current Redis limiter is `internal/infrastructure/epp/middleware`, which is due to be replaced in Phases 3 and 5.

---

## Where it stands today

The transport and session layer is in place (Phase 1). The command surface is still a **protocol stub**:
- It binds `<hello>`, login, logout and a placeholder `domain:check` that ignores the names it is asked about. Every other RFC 5730 command is answered 2101 (unimplemented).
- **Login is not authenticated.** Any request carrying a clID and a pw succeeds.
- The client certificate is required but **not verified** (`tls.RequireAnyClientCert`).

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

*Today:* everything in this section except the source-IP allowlist, which is Phase 2. The server certificate comes from file and is hot-reloaded. PROXY protocol, the per-node connection and handshake caps, the handshake timeout, drain on SIGTERM and the ops port are all in place.

## 2. Identity & credentials

A login must pass three checks:
1. **Source IP** is in the registrar's allowlist.
2. **Client certificate** matches an active pin of the registrar.
3. **Password** matches one of the login's active passwords.

**Logins and passwords are different things.**
- A registrar can have **several EPP logins**. They separate *systems or purposes*, for example a production fleet and a read-only monitoring login. Each has a role (`transform` or `readonly`), its own session cap and its own audit trail. Every login maps to one registrar, and objects are always sponsored by the registrar's ClID, never by a login.
- A login can hold **several passwords** (2 active by default). They exist for *rotation*. Passwords are hashed with Argon2id.

**Every credential follows the same lifecycle:** a password, a certificate pin and an IP allowlist entry each move `active` → `retiring` (until a set time) → `retired`. Each records when it was created, when it was **last used** and from where, so it is visible when an old credential is safe to retire. Retiring comes in two kinds:
- **Normal retire:** new logins are refused. Sessions already authenticated with the credential run until they end.
- **Compromise retire:** additionally kills every live session that used the credential, immediately. This works because each session records the credential IDs it logged in with (§3).

**Rolling a password without disruption:**
1. Add a new password through the admin API (later, the registrar portal).
2. Both passwords work while the registrar's systems switch over.
3. Watch the old password's last-used time.
4. Retire it once it is no longer used.

`<newPW>` keeps its strict RFC 5730 meaning: the new password replaces the one used to log in, which stops working at once. That is what ICANN's RST tests exercise. Registrars who need an overlap use add-then-retire. RFC 8807 (login security) is a follow-up item.

**Client certificate pins:**
- A pin is the SPKI SHA-256 fingerprint of a certificate's public key.
- **Pins belong to the registrar**, optionally restricted to named logins. A registrar that uses one certificate on every connection rolls it once, not once per login.
- Up to 3 pins can be active per registrar (current, next and a spare for staggered multi-site rollouts).
- **Renewing a certificate on the same key changes nothing**, because the SPKI is unchanged. Only a key change needs a new pin.
- A pin can be uploaded before its certificate's `notBefore`, so it is ready the moment the registrar switches.
- A certificate is **rejected outside its validity window**, with no grace period after `notAfter`. Expiry alerts fire at 30 and 7 days.
- Each pin records when it was last used, and the session registry shows how many live sessions rely on it, so a registrar can watch the new pin take over before retiring the old one.
- Registrars may use a public-CA or self-signed certificate. We do not run a CA (ADR 0013).

**Credential changes are announced.** Adding, retiring or expiring a password, pin or allowlist entry notifies the registrar's technical contact and is recorded as a domain event through the outbox (INV-01). An unexpected credential change is how a compromise shows up.

**Lockout is keyed on (login, source IP)** with exponential backoff. It applies only after the IP and certificate checks have passed, so a stranger cannot lock a registrar out.

**Rolling our server certificate:**
- The certificate comes from a public CA, so clients that validate by chain are unaffected by a roll.
- It is loaded from file and hot-reloaded: new handshakes get the new certificate, and live sessions are untouched.
- Registrars who pin *our* certificate are told the new fingerprint a set number of days ahead (optionally also published as a DANE/TLSA record).

*Today:* none of this exists. `Registrar` has no EPP logins, certificates or allowlist.

## 3. Session management

- **State machine:** CONNECTED → GREETED → AUTHENTICATED → CLOSED. A command before login, or a second login, is answered with 2002. A session runs one command at a time.
- **Timeouts:** a login timeout, an idle timeout, a maximum session lifetime and a per-frame read timeout.
- **Distributed session registry.** Each session is a lease in Redis, refreshed by heartbeat, carrying: session ID, clID, login, the IDs of the password and certificate pin it authenticated with, IP, node, connected-at, last-command-at and command count.
  - Counts are derived from live leases, not from counters, so they cannot drift.
  - A crashed node's sessions expire with their leases.
  - Acquiring a session slot is a single atomic operation, so concurrent connects cannot overshoot a cap.
- **Caps** apply per registrar, per login, per family, per source IP and per node. Over the cap, the server answers 2502 and closes.
- **Operators can list live sessions and kill one** through the admin API. They can also kill every session that authenticated with a given credential, which is the second half of a compromise retire (§2).

*Today:* the state machine and every timeout are in place. Each node keeps its own registry of live sessions, which drain uses. The cluster-wide lease registry, the caps that use it and the admin actions are Phase 3. Until then, connection counting across nodes uses the drifting counters described above.

## 4. Frame & protocol hygiene

- A **maximum frame size** is enforced from the 4-byte length header, before the body is read.
- XML is hardened: `<!DOCTYPE` is rejected, and depth and element count are capped.
- **XSD validation** runs against the object and extension URIs negotiated at login. A schema failure returns 2001, and an extension that was not announced is rejected.
- **Per-command object caps** (names per check, nameservers, DS records). A breach returns 2306.
- `clTRID` is echoed. `svTRID` is globally unique and doubles as the correlation ID in logs and audit.

*Today:* the frame size, DTD rejection, depth and element caps, the clTRID echo and the unique svTRID are in place. Commands that are not bound are answered 2000 (unknown) or 2101 (unimplemented), and the session continues. XSD validation is Phase 4, and per-command object caps are Phase 5.

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
  - `epp_connections_total{outcome}`: accepted, or why a connection was refused or closed early (node cap, handshake cap, failed handshake, login timeout, limiter, draining)
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

*Today:* structured JSON `slog`, and `/metrics` on the ops port. The ops port serves `epp_connections_total`, `epp_sessions_active`, `epp_tls_handshakes_active`, `epp_logins_total`, `epp_commands_total`, `epp_command_duration_seconds`, `epp_frame_bytes`, `epp_frames_rejected_total`, `epp_server_cert_not_after_timestamp_seconds`, `epp_server_cert_reload_errors_total` and `epp_draining`. The series with registrar or login labels arrive with authentication (Phase 2) and the session registry (Phase 3). There is no audit log yet.

---

## Roadmap

Each phase is one or more PRs and can be shipped on its own.

| Phase | Scope | Status |
|---|---|---|
| 0 | This document, ADR 0013, policy reference skeleton. Server moved to `internal/interface/epp`. Typed config in `internal/config/epp.go`. `slog` throughout. One end-to-end TLS session test | **Done** |
| 1 | Server certificate from file with hot reload, PROXY protocol v2, per-node caps, session state machine, clTRID/svTRID, configurable frame size and XML hardening, graceful drain, ops port with base metrics, TLS integration test harness | **Done** |
| 2 | EPP logins with multiple passwords, registrar-level certificate pins and IP allowlists, all with the retire lifecycle and last-used tracking (domain, persistence, admin API). Real authentication. Lockout keyed on (login, IP). Credential-change notifications. Certificate-expiry metric | Planned |
| 3 | Lease-based distributed session registry (replaces the middleware limiter), recording the credential IDs each session used. Session caps, fail-open, admin list and kill, including kill-by-credential | Planned |
| 4 | Command pipeline: XSD validation, policy hook, in-process dispatch to application services, real domain check/info/create with sponsorship checks (ADR 0006), audit log | Planned |
| 5 | Policy engine: versioned tiered policies, registrar families, token buckets, object caps, quotas, hot reload | Planned |
| 6 | Anti-dropcatch: failed-create budget and escalation, fair queueing, randomised release in `purgeLoop` | Planned |
| 7 | Alert rules, dashboards, load and drop-time simulation, fuzzing, RST v2.0 alignment, operator runbook | Planned |
