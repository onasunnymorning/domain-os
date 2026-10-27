# RDAP operator runbook

What to do when an RDAP alert fires, and how to carry out the routine operations. Every setting and limit named here is in [POLICY_REFERENCE.md](POLICY_REFERENCE.md).

- **Alerts** are in [`prometheus/rules/rdap.rules.yml`](../../prometheus/rules/rdap.rules.yml), with promtool unit tests run by `make alerts-check`. Each links to its section below. `critical` pages someone now, `warning` is looked at the same day, `info` is for a ticket. RDAP availability and round-trip time are SLA terms that ICANN measures, so the availability and latency alerts page.
- **The dashboard** is *RDAP server* in Grafana ([`grafana/dashboards/rdap.json`](../../grafana/dashboards/rdap.json)), provisioned with the local stack.
- **Admin API calls** below need `rdap:policy:admin`. `$API` is the admin API.
- A test (`internal/interface/rdap/ops_assets_test.go`) fails if a rule or panel names a metric the server does not serve, or a rule links to a section of this page that does not exist.

---

## Alerts

### RDAP server down
*`RDAPServerDown`, critical.* Prometheus cannot scrape a node's ops port. Every minute a TLD's RDAP is unreachable counts against the monthly downtime allowance.
1. Is the process running? Check the orchestrator and the node's logs for the reason it stopped. A bad setting stops it at start with every problem listed.
2. If the node runs but `/readyz` is 503, it is draining (`rdap_draining` is 1) or has not started serving.
3. Other nodes keep serving if the load balancer health-checks `/readyz`; confirm they have the capacity (`rdap_requests_in_flight`, `rdap_shed_total`).

