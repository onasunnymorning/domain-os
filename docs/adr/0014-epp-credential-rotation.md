# ADR 0014 — EPP credentials rotate by overlap: multiple passwords, registrar-level pins, retire lifecycle

- **Status:** Accepted
- **Date:** 2026-10-13
- **Deciders:** Platform / Registry
- **Amends:** [ADR 0013](0013-epp-server-architecture.md), decisions 2 and 3
- **Composes with:** [ADR 0006](0006-tenancy-model.md) (objects are sponsored by the registrar's ClID, never by a login)

## Context

ADR 0013 gave each EPP login one password, changed through `<newPW>`, and pinned client certificates to individual logins, two per login.

That breaks the moment a registrar runs more than one client system. When one system changes the password, every other system holding the old one fails its next login. A registrar with three logins that shares one certificate across them has to upload every new certificate three times. And nothing tells either side when an old credential is safe to retire. Retiring a credential is a guess, and a wrong guess is an outage for the registrar.

## Decision

1. **A login holds several passwords; a password is a credential record, not a field.** Two may be active per login by default. A registrar rolls a password without disruption by adding a new one, switching its systems over, and then retiring the old one.

2. **`<newPW>` keeps its strict RFC 5730 meaning.** It replaces the password used to log in, effective immediately. ICANN's RST tests exercise this behaviour, and quietly keeping the old password valid would make a security-relevant command do less than it says. Registrars who need an overlap use add-then-retire through the admin API, and later the registrar portal.

3. **Logins and passwords answer different questions.** Logins separate systems or purposes, each with its own role, session cap and audit trail. Passwords exist for rotation. Every login maps to one registrar, and sponsorship stays with the registrar's ClID (ADR 0006).

4. **Certificate pins belong to the registrar**, optionally restricted to named logins. Up to three can be active (current, next, and a spare for staggered rollouts). A pin is the SPKI SHA-256 of the public key, so renewing a certificate on the same key needs no action, and a pin can be registered before its certificate's `notBefore`.

5. **Client certificates are rejected outside their validity window, with no grace period.** Alerts at 30 and 7 days before expiry give the warning. A grace period after `notAfter` would make the expiry date meaningless, and an expired certificate is exactly the one nobody is watching.

6. **One lifecycle for every credential:** `active` → `retiring` (until a set time) → `retired`. It applies to passwords, pins and IP allowlist entries alike. Each credential records when it was created and when and from where it was last used.
   - A **normal retire** refuses new logins and lets existing sessions run until they end.
   - A **compromise retire** also kills every live session that used the credential. This needs the session registry to record the credential IDs each session authenticated with (ADR 0013, decision 4).

7. **Credential changes are announced.** Adding, retiring or expiring a credential notifies the registrar's technical contact and emits a domain event through the outbox (INV-01).

8. **Our server certificate rolls by hot reload.** It comes from a public CA, so clients that validate by chain are unaffected. New handshakes get the new certificate while live sessions continue. Registrars who pin our certificate get the new fingerprint ahead of time.

## Consequences

- **Phase 2 grows.** It now covers password, pin and allowlist entities with the retire lifecycle and last-used tracking, and change notifications.
- **Phase 3 grows.** The session lease records the password ID and pin ID it authenticated with, and the admin API gains kill-by-credential.
- **Last-used tracking is a write on every successful login.** It is coalesced, recording at most once per credential per minute, so login bursts do not become write bursts.
- **The registrar portal**, when it exists, gets the same add, list, retire and kill actions, scoped to the registrar's own credentials (ADR 0006).

## Alternatives considered

- **`<newPW>` with a grace period for the old password.** This would make the protocol command itself non-disruptive. It is rejected because it changes what an RFC command means, and because a registrar changing its password after a suspected leak expects the old one to stop working immediately.
- **Per-login pins (ADR 0013 as written).** These are simpler to reason about, but turn every certificate roll into N uploads for a registrar with N logins. Rejected in favour of registrar-level pins with an optional login restriction, which can express the per-login case when it is wanted.
- **Certificate expiry grace.** Rejected (decision 5).
