# ADR 0019 — EPP policies: named limits, assigned to registrars and families

- **Status:** Accepted
- **Date:** 2026-10-23
- **Deciders:** Platform / Registry
- **Implements:** [ADR 0013](0013-epp-server-architecture.md) decision 3 ("sensible defaults, all configurable") and the policy engine of `docs/epp/ARCHITECTURE.md` §5, under [ADR 0018](0018-where-configuration-lives.md) (policy is data)
- **Phase:** EPP Phase 5a; command rates and the create quota added in 5b (decisions 9 and 10)

## Context

Until Phase 5 the EPP server held every registrar to the same limits, set as environment variables: sessions per registrar and per login (`EPP_MAX_SESSIONS_PER_REGISTRAR`, `EPP_MAX_SESSIONS_PER_LOGIN`) and names per check (`EPP_MAX_CHECK_NAMES`). A new registrar could not be held to less, a large one given more, or affiliated registrars (a dropcatcher's many accreditations) capped together. A login's `readonly` role was stored but not enforced.

The plan called for "versioned tiered policies (default, probation, custom), assigned per registrar with per-registrar overrides", registrar families, and hot reload.

## Decision

1. **A policy is a named set of limits, stored in the database** (`epp_policies`), edited through the admin API, with every change recorded as an outbox event carrying the actor and the policy before and after. Limits are typed columns, each at least 1, validated by the entity (INV-08).
2. **Two policies are built in:** `default`, which every registrar not assigned another has, and `probation`, for new registrars. The `migrate` job seeds them from code constants if they are not stored; their limits can be changed, they cannot be deleted. Every other policy is custom. "Tier" is not a separate concept: it is which policy a registrar has.
3. **A registrar is assigned one policy, or none for `default`. There are no per-registrar overrides.** A registrar that needs different limits gets its own policy. Overrides would make the limits in force for a registrar the merge of two records, and a policy change would not reach registrars whose override shadows it; one assignment per registrar keeps "what applies to this registrar" a single lookup. A login's own session cap (ADR 0014) stays, and replaces the policy's per-login cap: it is about one login's client, not the registrar's terms.
4. **Registrar families** (`registrar_families`, `registrar_family_members`) group affiliated registrars. A registrar is in at most one family, enforced by the members' key. A family is assigned a policy too; its family limits (`sessions_per_family` in 5a, family rates in 5b) cap the family together, while each member's own policy caps the member.
5. **Only per-registrar terms are policy.** The session caps and the names per check are. The nameserver and DS caps, and the accepted DS algorithms, stay rules of the registry's data, the same for every registrar; the source-address session cap stays a server setting, since it applies before anyone has logged in.
6. **Decisions are a pure function** (`epppolicy.Decide`) of the login's role, its registrar's policy and the command, so every rule is tested without a server. The EPP adapter carries the decision out at the policy hook between validation and the handler.
7. **Changes are serialised** by one transaction-scoped advisory lock, and each reads what it replaces inside its transaction, so the before and after of every change event are exact. Foreign keys tie assignments and family members to registrars (deleting a registrar deletes them) and to policies (a policy assigned cannot be deleted).
8. **Nodes reload the whole set** (policies, assignments, families) every `EPP_CREDENTIAL_REFRESH_INTERVAL`, with the credentials. The tables are small, so there is no version counter to poll: the audit trail is the change events, and "which version is in force" is "the last load", reported by `epp_policies_refreshed_timestamp_seconds`. A failed load keeps the set last loaded; before the first, the built-in starting limits apply (fail open, ADR 0013). A reload also moves the node's open sessions to their registrar's current family, so a family's count follows its membership; the session registry's renewal drops a session from its old family's index in the same step that adds it to the new one.

9. **Command rates (5b) are GCRA token buckets**, one per (registrar, class) and (family, class), the classes `check`, `info`, `transform` and `poll`. A policy's `rates` are its registrars' buckets and its `family_rates` a family's, so a family's rates come from the family's policy as its session cap does. A `check` costs one token per name, every other command one. A command takes from all its buckets at once, in one Redis script on Redis's clock, or from none. A command whose tokens are at most `epppolicy.MaxRateDelay` (1s) away is held until then, with its tokens taken, so a client sending one command at a time is slowed, not failed; one further away is refused with 2400 and takes nothing. EPP has no rate-limit result code: 2400 is the retryable failure, and 2502 would close the session, which costs a reconnect and a login. GCRA keeps one number per bucket (the time it would be full again), so there is no refill loop and nothing to drift. Without Redis, buckets are kept per node and `epp_policy_store_degraded{store="rates"}` is set (ADR 0013); after a Redis failure commands use the node's buckets for 5s before Redis is tried again, so an unresponsive Redis does not slow every command.
10. **The create quota (5b)** is `creates_per_day`, successful domain creates per registrar per UTC day; 0 is no cap. A create is counted before it runs and given back if it fails, so concurrent creates cannot pass the cap and failed creates do not count (counting those is Phase 6's failed-create budget). Over the cap is 2308 (data management policy violation).

## Consequences

- The three environment variables are removed (pre-v1: no fallback). Their values are the `default` policy's starting limits.
- `probation` starts with the default's session caps and names per check, a lower family cap (30), lower command rates and 100 creates a day.
- Assigning, re-assigning or changing a policy takes effect within the refresh interval, for the next command and the next login. Sessions already open stay open under a lowered session cap.
- Command rates and the create quota became policy columns in 5b with no change to how policies are assigned or reloaded. Policies stored before them were given their policy's starting values (a custom one the default's) by the migration.
- Holding a command occupies its session, which sends one command at a time anyway, for at most 1s; the node's other sessions are unaffected.
- There is no admin UI for policies yet; the admin API is the interface, like the EPP credentials and sessions.
