# RDAP server: settings and policy reference

This page lists every setting that governs the RDAP server: its default, where it lives, how to change it, and when a change takes effect. When you add, remove or re-default a setting, update this page in the same PR. The design is in [ARCHITECTURE.md](ARCHITECTURE.md). The decisions are in [ADR 0020](../adr/0020-rdap-server-architecture.md), [ADR 0021](../adr/0021-rdap-publication-and-redaction.md) and [ADR 0022](../adr/0022-rdap-access-policies.md).

A value marked *(planned)* does not exist yet. The phase that introduces it is named in [ARCHITECTURE.md § Roadmap](ARCHITECTURE.md#roadmap).

Settings live in one of two places ([ADR 0018](../adr/0018-where-configuration-lives.md)):

| Kind | Lives in | How to change | Takes effect |
|---|---|---|---|
| **Server settings**: how a node listens, terminates TLS, times out and protects itself | Environment variables, read once at start by `internal/config/rdap.go` (INV-17), registered in `internal/config/env_registry.go` and published in `deploy/contract.json` | The deployment's environment | On restart. A bad value stops the server at start |
| **Policies**: the limits a network gets, and what each TLD publishes | Database: access policies assigned to networks ([§ Access policies](#access-policies)); publication policies assigned to TLDs ([PUBLICATION_REFERENCE.md](PUBLICATION_REFERENCE.md)) | Admin API, under `rdap:policy:admin` | Within `RDAP_POLICY_REFRESH_INTERVAL`, without a restart |

---

## Server settings: environment

Durations use Go syntax (`30s`, `10m`). Every value is checked at start. A bad value, or an inconsistent combination, stops the server and lists every problem at once.

| Variable | Default | Meaning | Phase |
|---|---|---|---|
| `ROID_REPOSITORY_ID` | *(required)* | The repository ID appended to every handle. See the EPP reference and [ADR 0017](../adr/0017-roid-repository-id.md) | 1 |
| `RDAP_PORT` | `443` | HTTPS listen port. RST requires base URLs on 443. The local stack uses `8443` because the image runs as a non-root user | 1 |
| `RDAP_OPS_PORT` | `9701` | Plain-HTTP ops port: `/healthz`, `/readyz`, `/metrics`. Never expose it publicly | 1 |
| `RDAP_SERVER_NAME` | *(set in every deployment)* | The RDAP hostname. A certificate from file that does not cover it is refused at start and on reload (rdap-91) | 1 |
| `RDAP_BASE_URL` | `https://RDAP_SERVER_NAME/` | Public base, as `https://host/path/`. Each TLD is served under `{base}{tld}/`, which is what RST's `rdap.baseURLs` lists. One of this and `RDAP_SERVER_NAME` is required | 1 |
| `RDAP_TLS_CERT_PATH`, `RDAP_TLS_KEY_PATH` | *(required)* | The server certificate (with chain) and key, reloaded when either file changes | 1 |
| `RDAP_TLS_SELF_SIGNED` | `false` | Present a generated certificate instead. Local development only | 1 |
| `RDAP_TLS_RELOAD_INTERVAL` | `1m` | How often the certificate files are checked | 1 |
| `RDAP_PROXY_PROTOCOL` | `false` | Require a PROXY protocol header on every connection. Must be `true` behind a load balancer | 1 |
| `RDAP_PROXY_TRUSTED_CIDRS` | *(required with the above)* | Load balancers whose PROXY header is trusted. Connections from anywhere else are dropped | 1 |
| `RDAP_READ_TIMEOUT` | `5s` | Reading a request, including any PROXY header | 1 |
| `RDAP_WRITE_TIMEOUT` | `10s` | Writing a response | 1 |
| `RDAP_IDLE_TIMEOUT` | `60s` | A keep-alive connection waiting for its next request | 1 |
| `RDAP_DRAIN_TIMEOUT` | `20s` | On SIGTERM, how long requests in flight may finish after readiness turns 503 | 1 |
| `RDAP_TERMS_OF_SERVICE_URL` | *(unset)* | The registry's RDAP terms of use, linked from the Terms of Use notice. Unset leaves the notice out, which the gTLD profile does not allow, and logs a warning at start. Moves into each TLD's publication policy in Phase 3 | 1 |
| `RDAP_DB_MAX_OPEN_CONNS` | `20` | Database pool cap per node | 1 |
| `DB_*`, `SCHEMA_GUARD`, `LOG_LEVEL` | as the other services | Database connection, schema check and log level | 1 |
| `RDAP_MAX_IN_FLIGHT` | `200` *(planned)* | Requests a node serves at once, to protect the node | 4 |
| `RDAP_MAX_CONNECTIONS` | `5000` *(planned)* | Open connections per node | 4 |
| `RDAP_MAX_CONNECTIONS_PER_IP` | `20` *(planned)* | Open connections per client address per node | 4 |
| `RDAP_POLICY_REFRESH_INTERVAL` | `30s` *(planned)* | How often a node reloads access and publication policies | 3 |
| `REDIS_*` | as EPP *(planned)* | The limiter's store. Without it the limiter fails open to per-node buckets | 4 |

A read replica for lookups is not offered yet. It is added if load testing (Phase 6) shows the primary is the bottleneck.

## Access policies

Named limits assigned to CIDR ranges (ADR 0022). A client in no assigned range has `anonymous`, and the most specific range that contains the client wins.

| Limit | `anonymous` | `monitoring` | Meaning |
|---|---|---|---|
| `per_address` | 2/s, burst 10 | 50/s, burst 200 | Bucket per IPv4 /32 or IPv6 /64 |
| `per_aggregate` | 20/s, burst 60 | none | Bucket per IPv4 /24 or IPv6 /48 |
| `not_found_cost` | 3 | 1 | Tokens a lookup that finds nothing takes |

A refused request gets a 429 with `Retry-After` and an RDAP error naming the limit and the scope.

`monitoring` must hold ICANN's SLA monitoring networks and, for an RST run, RST's `rdap.clientACL`. A throttled probe counts against the SLA.
