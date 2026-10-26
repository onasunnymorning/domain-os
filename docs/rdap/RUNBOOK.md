# RDAP operator runbook

What to do when an RDAP alert fires, and how to carry out the routine operations. Every setting and limit named here is in [POLICY_REFERENCE.md](POLICY_REFERENCE.md).

> **Skeleton.** The server, its alerts and its dashboard do not exist yet. Phase 5 of [ARCHITECTURE.md](ARCHITECTURE.md#roadmap) fills this page in, and a test keeps every alert's `runbook:` link pointing at a section that exists. The sections below are the ones Phase 5 is expected to need.

- **Alerts** will be in `prometheus/rules/rdap.rules.yml`. Severities mean the same as for EPP: `critical` pages someone now, `warning` is looked at the same day, `info` is for a ticket.
- **The dashboard** will be *RDAP server* in Grafana (`grafana/dashboards/rdap.json`).

---

## Alerts

### Availability
*(Phase 5.)* The service is down or not ready, or the 5xx ratio is above threshold. RDAP downtime counts against the monthly SLA allowance, so this is critical.

### SLA latency
*(Phase 5.)* p95 latency is above the 400 ms round-trip time. Check the database pool, the replica's lag and the in-flight cap.

### Abuse
*(Phase 5.)* 429s are spiking, a single address or aggregate is near its cap, or the 404 ratio is climbing (enumeration).

### Stores
*(Phase 5.)* The limiter is degraded (Redis), or a policy refresh is stale.

### Certificates
*(Phase 5.)* The server certificate is near expiry.

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
*(Phase 6.)*