### RDAP server errors
*`RDAPServerErrors`, critical; `RDAPLookupsFailing`, warning.* Requests are answered 500 (a lookup could not read the registry) or 503 (the node is shedding load).
1. `rdap_lookup_errors_total` by route, and the node's logs ("RDAP lookup failed reading the registry"), say whether the database is the cause: connectivity, `RDAP_DB_MAX_OPEN_CONNS` exhausted, or slow queries.
2. If it is 503s, see [The node is shedding load](#the-node-is-shedding-load).

### Latency over the SLA
*`RDAPLatencyOverSLA`, critical.* The 95th percentile response time is over 400 ms, the SLA's round-trip limit.
1. *p95 by route* on the dashboard: one route, or all?
2. All routes: the database (pool saturation, slow queries) or the node (CPU, `rdap_requests_in_flight` near `RDAP_MAX_IN_FLIGHT`). Add nodes, or raise the pool if the database has room.
3. One route: a slow query for that object type; check the lookup's plan.

### The node is shedding load
*`RDAPShedding`, warning.* A node refused requests over `RDAP_MAX_IN_FLIGHT` (503) or connections over `RDAP_MAX_CONNECTIONS`.
1. Shedding means the node is full, not that a client is abusive: client limits answer 429 before this point. Add nodes or raise the caps if the node has CPU and database headroom.
2. If one network drives the load, give it a stricter access policy ([Change a network's limits](#change-a-networks-limits)).

### Monitoring is being throttled
*`RDAPMonitoringThrottled`, critical.* A client on the `monitoring` policy (ICANN's SLA probes, RST's test clients) was refused. A throttled probe is an SLA miss the registry caused.
1. Raise the `monitoring` policy's limits at once: `PUT $API/rdap/access-policies/monitoring`.
2. Check which scope refused it on the dashboard. An `address` refusal means the probe is busier than the policy allows; there is no aggregate limit on `monitoring` by default.

### Clients are being refused
*`RDAPClientsRefused`, info.* More than one request a second is refused at a limit, for 15 minutes. Refusals are the limits working; this alert is the evidence for whether to raise them.
1. *Refused by policy and scope*: a few networks, or broad? A few busy networks can be given their own policy; broad refusals under `anonymous` may mean its limits are too low for real use.
2. Raise limits from this evidence, not by default ([Change a network's limits](#change-a-networks-limits)).

### Enumeration
*`RDAPEnumeration`, info.* Most lookups find nothing, at volume: someone is guessing names. Each miss already costs `notFoundCost` tokens.
1. Find the networks on the dashboard (refusals by scope) or in the load balancer's logs.
2. Give persistent sources a stricter policy, with a higher `notFoundCost`.

### The rate limiter is degraded
*`RDAPRateLimitDegraded`, warning.* A node cannot reach Redis and limits clients with its own buckets (fail open, ADR 0022). Clients get up to one limit per node instead of one in total.
1. Check Redis: reachability from the node, memory, `REDIS_*` settings.
2. Nothing else needs doing; the node returns to Redis on its own within seconds of it answering.

### Policies are not refreshing
*`RDAPPoliciesStale`, warning.* A node has not reloaded the publication and access policies for over five minutes. It keeps applying the set it last loaded.
1. The node's logs say why ("RDAP policies not refreshed"): usually the database.
2. Changes made through the admin API since the last reload are not applied on that node until it reloads.

## Tasks

### Change a network's limits
Needs `rdap:policy:admin`. Find the policy a network has with `GET $API/rdap/access-networks` (a client in none is `anonymous`). Raise a policy for everyone who has it with `PUT $API/rdap/access-policies/$NAME`, or give one network its own: create a policy, then `PUT $API/rdap/access-networks` with `{"network": "$CIDR", "policy": "$NAME"}`. Nodes apply it within `RDAP_POLICY_REFRESH_INTERVAL`. Raise limits from evidence: `rdap_policy_decisions_total{action="refused"}` by policy and scope.

### Put a monitoring network on the `monitoring` policy
Do this for ICANN's SLA probes, and for RST's `rdap.clientACL` before a test run: `PUT $API/rdap/access-networks` with `{"network": "$CIDR", "policy": "monitoring"}` for each network. A throttled probe counts against the SLA.

### Change what a TLD publishes
Needs `rdap:policy:admin`. Read what applies with `GET $API/tlds/$TLD/rdap/publication-policy`. Either change that policy (`PUT $API/rdap/publication-policies/$NAME`, with every field of every published role), or create one and assign it (`PUT $API/tlds/$TLD/rdap/publication-policy` with `{"policy": "$NAME"}`). Every node applies it within `RDAP_POLICY_REFRESH_INTERVAL`; confirm with `rdap_policies_refreshed_timestamp_seconds`. Record the policy owner's sign-off in [PUBLICATION_REFERENCE.md](PUBLICATION_REFERENCE.md) for a policy that serves a live TLD.

### Rotate the certificate
Replace the files at `RDAP_TLS_CERT_PATH` and `RDAP_TLS_KEY_PATH` (key first, then certificate). Each node picks them up within `RDAP_TLS_RELOAD_INTERVAL`, for new connections only. A certificate that is expired, not yet valid or does not cover `RDAP_SERVER_NAME` is refused: the node keeps the current one, logs the reason, and counts it in `rdap_server_cert_reload_errors_total`. Check `rdap_server_cert_not_after_timestamp_seconds` on every node to confirm the new expiry.

### Drain a node
Send SIGTERM. `/readyz` on the ops port turns 503 and `rdap_draining` turns 1, new connections are refused, and requests in flight get `RDAP_DRAIN_TIMEOUT` to finish. The load balancer's health check must use `/readyz`, not the HTTPS port.

### Load test
`go run ./cmd/tools/rdapload -base https://$HOST/$TLD/ -names names.txt -workers 8 -duration 1m` against a test environment. It reports status codes, latency percentiles and how many requests were over 400 ms. `-miss 0.3` makes 30% of lookups ask for names that do not exist, as an enumerator would; `-rate` paces each worker. From one address the client meets its own access policy first: to measure the server rather than the limiter, assign the load generator's network a generous policy ([Change a network's limits](#change-a-networks-limits)) and remove it afterwards.
