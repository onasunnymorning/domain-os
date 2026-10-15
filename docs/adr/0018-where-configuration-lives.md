# ADR 0018 — Where configuration lives: environment, database, or installation record

- **Status:** Accepted
- **Date:** 2026-10-14
- **Deciders:** Platform / Registry
- **Generalises:** [ADR 0013](0013-epp-server-architecture.md) decision 5 ("policy is data"), which drew this line for the EPP server only
- **Composes with:** [ADR 0010](0010-database-migrations.md) (the migrate job), [ADR 0017](0017-roid-repository-id.md) (the first installation record; proposed in #487), INV-17

## Context

Some settings are environment variables, some live in database tables edited through the admin API, and since ADR 0017 one value is also recorded in the database at first start. Contributors adding a setting need a rule for where it goes. Without one, settings end up in both places with no declared source of truth, or a generic settings table fills up with untyped strings nobody owns.

## Decision

Every setting has exactly one home, chosen by what kind of setting it is.

1. **Environment: bootstrap and deployment settings.** Use this for:
   - what a process needs before it can reach the database (DB and Redis connections, ports, TLS paths);
   - per-process or per-node tuning (pool sizes, timeouts, node ID);
   - references to secrets;
   - anything that differs between environments running the same code and data shape (dev, staging, prod).

   Such a setting changes with a deploy. It is read once at start, in `cmd/` or `internal/config` (INV-17). It is registered in `internal/config/env_registry.go`, which generates `deploy/contract.json` and `.env.example`; `TestEnvRegistryDrift`, `TestContractDrift` and `TestEnvExampleDrift` keep them in step.

2. **Database: business and policy settings.** Use this for what operators change at runtime, what varies per TLD, registrar or tier, and what needs history, an admin UI or a change without a restart. Examples: phases, prices, EPP credentials and allowed networks, and EPP policies (Phase 5).
   - Each kind of setting gets a **typed table** with a validating entity (INV-08), and a migration (ADR 0010).
   - It is edited through the admin API, never by hand in the database, so every change goes through the same validation and is attributable.
   - Processes cache it and reload it, by polling today (`EPP_CREDENTIAL_REFRESH_INTERVAL`, 30s).

3. **Installation record: identity that must never change.** Use this for facts about the data that every process must agree on and that cannot change without rewriting what was published. The ROID repository ID is the first (ADR 0017).
   - Such a fact is recorded in `installation_settings` on first start, and processes check against it at start.
   - The table holds only facts of this kind, each with a typed accessor and a check. **It is not a general settings store.**

**Secrets** are never stored in plaintext in the database or in the repository. They live in the secret store, and the environment carries the value or a reference to it.

### Moving a setting

Move a setting from the environment to the database when any of these becomes true:
- someone who does not deploy needs to change it;
- it varies per tenant, TLD or registrar;
- changes need history or approval;
- a restart to change it is no longer acceptable.

A setting never moves the other way just because the database is down; that is what caching and fail-open behaviour are for (ADR 0013).

## Consequences

- **A reviewer can ask one question of a new setting:** which kind is it? A setting in two places, an untyped key in `installation_settings`, or a policy limit read from the environment is a review finding.
- **The ROID repository ID is the open case.** It currently comes from the environment and is checked against the installation record. Because it describes the data rather than the deployment, it will eventually come only from the record: set once by an explicit init step, and read by every process. We will do this when setting it on every service becomes a burden, or when a second identity fact appears. The guard already holds the value, so the move is small.
- **Polling scales to tens of nodes.** At hundreds, database settings should be pushed instead: Postgres `LISTEN/NOTIFY`, or Redis pub/sub as the EPP session-kill path already uses.
- **Database settings are cached, never read per request.** A change is visible after at most one reload interval. Document that interval with the setting: for the EPP server, [`docs/epp/POLICY_REFERENCE.md`](../epp/POLICY_REFERENCE.md) records when each change takes effect.
