# Phase 0 Inventory — one-command local environment (#392)

Recon performed 2026-08-06 against `392-one-command-local-environment` (no commits ahead of `main`).
Verified by running the commands, not by reading the README.

**Gate status: 4 halt conditions fired, all now resolved.** Details in "Halt conditions" below; resolutions in "Decisions taken".

---

## Decisions taken

| Halt | Decision |
|---|---|
| Config source ambiguous | `.env.example` is **generated** from `internal/config/env_registry.go` by `cmd/tools/genenvexample`, drift-tested by `TestEnvExampleDrift` and wired into `make ci-envcheck`. Doppler is off the local path entirely; `make dev-doppler` preserves the maintainer flow. |
| Branch-tagged images | Every app service builds locally. `admin-init` and `whois` gained the `build:` stanza they were missing, and all `${BRANCH}` references now default to `latest`. Local never pulls an app image. |
| External deps in boot path | The live IANA fetch is replaced locally by an offline synthetic seeder (`ryAdminAPI seed`). Doppler is gone from `make dev`. Nothing in the boot or test path reaches the network after image pulls. |
| Version drift | `.tool-versions` added (Go 1.26.5, Node 22). `minio/mc:latest` pinned. All eight infrastructure images confirmed amd64 + arm64. |

Note on the Go `toolchain` directive: it was added and then reverted. This module vendors its dependencies, and adding `toolchain` makes `go build` refuse with *"updates to go.mod needed, disabled by -mod=vendor"* until `go mod tidy` runs. The existing `go 1.26.5` line already pins the exact patch version, so the directive was redundant; `.tool-versions` carries the developer-facing pin instead.

---

## Inventory

| Check | Finding | Status |
|---|---|---|
| **Makefile** | Exists, 17KB, ~60 targets. Already has `dev`, `test`, `stop`, `stop-full`, `clean`, `clean-docker`, `db-reset`, `ci-local`. No `doctor`. `make help` works (awk over `##` comments). Two extra makefiles: `Makefile.epp`, `Makefile.epp-server` (not included by the root Makefile). | ✅ answered |
| **Compose** | `docker-compose.yml` (12KB, current — touched 2026-07-16) and `docker-compose-ci.yml`. Also a Tiltfile for `make local`. Compose is the real path; Tilt is broken (see below). | ✅ answered |
| **Services** | **RabbitMQ does not exist anywhere in this repo.** Actual set — see table below. | 🔴 halt |
| **Versions** | `go.mod` says `go 1.26.5`, no `toolchain` directive. No `.tool-versions`. README claims "Go 1.21+" (stale by 5 minors). Frontend `.nvmrc` = 22, `engines.node` = `>=20 <23`; this machine runs node **v25.6.1** — already out of range. Two images unpinned. | 🔴 halt |
| **Config** | Three competing sources — see "Config sources". | 🔴 halt |
| **Migrations** | No migration files and no migration tool. GORM `AutoMigrate` runs on API boot, gated by `AUTO_MIGRATE=true`. Repeatable, but schema is defined only by Go structs. | ✅ answered |
| **Seed data** | **No seed mechanism exists.** Closest thing: `admin-init` runs `ryAdminAPI init-registrars`, which is not a seeder (see "Boot path"). `initdata/icannRegistrarList.csv` is a public ICANN download, unused by the boot path. | ✅ answered |
| **Test suite** | **Green.** `go test ./...` → exit 0, 29 packages ok, 0 failures, 36 with no test files. Full run ≈ 4 min warm. | ✅ answered |
| **External deps** | Yes, three of them, all in the boot path. See "Halt conditions" #1. | 🔴 halt |

---

## Services actually required to boot

The ticket assumed Postgres + Redis + RabbitMQ. The real set, from `docker-compose.yml` profiles:

