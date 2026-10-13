# EPP server: settings and policy reference

This page lists every setting that governs the EPP server: its default, where it lives, how to change it, and when a change takes effect. When you add, remove or re-default a setting, update this page in the same PR. The design behind these settings is in [ARCHITECTURE.md](ARCHITECTURE.md), and the decisions are in [ADR 0013](../adr/0013-epp-server-architecture.md) and [ADR 0014](../adr/0014-epp-credential-rotation.md).

Settings live in one of two places:

| Kind | Lives in | How to change | Takes effect |
|---|---|---|---|
| **Server settings**: how a node listens, terminates TLS and times out | Environment variables, read once at start by `internal/config/epp.go` (INV-17). Each is registered in `internal/config/env_registry.go` and published in `deploy/contract.json` (`make generate-contract`) | Set the variable in the deployment's environment (Doppler for local and dev) | On restart. An unusable value stops the server at start rather than being silently replaced by a default |
| **Policy limits**: what a registrar may do over its sessions | Database, versioned per tier with per-registrar overrides *(Phase 5)* | Admin API *(Phase 5)* | Within the policy reload interval, without a restart *(Phase 5)* |

A value marked *(planned)* does not exist yet. The phase that introduces it is named in [ARCHITECTURE.md § Roadmap](ARCHITECTURE.md#roadmap).

---

## Server settings: environment

| Variable | Default | Meaning |
|---|---|---|
| `EPP_PORT` | `700` | TCP port the EPP listener binds |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` (or `warning`), or `error`, in any case |
| `REDIS_HOST` | `localhost` | Redis holding the shared connection and lockout counters |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | *(empty: no auth)* | Redis password. Secret |

## Server settings: fixed in code today

These are constants in `internal/interface/epp/server.go`, so changing one means a code change and a release. Phase 1 moves them to environment variables with the new defaults shown.

| Setting | Today | Phase 1 default | Notes |
|---|---|---|---|
| Max frame size | 1000 B | 64 KiB | A create with 13 NS, 8 DS and extensions is about 8 KB. A 50-name check is at most about 15 KB |
| Read timeout (per frame) | 10s | 30s | |
| Write timeout | 2m | 60s | |
| Idle timeout | 350s | 10m | |
| Max session lifetime | 1h | 24h | |
| Login timeout | *(none)* | 30s | Time allowed between greeting and a successful `<login>` |
| Handshake timeout | *(epp-lib: the read timeout)* | 10s | |
| Max connections per node | *(none)* | 2000 | |
| Concurrent handshakes per node | *(none)* | 200 | |
| TLS minimum version | 1.2 | 1.2 | 1.3 is preferred when the client offers it |
| XML depth / elements | *(none)* | 32 / 2000 | |
| Server certificate | Self-signed, 1 day, generated at boot | From file, hot-reloaded | Self-signed only for local development |
| PROXY protocol, trusted CIDRs | *(off)* | Configurable | Required behind a load balancer, or per-IP caps become global |
| Drain timeout | *(none)* | Configurable | Time allowed to finish in-flight commands on SIGTERM |
| Ops port | *(none)* | Configurable | `/healthz`, `/readyz`, `/metrics` |

## Current limiter: fixed in code today

These values come from `middleware.DefaultRateLimitConfig()` in `internal/infrastructure/epp/middleware/rate_limiter.go`. Phase 3 (sessions) and Phase 5 (rates) delete that limiter, and the policy limits below replace these values. Several have no effect today, as noted.

| Setting | Value | Effect today |
|---|---|---|
| Connections per IP | 10 | Enforced, but counts drift (5-minute TTL on a live count) |
| Connections per registrar | 100 | **None:** the registrar is never recorded on the session |
| Requests per second / burst | 100 / 200 | **None:** never called |
| Failed logins before lock | 5 | Counted per (clID, IP), but the lock is applied to the clID alone |
| Lockout duration | 15m | |

---

## Policy limits *(planned)*

The defaults below form the `default` tier and will be seeded from code constants. A tier can be overridden per registrar. "Family" means a group of affiliated registrars capped together.

### Sessions *(Phase 3)*

| Limit | Default |
|---|---|
| Sessions per registrar | 10 |
| Sessions per EPP login | 10 |
| Sessions per family | 40 |
| Sessions per source IP | 20 |
| Session lease TTL / heartbeat | 90s / 30s |

Over a cap, the server answers 2502 and closes. If Redis is unavailable, enforcement falls back to node-local caps (fail-open) and `epp_policy_store_degraded` is set.

### Authentication *(Phase 2)*

| Limit | Default |
|---|---|
| Failed logins per (login, IP) before lock | 5 within 15m |
| Lock duration | 15m, doubling on each repeat, up to 24h |
| Active passwords per login | 2 |
| Active certificate pins per registrar | 3 |
| Active IP allowlist entries per registrar | 20 |
| Default `retiring` window when retiring a credential | 7 days (overridable per retire; 0 retires at once) |
| Client certificate validity | Enforced: no grace before `notBefore` or after `notAfter` |
| Certificate expiry alerts | 30 and 7 days before expiry |
| `<newPW>` | Strict RFC 5730: replaces the password used to log in, effective immediately. Overlap is done with add-then-retire through the admin API |

### Command rates *(Phase 5)*

Token buckets per registrar. A family gets 3× the registrar rate.

| Class | Rate | Burst | Cost |
|---|---|---|---|
| `check` | 250 names/s | 1000 names | One token **per name** |
| `info` | 20/s | 100 | Per command |
| transform (create, update, delete, renew, transfer) | 10/s | 50 | Per command |
| `poll` | 5/s | 20 | Per command |

`check` is metered in names so that batch size does not change capacity. A higher-volume registrar gets a tier with a larger budget, not a larger batch.

### Per-command object caps *(Phase 5)*

| Cap | Default | On breach |
|---|---|---|
| Names per `domain:check`, `host:check`, `contact:check` | 50 | 2306 |
| Nameservers per domain | 13 | 2306 |
| DS records per domain | 8 | 2306 |

### Quotas *(Phase 5)*

| Quota | Default |
|---|---|
| Creates per day, `probation` tier | 100 |

### Failed creates *(Phase 6)*

| Setting | Default |
|---|---|
| Trigger | At least 100 `domain:create` results of 2302 in 5 minutes **and** a failure ratio of at least 90% |
| Escalation | Tarpit (+1s per command, rising) → 2502 → login suspended for 15m, with an alert |
| Accounting | Daily failed-create count per registrar, kept as the basis for a future fee |
