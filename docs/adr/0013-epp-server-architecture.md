# ADR 0013 — EPP server: layered enforcement, policy as data, pinned registrar certificates

- **Status:** Accepted
- **Date:** 2026-10-13
- **Deciders:** Platform / Registry
- **Supersedes:** none. It replaces the aspirational `docs/EPP_PRODUCTION_ARCHITECTURE.md`, which was never an ADR.
- **Composes with:** [ADR 0006](0006-tenancy-model.md) (registrar scope by sponsorship governs every EPP transactional verb), [ADR 0010](0010-database-migrations.md) (the new credential and policy tables ship through the single migrator)

## Context

The EPP server is a protocol stub:
- It accepts any login.
- It requests a client certificate but does not verify it.
- It presents a self-signed certificate generated at boot.
- It binds only greeting, login, logout and a placeholder `domain:check`.

Its Redis limiter counts connections with INCR/DECR under a TTL, so counts drift. Its per-registrar caps never fire because no registrar is ever recorded on the session. It never calls its request-rate check, and it locks accounts by clID alone, so anyone who knows a clID can lock that registrar out. [`docs/epp/ARCHITECTURE.md`](../epp/ARCHITECTURE.md) lists the details.

*Correction to ADR 0006.* ADR 0006's context section says the EPP server "already authenticates per registrar" and stamps the registrar on the session. It does neither: the login handler accepts any credentials, and the line that would have stamped the registrar was a no-op that has since been removed. ADR 0006's conclusion is unaffected, since it already treats the transactional surface as not yet built.

Before building that surface we need to settle how the server is shaped, so that each verb lands on that shape.

## Decision

1. **Enforcement is layered, and the cheapest check runs first:** network (source-IP allowlist, PROXY protocol), then TLS, then identity, then session, then frame, then policy, then business logic. Each layer rejects what it can before the next one runs.

2. **Registrar certificates are pinned. We do not run a CA.**
   - A registrar uploads any certificate, from a public CA or self-signed.
   - We pin its SPKI SHA-256 fingerprint to one EPP login.
   - Each login may hold two active certificates, so rotation can overlap.
   - Revocation is deleting the pin, which takes effect at once. There is no CRL or OCSP to operate.
   - A login succeeds only when the source IP, the certificate and the password all match.

   We revisit a registry-run CA only if registrars ask us to issue certificates.

3. **A registrar has EPP logins; credentials do not live on `Registrar`.** Each login carries a role, a session cap and its own audit trail. Lockout is keyed on (login, source IP) and runs after the IP and certificate checks, so a stranger cannot lock a registrar out.

4. **Session counts are leases, not counters.** Each session is a Redis lease refreshed by heartbeat. A cap is checked and taken in one atomic operation. A dead node's sessions expire with their leases.

5. **Policy is data.**
   - Session caps, command rates, object caps, quotas and the failed-create budget are versioned, tiered and overridable per registrar and per registrar family.
   - They are stored in the database, edited through the admin API, and hot-reloaded by every node.
   - Server settings (ports, timeouts, frame size, TLS) are environment variables read once at start (INV-17).
   - [`docs/epp/POLICY_REFERENCE.md`](../epp/POLICY_REFERENCE.md) lists every setting with its default and how to change it.

6. **`check` is metered per name.** The per-command name cap (default 50) only sets efficiency. Capacity is set by the names-per-second budget of the registrar's tier.

7. **If Redis is unavailable, enforcement fails open.** Nodes fall back to local caps and buckets, and expose `epp_policy_store_degraded` so the condition alerts. A registry that refuses every session because a cache is down fails its SLA. A node-local cap still bounds the damage.

8. **EPP calls application services in-process.** `internal/interface/epp` is an inbound adapter beside `internal/interface/rest`, calling the same services. It does not go through the REST API. `cmd/epp` is the composition root only.

9. **A failed-create fee is deferred, but the data for it is not.** Daily failed-create counts per registrar are kept from the start, so a fee can be introduced when the registry policy is published without needing historical data that nobody kept.

## Consequences

- **New domain concepts:** EPP login, EPP certificate, registrar IP allowlist entry, EPP policy, and registrar family. Each has a validating constructor (INV-08) and ships through the migrator (ADR 0010).
- **The admin API gains Redis access** to list and kill live sessions.
- **The middleware limiter is deleted** rather than repaired, in Phase 3 (sessions) and Phase 5 (rates).
- **Behind a load balancer, PROXY protocol is mandatory.** Without it, every per-IP control silently becomes a global one.
- **Randomising the pendingDelete release order changes `purgeLoop`.** The randomness has to come from an activity or `workflow.SideEffect` to stay replay-safe (INV-06).

## Alternatives considered

- **A registry-run private CA.** It gives tighter control over what a certificate asserts. The cost is that we must operate issuance, a CRL or OCSP, and a renewal path for every registrar. Pinning gives the same assurance for authentication with none of that. Rejected for now.
- **Keeping INCR/DECR counters and fixing the TTL.** Any counter needs a decrement that is guaranteed to run, and a crashed node never runs it. Leases expire on their own. Rejected.
- **Fail closed when Redis is down.** This turns a cache outage into a registry outage. Rejected in favour of local caps plus an alert.
- **Metering `check` per command.** That makes the batch-size cap a capacity knob: raising it from 20 to 50 would multiply scanning capacity by 2.5. Rejected.