| Service | Image | Profile | Port | Notes |
|---|---|---|---|---|
| `db` | `postgres:16.1` | infra, essential, full | 5432 | Application database |
| `redis` | `redis:7-alpine` | infra, essential, full | 6379 | EPP sessions + rate limiting |
| `minio` | `minio/minio:RELEASE.2025-09-07T16-13-09Z` | infra, essential, full | 9000/9001 | S3-compatible, escrow + snapshots |
| `minio-setup` | `minio/mc:latest` | infra, essential, full | — | **unpinned tag** |
| `temporal` | `temporalio/auto-setup:1.25.2` | essential, full | 7233 | Workflow engine — replaces the assumed RabbitMQ |
| `temporal-postgres` | `postgres:14` | essential, full | — | **Second Postgres, two majors behind `db`** |
| `temporal-ui` | `temporalio/ui:2.44.0` | essential, full | 8081 | |
| `admin-api` | `gprins/domain-os-api:${BRANCH}` | essential, full | 8080 | |
| `admin-init` | `gprins/domain-os-api:${BRANCH}` | essential, full | — | one-shot |
| `unified-worker` | `gprins/domain-os-worker:${BRANCH}` | essential, full | — | |
| `epp-server` | `gprins/domain-os-epp:${BRANCH:-latest}` | essential, full | **700** | privileged port |
| `whois` | `gprins/domain-os-whois:${BRANCH}` | full | **43** | privileged port |
| `mcp-server` | `gprins/domain-os-mcp:${BRANCH}` | full | 3001 | |
| `prometheus` | `prom/prometheus:v3.10.0` | full | 9090 | |
| `grafana` | `grafana/grafana:12.4.1` | full | 3000 | **collides with Next.js dev default** |

Named volumes: `db`, `prom_data`, `redis_data`, `temporal_pgdata`, `minio_data` — all five exist, so `make reset` has explicit targets.

Health checks: 8 services have them. `admin-init` correctly waits on `admin-api: service_healthy`, so the wait-on-health contract is already partly satisfied — no `sleep` anywhere.

**Multi-arch:** `postgres:16.1`, `postgres:14`, `redis:7-alpine`, `temporalio/auto-setup:1.25.2`, `temporalio/ui:2.44.0` all publish amd64 + arm64. Prometheus, Grafana, and MinIO not yet verified (manifest queries timed out); all three are known multi-arch upstream but this needs confirming in Phase 1.

---

## Config sources (ambiguous — halt)

Three sources disagree about what is authoritative:

1. **Doppler** — `DOPPLER := doppler run --` is baked into `make dev`, `dev-build`, `dev-logs`, `dev-frontend`, `stop`, `stop-full`, `clean-docker`, `local`, `askg`, and both Tilt modes. `doppler` is installed on this machine but **not authenticated** (`doppler me` returns an empty table). So `make dev` currently fails for me too.
2. **`example.env`** — 50 variables, well commented, each marked `[REQUIRED]`/`[OPTIONAL]`/`[SECRET]`. But the file is named `example.env`, not `.env.example`, and its header says Doppler is the real mechanism.
3. **`internal/config/env_registry.go`** — the actual authority. A typed registry with per-variable `Required`, `RequiredWhen`, and `Description`, plus per-service contracts in `contract.go` and drift tests (`TestEnvRegistryDrift`, `TestContractDrift`) wired into `make ci-envcheck`. It generates `deploy/contract.json` via `cmd/tools/gencontract`.

The committed `.env` contains only `AUTH0_WORKER_CLIENT_ID` and `AUTH0_WORKER_CLIENT_SECRET` — it is gitignored and is not a working local config.

**Implication for Phase 3:** `.env.example` must be *generated* from `env_registry.go` by a tool alongside `gencontract`, and drift-tested. Hand-writing it re-creates the problem this repo already solved once.

---

## Boot path — what reaches the network

`make dev` → `doppler run -- docker compose --profile essential up -d`.

1. **Doppler auth.** Cloud secret manager, requires login. A collaborator hits this at minute one.
2. **Docker Hub pulls of branch-tagged images.** `admin-init`, `whois`, and `unified-worker`'s image tag `${BRANCH}` resolves to the sanitised current branch name. On this branch that is `gprins/domain-os-api:392-one-command-local-environment`, which does not exist on Docker Hub. `admin-api`, `epp-server`, `unified-worker`, and `mcp-server` have a `build:` stanza and will build locally; **`admin-init` and `whois` do not** — they can only pull. So `make dev` on any branch without a published image fails at `admin-init`.
3. **IANA over the public internet.** `admin-init` → `SyncRegistrarsWorkflow` → `GetIANARegistrars` activity → `GET {API}/ianaregistrars` → `https://www.iana.org/assignments/registrar-ids/registrar-ids.xml`. This is the only thing that populates registrars on a fresh database, and it is a live fetch.

