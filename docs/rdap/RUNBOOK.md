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
*(Phase 4.)*

### Put a monitoring network on the `monitoring` policy
*(Phase 4.)* Do this for ICANN's SLA probes, and for RST's `rdap.clientACL` before a test run.

### Change what a TLD publishes
*(Phase 3.)* See [PUBLICATION_REFERENCE.md](PUBLICATION_REFERENCE.md).

### Rotate the certificate
*(Phase 1.)*

### Drain a node
*(Phase 1.)*

### Load test
*(Phase 6.)*
