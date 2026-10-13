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

Durations use Go syntax (`30s`, `10m`, `24h`). Every value is checked at start. A bad value, or an inconsistent combination, stops the server and lists every problem at once.

**Listening and TLS**

| Variable | Default | Meaning |
|---|---|---|
| `EPP_PORT` | `700` | TCP port the EPP listener binds |
| `EPP_OPS_PORT` | `9700` | Plain-HTTP port for `/healthz` (process up), `/readyz` (accepting sessions; 503 while starting or draining) and `/metrics`. Keep it on the internal network |
| `EPP_TLS_CERT_PATH` | *(required)* | PEM server certificate, with its chain, presented to registrars |
| `EPP_TLS_KEY_PATH` | *(required)* | PEM private key for that certificate |
| `EPP_TLS_SELF_SIGNED` | `false` | Present a generated one-day self-signed certificate instead. **Local development only.** Conflicts with the two file variables. Docker Compose and Tilt set it |
| `EPP_TLS_RELOAD_INTERVAL` | `1m` | How often the certificate and key files are checked for a change |
| `EPP_SERVER_NAME` | *(unset)* | The hostname registrars connect to (RST's `epp.hostName`). When set, a certificate that does not cover it in a subjectAltName (exact or wildcard) is refused. **Set it in every deployment** |
| `EPP_PROXY_PROTOCOL` | `false` | Require a PROXY protocol (v1 or v2) header on every connection. **Must be `true` behind a load balancer**, or every per-IP control sees only the balancer's address |
| `EPP_PROXY_TRUSTED_CIDRS` | *(required when the above is true)* | Comma-separated CIDRs or IPs of the load balancers. A connection from anywhere else is dropped at accept |

Fixed in code (RST epp-01):
- **TLS versions:** 1.2 minimum; 1.3 is used when the client offers it.
- **TLS 1.2 cipher suites:** only the four RFC 9325 §4.2 recommends, ECDHE with AES-GCM for RSA and ECDSA keys. TLS 1.3 suites are all AEAD and not configurable in Go.
- **Certificate checks:** a certificate from file that is expired or not yet valid is refused, at start and on reload. So is one that misses `EPP_SERVER_NAME`. At start the server will not run; on reload it keeps the current certificate. The certificate file must contain the intermediates after the leaf, because clients are given exactly what is in the file.

**Rolling the server certificate.** Replace the certificate and key files in place; the server picks them up within `EPP_TLS_RELOAD_INTERVAL`:
- New handshakes get the new certificate. Sessions already open keep the one they negotiated, so nothing is dropped.
- A file that is missing, does not parse, or fails the checks above is ignored, and the current certificate stays in use. Each failure increments `epp_server_cert_reload_errors_total`, so a half-written renewal cannot take the server down.
- Write the key before the certificate. A certificate that does not match its key is treated as a failure and retried on the next check.
- `epp_server_cert_not_after_timestamp_seconds` exposes the expiry of the certificate in use.

**Per-node limits.** These hold without Redis, and each node enforces its own.

| Variable | Default | Meaning |
|---|---|---|
| `EPP_MAX_CONNECTIONS` | `2000` | Open connections on one node. Above this, new connections are closed at accept |
| `EPP_MAX_HANDSHAKES` | `200` | TLS handshakes in progress on one node. Above this, new connections are closed at accept. Must not exceed `EPP_MAX_CONNECTIONS` |
| `EPP_MAX_FRAME_BYTES` | `65536` | Largest frame accepted, including its 4-byte length header (minimum 4096). A create with 13 NS, 8 DS and extensions is about 8 KB, and a 50-name check is at most about 15 KB. A larger frame **closes the session**: the rest of it cannot be skipped safely |
| `EPP_XML_MAX_DEPTH` | `32` | Deepest element nesting. A deeper frame is answered 2001 and the session continues |
| `EPP_XML_MAX_ELEMENTS` | `2000` | Most elements in a frame. A larger one is answered 2001 and the session continues |

A frame containing a `<!DOCTYPE>` or any other `<!...>` declaration is always answered 2001. So is malformed XML, a root other than `<epp>`, a non-UTF-8 encoding, and a `clTRID` outside 3–64 characters. None of these close the session.

**Timeouts**

| Variable | Default | Meaning |
|---|---|---|
| `EPP_HANDSHAKE_TIMEOUT` | `10s` | TCP accept to completed TLS handshake, including reading any PROXY header |
| `EPP_LOGIN_TIMEOUT` | `30s` | Greeting to a successful `<login>`. A command already running finishes first |
| `EPP_IDLE_TIMEOUT` | `10m` | Waiting between commands |
| `EPP_READ_TIMEOUT` | `30s` | Reading one frame once its header has arrived |
| `EPP_WRITE_TIMEOUT` | `60s` | Writing one response |
| `EPP_MAX_SESSION_DURATION` | `24h` | Longest a session may stay open |
| `EPP_DRAIN_TIMEOUT` | `30s` | On SIGTERM, time for in-flight commands to finish. Idle sessions close at once, and sessions still open at the deadline are closed |

**Authentication**

| Variable | Default | Meaning |
|---|---|---|
| `EPP_REQUIRE_ALLOWLIST` | `true` | Drop a connection from outside **every** registrar's allowed networks before the TLS handshake, and refuse a login from outside **its own** registrar's. `false` only for local development, where Docker's NAT hides the client address. Docker Compose and Tilt set it |
| `EPP_CREDENTIAL_REFRESH_INTERVAL` | `30s` | How often a node reloads the union of allowed networks, and ends sessions that used a credential retired as compromised. A new allowed network can take this long to admit its first connection; a compromise retire can take this long to end sessions. Logins themselves always read the database |
| `EPP_MAX_CONCURRENT_LOGINS` | `8` | Password verifications in progress on one node. Each Argon2id verification holds 19 MiB, so this bounds login memory at about 150 MiB. More logins wait their turn |
| `EPP_MAX_LOGIN_ATTEMPTS` | `3` | Failed logins one session may make. The last is answered 2501 and the session closes |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASS`, `DB_NAME`, `DB_SSLMODE` | as for the other services | The database holding registrars' credentials. A node uses at most 10 connections. `SCHEMA_GUARD` applies as for the API |

**Shared state and logging**

| Variable | Default | Meaning |
|---|---|---|
| `REDIS_HOST` | `localhost` | Redis holding the session registry and the lockouts. A single Redis (or one hash slot): the registry's scripts touch several keys at once. The admin API reads the same Redis to list and end sessions |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | *(empty: no auth)* | Redis password. Secret |
| `LOG_LEVEL` | `info` | `debug`, `info`, `warn` (or `warning`), or `error`, in any case. `debug` logs every command with its clTRID, svTRID, result code and duration |

## Credentials: admin API

A registrar's EPP logins, passwords, certificate pins and allowed networks are managed through the admin API, under `/registrars/{clid}/epp`. Every endpoint needs the `epp:credentials:admin` permission. Registrars are not owned by an operator, so this is a platform permission; with Auth0 disabled, the static admin token holds it. The model is [ARCHITECTURE.md §2](ARCHITECTURE.md#2-identity--credentials) and the decisions are [ADR 0014](../adr/0014-epp-credential-rotation.md).

| Action | Request |
|---|---|
| Show everything, with each credential's state now (never a password hash) | `GET /registrars/{clid}/epp` |
| Add a login. `role` is `transform` (default) or `readonly` | `POST /registrars/{clid}/epp/logins` `{"login_id": "...", "role": "..."}` |
| Change a login's `role`, `enabled` or `session_cap` | `PATCH /registrars/{clid}/epp/logins/{login}` |
| Add a password. With no body the server generates a 16-character password and returns it **once**; it cannot be retrieved again | `POST /registrars/{clid}/epp/logins/{login}/passwords` `{"password": "..."}` |
| Pin a client certificate's key. `logins` optionally restricts the pin | `POST /registrars/{clid}/epp/certificate-pins` `{"certificate_pem": "...", "logins": [...]}` |
| Allow a source network | `POST /registrars/{clid}/epp/allowed-networks` `{"network": "192.0.2.0/24", "note": "..."}` |
| Retire a password, pin or network | `POST .../passwords/{id}/retire`, `.../certificate-pins/{id}/retire`, `.../allowed-networks/{id}/retire` `{"kind": "normal" or "compromise", "at": "RFC 3339"}` |

Every change is written in one transaction with an outbox event (`epp.login.created`, `epp.password.added`, `epp.certificate_pin.retired`, and so on; subject: the registrar's ClID). Events never carry a password or a hash. Delivering them to the registrar's technical contact needs an outbox consumer that does not exist yet.

These rules are fixed in code (`pkg/domain/entities/eppRegistrarAccess.go`) until the policy engine makes them per-tier (Phase 5):

| Rule | Value |
|---|---|
| Logins per registrar | 10, enabled or not |
| Login ID | 3 to 16 ASCII characters (`clIDType`), unique across registrars. It may be the registrar's own ClID, never another registrar's |
| Session cap per login | 0 (the policy default) to 100 |
| Usable passwords per login | 2. Retire one to add a third |
| Password | 6 to 16 printable ASCII characters (RFC 5730 `pwType`), no leading, trailing or repeated spaces. Stored as Argon2id (19 MiB, 2 passes, 1 lane). Prefer the generated password: 16 characters, about 92 bits |
| Usable certificate pins per registrar | 3 (current, next, and a spare for a staggered roll) |
| Pinned certificate | Any CA or self-signed. RSA keys of at least 2048 bits, ECDSA or Ed25519. An expired certificate is refused; one not yet valid is accepted, so the pin is ready before the switch |
| Usable allowed networks per registrar | 20 |
| Allowed network | A single address, or a CIDR no broader than /24 (IPv4) or /48 (IPv6) |
| Normal retire without `at` | Usable for 7 more days, shown as `retiring`. An `at` that is not in the future retires at once. New logins are refused from then on; open sessions run until they end |
| Compromise retire | At once, and every session that used the credential is ended. Also allowed on an already retired credential, which keeps its retire time |

A credential is never deleted. A retired one stays as the record of when, and from where, it was last used.

---

## Policy limits *(planned)*

The defaults below form the `default` tier and will be seeded from code constants. A tier can be overridden per registrar. "Family" means a group of affiliated registrars capped together.

### Sessions

In place (Phase 3), as server settings; the policy engine (Phase 5) makes the caps per tier and adds the per-family cap.

| Variable | Default | Meaning |
|---|---|---|
| `EPP_MAX_SESSIONS_PER_SOURCE_IP` | `20` | Live sessions from one source address, across every node. Over it, the connection closes before the greeting (`epp_connections_total{outcome="rejected_source_cap"}`) |
| `EPP_MAX_SESSIONS_PER_REGISTRAR` | `10` | Logged-in sessions per registrar, across every node and login. Over it, `<login>` is answered 2502 and the session closes |
| `EPP_MAX_SESSIONS_PER_LOGIN` | `10` | Logged-in sessions per login, across every node. A login's own `session_cap` ([Credentials](#credentials-admin-api)) overrides it. Over it, 2502 |
| `EPP_SESSION_LEASE` | `90s` | How long a session stays in the registry without a renewal. A crashed node's sessions disappear after this. Lease times use Redis's clock, not the nodes', so clock skew between nodes does not matter. A closed session cannot be renewed back by a heartbeat that read it just before it closed |
| `EPP_SESSION_HEARTBEAT` | `30s` | How often a node renews its sessions. At most half the lease, so one missed renewal never drops a live session. The registry's command count and last command time are this fresh |
| `EPP_NODE_ID` | the hostname | The node's name in the registry |
| Sessions per family | 40, *(Phase 5)* | Needs registrar families |

**If Redis cannot answer** (each call is bounded to 500 ms), the node applies the same caps to its own sessions and sets `epp_policy_store_degraded{store="sessions"}`. A node's own count is a backstop, not the cluster's: with N nodes, up to N times a cap can be open until Redis returns. When it does, each node puts its sessions back within one heartbeat.

**Admin API** (permission `epp:sessions:admin`; the static admin token holds it when Auth0 is off):

| Action | Request |
|---|---|
| List live sessions, with node, source, registrar, login, credential IDs, connect and login times, command count and last command time | `GET /epp/sessions?clid=&login=&client_ip=&credential_id=`, cursor-paginated |
| One session | `GET /epp/sessions/{id}` |
| End a session | `DELETE /epp/sessions/{id}` → 202 |
| End every session of a registrar, login or source, or that used a credential | `DELETE /epp/sessions?clid=` (or `login`, `client_ip`, `credential_id`) → 202. At least one filter is required |

Ending a session first records an `epp.session.kill_requested` outbox event per session, with the operator as actor; if that cannot be written, nothing is sent and the request fails. Then it publishes a request every node receives, and the node holding the session closes it within moments (`epp_sessions_ended_total{reason="admin_kill"}`). The answer is 202: a node that cannot reach Redis at that instant misses the request. A compromise retire of a credential ends its sessions on its own, within `EPP_CREDENTIAL_REFRESH_INTERVAL`; ending them by `credential_id` is immediate.

### Authentication

In place. A `<login>` is checked in this order, against the database, so an admin API change applies to the next login on every node:

1. The login exists and is enabled.
2. The source address is in one of **its registrar's** usable allowed networks (unless `EPP_REQUIRE_ALLOWLIST=false`).
3. The client certificate's key is pinned by the registrar, for this login, and the certificate is within its validity period.
4. The (login, source address) pair is not locked out.
5. The password matches one of the login's usable passwords.

Every refusal is answered 2200 with the same text, so a client cannot tell which check failed; `epp_login_failures_total{reason}` and the log say which. Only a wrong password at step 5 counts towards a lockout: by then the client has shown an allowed address and a pinned certificate, so a stranger cannot lock a registrar out.

| Rule | Value | Where to change it |
|---|---|---|
| Failed logins per (login, source address) before lock | 5 within 15m. Failures that arrive while locked are not counted, so a burst cannot start the next, doubled lock | `lockout.DefaultPolicy` (code) until Phase 5 |
| Lock duration | 15m, doubling on each further lock within 24h, up to 24h. A successful login clears it | `lockout.DefaultPolicy` (code) until Phase 5 |
| While locked | 2501, and the session closes, even with the right password | |
| Failed logins per session | 3, then 2501 and the session closes | `EPP_MAX_LOGIN_ATTEMPTS` |
| Redis unavailable | Lockouts are counted on the node (fail-open, ADR 0013), with each Redis call bounded to 500 ms, and `epp_policy_store_degraded{store="lockout"}` is 1. The node-local count holds at most 100,000 (login, address) pairs and never drops a lock in force to make room. Redis being down at start is a warning, not a failure | |
| Credential store unavailable | 2400; the session stays open and may retry. Open sessions are unaffected | |
| Client certificate validity | Enforced at every login: no grace before `notBefore` or after `notAfter` | |
| Certificate expiry alerts | 30 and 7 days before expiry, from `epp_client_cert_not_after_timestamp_seconds{clid,login}` (alert rules: Phase 7) | |
| `<newPW>` | Strict RFC 5730: the new password replaces the one used to log in, which stops working at once. An unacceptable `<newPW>` is 2306 and the login does not happen. Overlap is done with add-then-retire through the admin API | |
| Last used | Recorded for the password, pin and network of each successful login, written at most once a minute per node | |
| Compromise retire | Every session that authenticated with the credential, or set it through `<newPW>`, is ended within `EPP_CREDENTIAL_REFRESH_INTERVAL`. An already retired credential, such as a password replaced by `<newPW>`, can still be retired as compromised | |

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