**Not a problem:** there is no OpenTelemetry code in this repo at all, and no Grafana Cloud. Grafana is a local container in the `full` profile only. Observability is New Relic (`NEW_RELIC_ENABLED=false` by default) and Prometheus (`PROMETHEUS_ENABLED=false`). **Constraint 2's OTel/Grafana Cloud concern does not apply.** Auth0 also defaults to `AUTH0_ENABLED=false`. Ticket gap #1 is therefore narrower than feared: no AWS, no Grafana Cloud — but Doppler, Docker Hub, and IANA all sit in the boot path.

---

## Test suite

Run with a `postgres:16.1` container on 5432, as `make test-unit` does:

```
exit 0 — 29 ok, 0 FAIL, 36 no-test-files
```

Run again with **no database at all**, to isolate what needs live services:

| Package | Without Postgres |
|---|---|
| `internal/infrastructure/db/postgres` | FAIL |
| `internal/interface/rest/tests` | FAIL |
| *everything else (27 packages)* | passes offline |

Only two packages need a live service, and it is only Postgres — no test needs Redis, Temporal, MinIO, or any network endpoint. The `ianaregistrars` and `icannspec5` tests are pure unit tests despite their names; they never call out.

The failure mode is poor: a raw GORM `connection refused` against database `dos_unittests` with no skip and no actionable message. That is exactly the wall a new collaborator hits.

**Build tags in use:** `//go:build eval` (`internal/askg/eval`) and `//go:build integration` (`epp/middleware/rate_limiter_test.go`) — both correctly excluded from a default `go test ./...`.

---

## Fixture provenance (Constraint 1)

No escrow-derived fixture is committed. Detail:

- `initdata/icannRegistrarList.csv` — public ICANN accredited-registrar export. Clean.
- `testdata/example56.{cert,key}.pem` — committed TLS keypair for EPP. **Answers gap #3:** EPP TLS materials exist, so conformance tests are not blocked. But a private key is committed to the repo; Phase 2 should generate these in `make dev` and drop them from git, per the ticket's own wording.
- `internal/application/workflows/testdata/serial_drift_cases.yaml` — synthetic, uses `*.example` nameservers. Clean.
- `internal/askg/eval/testdata/cases.yaml` — **one real registration**: `01-net.radio`, with its real nameservers (`ns1–ns5.safebrands.*`) and real sponsoring registrar (`1290-safebrands`). This is public registration data, **not registrant PII** — no name, email, phone, street, or postal field appears anywhere in the file. The other four domains are invented (`bigcorp.best`, `expired-domain.best`, …). Low severity, but it is real-world data and should be swapped for a synthetic equivalent while we are here.
- `local/co-import/escrow-import-co-*.zip` — **a real escrow deposit on disk.** `local/` is gitignored, so it is not committed and not a fixture. Flagging it only so it never becomes one.

**Verdict on gap #6: no fixture needs replacing on PII grounds.** One cosmetic real-data reference to swap.

---

## Additional defects found (not in the ticket)

