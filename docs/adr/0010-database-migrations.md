# ADR 0010 — One migrator, guarded consumers: how the schema changes in deployed environments

- **Status:** Accepted
- **Date:** 2026-10-02
- **Deciders:** Platform
- **Composes with:** [#410](https://github.com/onasunnymorning/domain-os/issues/410) (choosing a versioned migration tool, still open; see "Destructive changes" below)
- **Infra counterpart:** alpaca-infra `docs/adr/ADR-004-release-process.md` (how an environment runs the `migrate` job)

## Context

The schema is defined only by Go structs and applied by GORM `AutoMigrate`
(`internal/infrastructure/db/postgres/connection.go`, `AutoMigrate`), plus a
list of idempotent `CREATE INDEX IF NOT EXISTS` statements. Until now the only
way to run it was `AUTO_MIGRATE=true` on admin-api, which migrates inside the
API's startup, before the HTTP listener binds. That has three problems in a
deployed environment:

1. **Concurrency.** In a rolling deploy two admin-api tasks migrated at once and
   blocked each other on locks. The new task never answered its health check
   and was killed. The test environment therefore runs with `AUTO_MIGRATE=false`,
   and migrating became a manual flip-apply-roll-flip-roll procedure that was
   easy to skip. 0.9.0's escrow tables shipped without being created.
2. **Coupling.** A long migration, such as an index on a large table, runs
   inside a health-checked rolling deploy.
3. **No guard.** Nothing stops a service from running against a schema older
   than its code. The failure shows up later, as `relation ... does not exist`
   inside some request or activity.

## Decision

**Exactly one code path changes the schema, and every long-running service
refuses to start on a schema older than itself.**

1. **`ryAdminAPI migrate`** (`cmd/api/ry-admin/migrate.go`) is the deployed
   migrator, published in the contract as the `migrate` job
   (`deploy/contract.json` → `jobs.migrate`). It calls `postgres.Migrate`, which:
   - takes a Postgres **advisory lock** (key `0x646f736d696772`, "dosmigr"),
     waiting up to 10 minutes, so concurrent migrators queue instead of
     deadlocking;
   - runs `AutoMigrate`, unchanged;
   - records the build version in a single-row **`schema_version`** table. The
     version **only moves forward**: an older build's migrate, after a code
     rollback, applies its additive changes and leaves the newer version in
     place;
   - exits non-zero on any failure, so a release process stops before rolling
     anything.

   It is idempotent. On an up-to-date schema it applies nothing.
2. **The schema guard.** admin-api, unified-worker and mcp-server call
   `postgres.EnforceSchemaGuard` right after connecting. If `schema_version` is
   missing or older than the build version, they log one line naming both
   versions and exit 1. A **newer** schema is accepted, because that is a code
   rollback.
   - `SCHEMA_GUARD=warn|off` is an escape hatch, declared in the contract and
     not meant to be set in a deployed environment.
   - Non-release builds (`buildinfo.Version` of `dev`, or anything that isn't
     MAJOR.MINOR.PATCH) are neither gated nor recorded.
3. **`AUTO_MIGRATE=true` stays for local development** (docker-compose, `make
   dev`). It now goes through the same `postgres.Migrate`, so it gets the same
   lock and version record, and a locally migrated database passes the guard.
   Deployed environments set it to `false` and run the `migrate` job instead.

## Why this is safe: expand-only

`AutoMigrate` creates missing tables, columns and indexes. It never drops or
renames anything. So:

- **Migrate, then roll.** Old code runs against the migrated schema throughout
  a rolling deploy, because everything it uses is still there.
- **Code rollback needs no schema rollback.** The guard accepts a newer schema,
  and the recorded version doesn't move backwards.

### Destructive changes (not covered by this ADR's mechanism)

Dropping or renaming a column, or a type change that rewrites a table, cannot be
done safely by `AutoMigrate` and must never happen automatically. Such a change
is made as **expand → contract across two releases**:

1. Release N adds the new shape. Code writes both shapes and reads the new one.
2. Release N+1 removes the old shape in an explicit, reviewed migration step.

The first time a change needs a contract step, it triggers [#410](https://github.com/onasunnymorning/domain-os/issues/410):
adopt a versioned tool (goose or atlas) for those steps, run by the same
`migrate` job, under the same lock and version record.

## Consequences

- **Deploy order is now a contract:** run `jobs.migrate` to completion
  (exit 0), then roll services. An environment that skips it gets services that
  refuse to start, with the reason in their first log lines. It does not get a
  service that half-works.
- **Every release requires a migrate run**, even with no schema change. On an
  up-to-date schema the run applies nothing and takes seconds.
- `schema_version` is a new table, created by the first `migrate` run.
- The guard adds one indexed single-row read to startup.

## Alternatives considered

- **Keep `AUTO_MIGRATE` on admin-api, with the lock added.** That fixes the
  deadlock but keeps the migration inside a health-checked rolling deploy, and
  nothing gates the other services.
- **Migrate in an init container of each service.** It's automatic, but a long
  migration then happens inside a rolling deploy with health checks running,
  and there is no single place to snapshot first or to stop the release on
  failure.
- **Adopt goose or atlas now.** That would turn every change into hand-written
  SQL today, to solve a problem (destructive changes) we don't have yet. It
  stays open as #410, with a concrete trigger.
