export const WORKER_QUEUE_ARCHITECTURE_DOC_MARKDOWN = `
# Worker Queue Architecture

## Overview

domain-os uses [Temporal](https://temporal.io) for workflow orchestration. The system is structured around **task queues** that isolate different workload types — ensuring that a 10-hour escrow import never blocks a 30-second DNS drift check.

This document explains the queue taxonomy, worker configuration rationale, and production tuning guidelines.

---

## Queue Taxonomy

\`\`\`mermaid
graph TB
    subgraph "Queue Taxonomy"
        Q1["⚡ fast-ops\\nLow-latency, short-lived\\nDNS checks, FX updates"]
        Q2["🔄 scheduled\\nPeriodic background work\\nEvent relay, event prune, spec5 sync"]
        Q3["📦 heavy-batch\\nLong-running, resource-intensive\\nEscrow import, snapshots, TLD cleanup"]
        Q4["♻️ lifecycle\\nState-machine transitions\\nExpiry, purge, restore, tombstone backfill"]
    end
\`\`\`

The four queues are:

- **\`fast-ops\`** — Low-latency, short-lived operations. Operator-facing tasks like DNS drift checks and FX rate updates that must complete in seconds to minutes.
- **\`scheduled\`** — Periodic background work. Event relay, event pruning, Spec5 sync, and registrar sync jobs that run on a fixed cadence.
- **\`heavy-batch\`** — Long-running, resource-intensive operations. Escrow imports, full snapshots, seed-from-snapshot, and TLD cleanups that can run for hours.
- **\`lifecycle\`** — State-machine transitions. Domain expiry loops, purge loops, restore workflows, and tombstone backfills that manage domain lifecycle state.

---

## Design Principle: Classify by Workload, Not by Domain

Queues are organized by **workload profile** (latency requirement × activity duration × trigger type) rather than by data origin.

### The anti-pattern

A queue named \`data-pipeline\` that hosts everything from 30-second DNS checks to 10-hour escrow imports. When an escrow import saturates the activity execution slots, DNS checks queue behind it for hours — even though they only need 30 seconds of work.

### The principle

Group workflows by their operational characteristics:
- **How fast must it respond?** (latency requirement)
- **How long does it run?** (activity duration)
- **What triggers it?** (ad-hoc vs. scheduled vs. event-driven)

This ensures that resource limits, poller counts, and scaling policies can be tuned per workload class rather than fighting competing requirements on a single queue.

### Workflow-to-Queue Assignment

| Workflow | Duration | Trigger | Activity Profile | Queue |
|----------|----------|---------|-----------------|-------|
| Serial Drift | ~30s | Ad-hoc / scheduled | DNS queries | \`fast-ops\` |
| UpdateFX | ~1 min | Scheduled (hourly) | HTTP fetch | \`fast-ops\` |
| SyncSpec5 | ~10 min | Scheduled | HTTP + DB | \`scheduled\` |
| EventRelay | ~10 min | Scheduled (every 5 min) | DB + S3 | \`scheduled\` |
| EventPrune | ~10 min | Scheduled (daily) | DB deletes | \`scheduled\` |
| SyncRegistrars | ~5 min | Scheduled (daily) | HTTP + DB | \`scheduled\` |
| Spec5Sweep | ~10 min | Ad-hoc | DB + S3 | \`scheduled\` |
| TombstoneBackfill | ~30 min | Ad-hoc | DB scan + writes | \`lifecycle\` |
| ExpiryLoop | ~30 min | Scheduled (hourly) | DB updates | \`lifecycle\` |
| PurgeLoop | ~30 min | Scheduled (hourly) | DB deletes + tombstones | \`lifecycle\` |
| RestoreWorkflow | ~5 min | Scheduled (4h) | DB updates | \`lifecycle\` |
| Escrow Import | up to 10h | Ad-hoc | S3 → staged DB → bulk ingest | \`heavy-batch\` |
| TLD Cleanup | up to 12h | Ad-hoc | DB bulk delete (5M+ rows) | \`heavy-batch\` |
| Take Snapshot | up to 12h | Ad-hoc | Full DB dump → S3 | \`heavy-batch\` |
| Seed from Snapshot | up to 12h | Ad-hoc | S3 → full DB restore | \`heavy-batch\` |

---

## Worker Configuration

Temporal workers expose several key tuning knobs. Here is what we set for each queue and why:

| Setting | fast-ops | scheduled | heavy-batch | lifecycle |
|---------|----------|-----------|-------------|----------|
| MaxConcurrentWorkflowTaskPollers | 5 | 4 | 3 | 4 |
| MaxConcurrentActivityTaskPollers | 5 | 4 | 3 | 4 |
| MaxConcurrentActivityExecutionSize | 50 | 20 | 5 | 30 |

### Why these values?

- **fast-ops needs more pollers.** These are operator-facing, latency-sensitive operations. More pollers means a higher chance of a "sync match" — Temporal handing the task directly to a waiting poller instead of queuing it. With 5 pollers, the probability that at least one is idle and waiting is much higher than with the default of 2.

- **heavy-batch needs fewer execution slots.** Each activity in this queue runs for hours and consumes significant database and S3 I/O. Limiting to 5 concurrent activities prevents resource exhaustion — an escrow import doing bulk INSERT operations alongside a TLD cleanup doing bulk DELETEs would saturate the database connection pool.

- **lifecycle is balanced.** These workflows run on a steady hourly cadence with moderate activity durations (~30 minutes). 30 execution slots and 4 pollers provide enough headroom for overlapping runs without overcommitting resources.

---

## Poller Mechanics

Understanding how Temporal's long-polling works is essential for diagnosing latency issues.

### How it works

1. Workers open **long-poll connections** to the Temporal server. Each connection blocks for up to 60 seconds waiting for a task.
2. When a task arrives, Temporal tries a **sync match** — handing the task directly to a waiting poller without queuing.
3. If **no poller is waiting** (all are in active polls that haven't returned yet), the task enters the queue and waits up to 60 seconds for a poll connection to expire and re-poll.
4. With only **2 pollers** (the default), both can easily be in-flight simultaneously. A task arriving in that window waits up to 60 seconds before being picked up.
5. **Heartbeat timeouts start counting from schedule time, not start time.** If a task sits in the queue for 55 seconds and has a 60-second heartbeat timeout, the activity has only 5 seconds to heartbeat before timing out — even though it just started executing.

### Practical impact

With the default 2 pollers, you can see schedule-to-start latencies of 60+ seconds on a queue that appears idle. Increasing to 5 pollers dramatically reduces this because the probability of all 5 being simultaneously in-flight is much lower.

---

## Deployment Topology

The system uses a **two-unit deployment model** to optimize cost and resource allocation:

\`\`\`mermaid
graph LR
    subgraph "Deploy Unit A - Always-on"
        W1["Worker: fast-ops"]
        W2["Worker: scheduled"]
        W3["Worker: lifecycle"]
    end
    subgraph "Deploy Unit B - Scale to 0"
        W4["Worker: heavy-batch"]
    end
    W1 -->|polls| TQ1["fast-ops queue"]
    W2 -->|polls| TQ2["scheduled queue"]
    W3 -->|polls| TQ3["lifecycle queue"]
    W4 -->|polls| TQ4["heavy-batch queue"]
\`\`\`

### Unit A — Always-on (1 replica)

Hosts the \`fast-ops\`, \`scheduled\`, and \`lifecycle\` workers. These queues have continuous work (scheduled workflows, periodic lifecycle sweeps) and need to be responsive at all times.

### Unit B — Scale to zero

Hosts the \`heavy-batch\` worker. Escrow imports, snapshots, and TLD cleanups are rare (a few times per week at most) but extremely resource-intensive. Keeping this worker at zero replicas until needed saves compute costs while ensuring heavy operations get dedicated resources when they run.

---

## Key Files

| File | Purpose |
|------|---------|
| \`internal/infrastructure/temporal/queues.go\` | Queue name constants (single source of truth) |
| \`internal/infrastructure/temporal/config.go\` | Client config from env vars |
| \`internal/infrastructure/temporal/temporal.go\` | Temporal client creation (API key, mTLS, or local) |
| \`cmd/workers/unified/main.go\` | Unified worker binary — registers workflows + activities per queue |
| \`internal/infrastructure/bootstrap/ensure.go\` | Schedule reconciliation on startup |
| \`internal/application/workflows/workflow_registry.go\` | Workflow metadata registry (queue assignment per workflow) |

---

## Troubleshooting: High Schedule-to-Start Latency

If workflows or activities are slow to start, work through this checklist:

### 1. Check poller count

\`worker.Options{}\` defaults to **2 pollers**. For latency-sensitive queues, increase to 5+. Low poller counts are the #1 cause of unexplained schedule-to-start delays.

### 2. Check queue isolation

Are short-lived and long-lived activities sharing the same queue? A 10-hour escrow import holding an activity execution slot can block a 30-second DNS check if they share a queue with limited slots. Separate them.

### 3. Check heartbeat timeout

If activities fail with a **heartbeat timeout on attempt 1** but succeed on attempt 2+, the issue is queue delay, not activity failure. The heartbeat clock starts at **schedule time**, so a 60-second queue delay eats into the heartbeat budget before the activity even begins executing.

### 4. Check sticky queue timeouts

A \`WORKFLOW_TASK_TIMED_OUT\` with \`TIMEOUT_TYPE_SCHEDULE_TO_START\` on a sticky queue means the worker's workflow cache expired. The workflow task gets kicked back to the normal queue, adding another full poll cycle (~60 seconds) on top of the original delay.

### 5. Check activity execution slots

If \`MaxConcurrentActivityExecutionSize\` is exhausted, the worker **stops polling** for new activity tasks. Long-running activities (like escrow import) can hold slots for hours, effectively blocking the entire queue. This is why heavy-batch has a separate queue with only 5 slots — it prevents slot exhaustion from cascading to other queues.
`;
