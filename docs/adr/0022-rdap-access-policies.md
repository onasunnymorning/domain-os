# ADR 0022 — RDAP access policies: low, named limits per network, on the EPP limiter

- **Status:** Accepted
- **Date:** 2026-10-26
- **Deciders:** Platform / Registry
- **Composes with:** [ADR 0013](0013-epp-server-architecture.md) (fail open), [ADR 0018](0018-where-configuration-lives.md) (policy is data, node limits are environment), [ADR 0019](0019-epp-policies.md) (the policy model and GCRA limiter reused here), [ADR 0020](0020-rdap-server-architecture.md) (the server)
- **Roadmap:** Phase 4 of [`docs/rdap/ARCHITECTURE.md`](../rdap/ARCHITECTURE.md); every setting in [`docs/rdap/POLICY_REFERENCE.md`](../rdap/POLICY_REFERENCE.md)

## Context

RDAP is public, unauthenticated and earns nothing, but its availability and latency are SLA terms. Two kinds of traffic matter most:

- **Abuse:** bulk harvesting, and enumeration by guessing names.
- **Monitoring:** ICANN's SLA probes and the RST test clients. If these are throttled, the throttling counts against our own SLA.

The registry's exposure should start low and be raised from evidence, without a deploy.

The EPP server solved the same problem for registrars (ADR 0019):

- named policies held in the database and reloaded by every node;
- GCRA buckets in Redis, taken atomically by one script, on Redis's clock;
- a breaker that falls back to per-node buckets when Redis is slow or down, so a cache outage does not become a registry outage.

RDAP has no logged-in identity, so its unit is the client's network, not a registrar.

## Decision

1. **The EPP limiter becomes shared, not copied.**
   - The GCRA script, the fail-open breaker and the per-node fallback move from `internal/infrastructure/epp/ratelimit` to `internal/infrastructure/ratelimit`, with a key prefix per service. (Built: the whole package moved, its failed-create budget with it, which keeps EPP's keys since it is EPP's alone.)
   - EPP moves to the shared package with no change in behaviour.
   - Two copies of a limiter would drift, as the first EPP limiter did.

2. **An RDAP access policy is a named set of limits, assigned to networks.** It follows ADR 0019:
   - Policies live in the database (`rdap_policies`), typed and validated by the entity.
   - Policies are assigned to CIDR ranges (`rdap_policy_assignments`). The most specific range that contains the client wins, and a client in no range has `anonymous`.
   - There are no per-network overrides: a network that needs different limits gets its own policy.
   - Changes go through the admin API under `rdap:policy:admin`, serialised by an advisory lock, with an outbox event carrying before and after.
   - Nodes reload the whole set every `RDAP_POLICY_REFRESH_INTERVAL`, keep the last good set if a load fails, and use the built-in starting limits before the first load.

3. **Two policies are built in**, seeded from code. They can be edited but not deleted:
   - `anonymous`, which every client has unless assigned another;
   - `monitoring`, for ICANN's SLA probes and the RST `rdap.clientACL`. Getting these networks assigned is on the pre-RST and go-live checklists.

4. **Each request takes from two buckets at once, or from none:**
   - the client's own address: an IPv4 /32, or an IPv6 /64, since one IPv6 subscriber often holds a whole /64;
   - its aggregate: an IPv4 /24 or an IPv6 /48, so one client cannot spread across a range.

   A lookup that finds nothing costs more tokens than one that does, so guessing names (enumeration) is slowed more than ordinary use, and a 404 cannot be used to probe cheaply.

5. **Starting limits are deliberately low** and are raised from observed traffic:

   | Policy | Per address | Per aggregate | Cost of a not-found |
   |---|---|---|---|
   | `anonymous` | 2/s, burst 10 | 20/s, burst 60 | 3 tokens |
   | `monitoring` | 50/s, burst 200 | none | 1 token |

6. **A refusal is a 429, never a 503.**
   - The 429 carries `Retry-After` and an RDAP error object naming the limit and the scope (address or aggregate).
   - A 503 would read as an outage to ICANN's monitors and count against the SLA.
   - The request is refused before any database work.

7. **Node limits are environment settings, not policy** (ADR 0018), because they protect the node, not a client's terms:
   - in-flight requests (`RDAP_MAX_IN_FLIGHT`);
   - open connections in total and per address;
   - read, write and idle timeouts;
   - methods other than GET and HEAD are refused.

8. **Fail open, as EPP does.** Without Redis, buckets are kept per node and `rdap_ratelimit_degraded` is set. The cost is that N nodes allow up to N times a limit while Redis is down. That is preferred to refusing everyone.

## Consequences

- RDAP needs Redis in every environment where it is limited. Running without Redis is degraded, not broken.
- The admin API gains RDAP policy endpoints. As for EPP policies, there is no admin UI yet.
- Raising limits is a policy change that takes effect within one refresh interval. The alerts (Phase 5) show refusals per policy and the sources nearest their caps, so the evidence for a raise is in the dashboard.
- A client behind carrier-grade NAT shares its aggregate with its neighbours. If a legitimate network needs more, it gets its own policy.

## Amendments

### 2026-10-26: as built (Phase 4)

- **No per-address connection cap.** Behind PROXY protocol a connection's client address is known only once the header is read, and reading it at accept would let one slow client stall every accept. The request limit already bounds what one address can do, so the node caps its total connections (`RDAP_MAX_CONNECTIONS`) and requests in flight (`RDAP_MAX_IN_FLIGHT`) only.
- **Requests over the in-flight cap are a 503**, with `Retry-After: 1`. Decision 6 keeps 503 out of client limits; this is the node itself being full, which the SLA should see and an alert should fire on.
- **The not-found cost is charged after the answer.** Whether a lookup finds anything is only known once it has run, so a request is admitted on one token and a 404 then takes `notFoundCost - 1` more, owed rather than refused. The debt pushes the client's next requests back.
- **Rates reuse `entities.EPPRate`** (`per_second`, `burst`), the limiter's existing type, rather than a copy.
