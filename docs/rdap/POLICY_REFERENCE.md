# RDAP server: settings and policy reference

This page lists every setting that governs the RDAP server: its default, where it lives, how to change it, and when a change takes effect. When you add, remove or re-default a setting, update this page in the same PR. The design is in [ARCHITECTURE.md](ARCHITECTURE.md). The decisions are in [ADR 0020](../adr/0020-rdap-server-architecture.md), [ADR 0021](../adr/0021-rdap-publication-and-redaction.md) and [ADR 0022](../adr/0022-rdap-access-policies.md).

Every value on this page is *(planned)*. The phase that introduces each one is named in [ARCHITECTURE.md § Roadmap](ARCHITECTURE.md#roadmap).

Settings live in one of two places ([ADR 0018](../adr/0018-where-configuration-lives.md)):

| Kind | Lives in | How to change | Takes effect |
|---|---|---|---|
| **Server settings**: how a node listens, terminates TLS, times out and protects itself | Environment variables, read once at start by `internal/config/rdap.go` (INV-17), registered in `internal/config/env_registry.go` and published in `deploy/contract.json` | The deployment's environment | On restart. A bad value stops the server at start |
| **Policies**: the limits a network gets, and what each TLD publishes | Database: access policies assigned to networks ([§ Access policies](#access-policies)); publication policies assigned to TLDs ([PUBLICATION_REFERENCE.md](PUBLICATION_REFERENCE.md)) | Admin API, under `rdap:policy:admin` | Within `RDAP_POLICY_REFRESH_INTERVAL`, without a restart |

---

## Server settings: environment

| Variable | Default | Meaning | Phase |
|---|---|---|---|
| `ROID_REPOSITORY_ID` | *(required)* | The repository ID appended to every handle; see the EPP reference and [ADR 0017](../adr/0017-roid-repository-id.md) | 1 |
| `RDAP_LISTEN_ADDR` | `:443` | Public HTTPS listener | 1 |
| `RDAP_OPS_ADDR` | `:9443` | Plain-HTTP ops port: `/healthz`, `/readyz`, `/metrics` | 1 |
| `RDAP_SERVER_NAME` | *(required)* | The RDAP hostname. The certificate must cover it | 1 |
| `RDAP_TLS_CERT_FILE`, `RDAP_TLS_KEY_FILE` | *(required)* | The server certificate and key, reloaded on change | 1 |
| `RDAP_TLS_RELOAD_INTERVAL` | `1m` | How often the certificate files are checked | 1 |
| `RDAP_PROXY_TRUSTED_CIDRS` | *(empty)* | Load balancers whose PROXY protocol header is trusted. Without it, every address limit is a limit on the load balancer | 1 |
| `RDAP_BASE_URL` | *(required)* | Public base, e.g. `https://rdap.example/`. Each TLD is served under `{base}{tld}/` | 1 |
| `RDAP_DB_DSN` | the primary | Optional read replica | 1 |
| `RDAP_DB_MAX_OPEN_CONNS` | `20` | Database pool cap per node | 1 |
| `RDAP_READ_TIMEOUT`, `RDAP_WRITE_TIMEOUT`, `RDAP_IDLE_TIMEOUT` | `5s`, `10s`, `60s` | HTTP timeouts | 1 |
| `RDAP_DRAIN_TIMEOUT` | `20s` | How long a SIGTERM waits for in-flight requests | 1 |
| `RDAP_MAX_IN_FLIGHT` | `200` | Requests a node serves at once. Requests over this get a 503 for node protection, which is alerted on | 4 |
| `RDAP_MAX_CONNECTIONS` | `5000` | Open connections per node | 4 |
| `RDAP_MAX_CONNECTIONS_PER_IP` | `20` | Open connections per client address per node | 4 |
| `RDAP_POLICY_REFRESH_INTERVAL` | `30s` | How often a node reloads access and publication policies | 3 |
| `REDIS_*` | as EPP | The limiter's store. Without it the limiter fails open to per-node buckets | 4 |

## Access policies

Named limits assigned to CIDR ranges (ADR 0022). A client in no assigned range has `anonymous`, and the most specific range that contains the client wins.

| Limit | `anonymous` | `monitoring` | Meaning |
|---|---|---|---|
| `per_address` | 2/s, burst 10 | 50/s, burst 200 | Bucket per IPv4 /32 or IPv6 /64 |
| `per_aggregate` | 20/s, burst 60 | none | Bucket per IPv4 /24 or IPv6 /48 |
| `not_found_cost` | 3 | 1 | Tokens a lookup that finds nothing takes |

A refused request gets a 429 with `Retry-After` and an RDAP error naming the limit and the scope.

`monitoring` must hold ICANN's SLA monitoring networks and, for an RST run, RST's `rdap.clientACL`. A throttled probe counts against the SLA.