1. **`make test` cannot run after `make dev`.** `make dev` binds host 5432 for the `db` service; `make test-unit` starts its own `postgres:16.1` on host 5432. The second one fails to bind. This breaks the acceptance criterion "`make test` green immediately after `make dev`, no manual steps between" — today it is not green, it is a port collision.
2. **`make test` opens a browser.** `test-unit` ends with `go tool cover -html=coverage.out`, which launches a browser window on success. Wrong behaviour for a target a newcomer runs to check their setup, and wrong for any non-interactive use.
3. **`make local` is broken.** The Tiltfile's first line is `local('cat VERSION')`; there is no `VERSION` file in the repo. Tilt fails before doing anything.
4. **Privileged ports.** EPP binds 700 and WHOIS binds 43. Fine on Docker Desktop for macOS; on Linux these need root or `CAP_NET_BIND_SERVICE`. Cross-platform parity (gap #4) is a real risk here, independent of image architecture.
5. **Port 3000 collision.** Grafana maps `3000:3000`; Next.js dev defaults to 3000. The Tiltfile already works around this with `PORT=3002`, but plain `make dev-frontend` does not.
6. **Repo hygiene.** ~250MB of committed build artifacts and databases sit in the root: `askg` (33MB), `escrowImport` (50MB), `ry-admin` (69MB), `unified` (53MB), `rest.test` (50MB), `mcp` (47MB), `coverage.out` (39MB), `jisc_analysis.db` (1.9MB), plus `README.md.bak`, `Tiltfile.bak`, `analysis_debug.json`. A collaborator's first `git clone` pays for all of it. Out of this ticket's scope, but it directly affects time-to-green.

---

## Halt conditions fired

**1. Unknown service dependency — resolved.** RabbitMQ is not part of this system; Temporal is. The dependency surface is 8 infrastructure containers, not 3. Phase 2 is materially larger than the ticket assumed.

**2. Version drift — unresolved, needs a decision.** No `toolchain` directive, no `.tool-versions`, README five minors stale, this machine's Node is outside the declared range, and two image tags float (`minio/mc:latest`, `${BRANCH:-latest}`). Prod intent for each service version is not recorded anywhere I can find.

**3. Config source ambiguous — unresolved, needs a decision.** Doppler vs `example.env` vs `env_registry.go`. Constraint 2 (boot fully offline) cannot hold while `make dev` shells through `doppler run`. This is the single biggest decision in the ticket: it changes the shape of Phases 3 and 5, and it changes the maintainer's own daily workflow.

**4. External dependency in the boot path — unresolved, needs a decision.** Doppler auth, Docker Hub branch-tagged pulls, and the live IANA fetch. The IANA fetch is the interesting one: it is the *only* thing that puts registrars in a fresh database, so removing it from the boot path requires the synthetic seed (gap #2) to exist first. Gap #2 is therefore a blocker for Constraint 2, not an independent nice-to-have.

---

## Phase 6 friction log

Every defect below was found by running the thing, not by reading it. All are fixed unless marked otherwise.

| # | Friction | Fix |
|---|---|---|
| 1 | **`make dev` returned non-zero on a fully successful run.** `docker compose up --wait` treats *any* container exit as a failure, including a one-shot task exiting 0. Hit twice — first `admin-init`, then `minio-setup`. | One-shot services are listed in `ONESHOT_SERVICES`, held back from the `--wait` set with `--scale N=0`, then run to completion in order with their exit codes propagated. |
| 2 | **`password authentication failed for user "postgres"`** on a stack that had been run before. Postgres applies `POSTGRES_PASSWORD` only when initialising an empty data directory, so a volume created under the old Doppler-supplied password rejects the new `.env` credentials. The error names SASL, not the volume — nothing points at the cause. | Cannot be fixed in code (it is Postgres' documented behaviour). Documented as its own row in the README troubleshooting table with the exact fix: `make reset && make dev`. |
| 3 | **Port 7000 is taken on a stock Mac.** macOS AirPlay Receiver (`ControlCenter`) listens there, so the first choice of EPP host port collided on my own machine. `make doctor` caught it before any collaborator could. | EPP publishes on **7700**. |
| 4 | **Two dead env vars produced a warning on every compose command.** `NEWRELIC_LICENCE_KEY` and `NEWRELIC_USER_KEY` were passed by both compose files; the code reads `NEW_RELIC_LICENSE_KEY`. Nothing read either name. | Corrected in `docker-compose.yml` and `docker-compose-ci.yml`. |
| 5 | Seeder rejected `.test` URLs — the URL validator requires a real public suffix. | Contact emails and URLs use `example.com` (also RFC 2606 reserved); domain names stay on `.test`. |
| 6 | Seeder rejected phase name `ga` — phase names are `ClIDType`, minimum 3 characters. | Renamed to `ga-phase`. |
| 7 | Registrars could not be accredited: `NewRegistrar` defaults to `readonly`, and `AccreditFor` requires `ok`. | The seeder sets `Status: ok` and `IANAStatus: Accredited` explicitly. |
| 8 | Domains seeded as `inactive`, so an "active" subject was not actually active. Hosts were created but never delegated. | The seeder links both nameservers per domain via `AddHostToDomainByHostName`, then re-reads before stamping lifecycle state. |
| 9 | `go install swag` sits in the shared `build` base stage of `Dockerfile`, so it runs regardless of `SKIP_SWAG`. | **Not fixed** — it is one shared layer across all three images, so the cost is paid once. Restructuring the Dockerfile is out of scope here. |
| 10 | **The port remap silently did nothing.** Compose *appends* to a service's `ports` list across files rather than replacing it, so `epp-server` published **both** 700 and 7700. The privileged port was still bound and the Linux-parity fix was cosmetic. Only visible by inspecting `docker ps` on a running stack. | `ports: !override` on all three remapped services. Verified in the merged config: whois `4343→43`, grafana `3010→3000`, epp `7700→700`, and nothing on 700/43/3000. |
| 11 | **`make doctor` false-positived on its own stack.** The ownership check grepped `docker ps` output for `:9000->`, but Docker collapses contiguous ports into ranges (`0.0.0.0:9000-9001->9000-9001/tcp`), so MinIO looked like a foreign process holding the port. A preflight tool that cries wolf is worse than none. | Ownership is now asked of Docker directly: `docker ps --filter publish=<port> --filter label=com.docker.compose.project=domain-os`. |

## Phase 6 status: partially complete

**What was verified, on this machine:**

- `make reset` drops all five named volumes cleanly.
- `make reset && make dev` succeeds end to end, exit 0, all services healthy, database migrated and seeded.
- `make test` is green **while the dev stack is running** — the acceptance criterion that a port collision made impossible before this ticket.
- `make test` opens no browser and prints a coverage total.
- `make doctor` correctly identified a real port conflict and named the process holding it.
- Seeding is idempotent: a second run skips every object and exits 0.

**What was NOT verified — this remains open:**

The ticket is explicit that my machine cannot validate this phase, and it is right. Every base image was already in the local image store, the Go module cache was warm, and BuildKit had layers from previous builds. The timings I measured (**17s** for a warm `reset` + `dev`; **253s** for a partially-warm rebuild) say nothing about a genuinely cold machine.

**The ≤15 minute target is therefore unconfirmed.** The honest estimate is that a true cold run is dominated by two things: pulling eight infrastructure images, and the `admin-api` image's `apk add python3 py3-pip bind-tools graphviz` plus `pip install dnsviz` layer, which alone ran over 200 seconds here. 15 minutes is plausible but not demonstrated.

Closing this out needs a clean VM, a fresh container with its own Docker daemon, or a colleague's machine — with a stopwatch and someone recording every point they had to stop and think.

## Revised read on the ticket's known gaps

| # | Gap | Phase 0 finding |
|---|---|---|
| 1 | Tests requiring cloud services | **Closed.** No test touches AWS, Grafana Cloud, or any network endpoint. Two packages need local Postgres only. |
| 2 | Synthetic seed generator missing | **Confirmed, and promoted to a blocker.** It is the prerequisite for removing the IANA fetch from the boot path. `gofakeit/v7` is already a dependency. Realistically more than the 2h estimate — recommend spinning out. |
| 3 | EPP TLS materials | **Closed.** `testdata/example56.{cert,key}.pem` exist and EPP tests pass offline. Should still move to generated-not-committed. |
| 4 | Cross-platform parity | **Partly closed.** Five of eight images confirmed amd64+arm64; three unverified. New risk found: privileged ports 700 and 43 on Linux. |
| 5 | Test suite runtime | **Closed.** ≈4 min warm for the full suite. Fast enough not to need splitting in this ticket. |
| 6 | Fixture provenance | **Closed.** No PII anywhere. One real domain reference to swap, cosmetic. |
