export const EVENT_CONSUMER_DOC_MARKDOWN = `# Event Consumer Cloud

## Overview

The Event Consumer Cloud is the data lifecycle system for \`domain_events\`. It ensures that domain events flow from the hot PostgreSQL tier through a tiered storage pipeline (warm MinIO → cold compressed archive) while keeping the hot database lean and fast.

**The older, the colder** — but always searchable for compliance and audit for at least 5 years.

## Architecture

\`\`\`mermaid
graph TB
    subgraph "Event Producers"
        DS[DomainService]
        RS[RegistrarService]
    end

    subgraph "Hot Tier — PostgreSQL"
        PG[("domain_events<br/>last 30 days")]
    end

    subgraph "Temporal Workflows"
        ER["Event Relay<br/>every 5 min"]
        EP["Event Prune<br/>daily"]
    end

    subgraph "Warm/Cold Tier — MinIO S3"
        S3[("s3://events/archive/<br/>JSONL.gz by day")]
    end

    subgraph "API Layer"
        API["Unified Search API<br/>GET /events/search"]
    end

    DS -->|Publish| PG
    RS -->|Publish| PG

    ER -->|"1. Fetch unpublished"| PG
    ER -->|"2. Archive"| S3
    ER -->|"3. Mark published"| PG

    EP -->|"Delete published<br/>older than 30d"| PG

    API -->|Hot query| PG
    API -->|Warm/Cold query| S3
\`\`\`

## Data Lifecycle

| Tier | Storage | Retention | Format | Use Case |
|------|---------|-----------|--------|----------|
| **Hot** | PostgreSQL | 0-30 days | Rows | Real-time UI queries, domain event timeline |
| **Warm** | MinIO S3 | 30 days - 1 year | JSONL.gz | Date-range compliance searches, audit trail |
| **Cold** | MinIO S3 | 1-5 years | Compressed Parquet | Long-term archival, infrequent lookups |

## Workflows

### Event Relay (Scheduled: every 5 minutes)

The relay is the core of the pipeline. It runs on the \`scheduled\` Temporal task queue.

\`\`\`mermaid
flowchart LR
    A["Scheduled: every 5 min"] --> B["FetchUnpublishedEvents<br/>LIMIT 500"]
    B --> C["ArchiveToMinIO<br/>Write JSONL.gz batch"]
    C --> D["MarkPublished<br/>UPDATE SET published = true"]
    D --> E{"More events?"}
    E -->|Yes| B
    E -->|No| F["Done"]
\`\`\`

| Step | Activity | Timeout | Description |
|------|----------|---------|-------------|
| 1 | \`FetchUnpublishedEvents\` | 5 min | Queries \`domain_events WHERE published = false ORDER BY occurred_at ASC LIMIT 500\` |
| 2 | \`ArchiveEventsToS3\` | 5 min | Marshals events as JSONL, compresses with gzip, uploads to MinIO |
| 3 | \`MarkEventsPublished\` | 5 min | Batch \`UPDATE domain_events SET published = true WHERE id IN (?)\` |
| 4 | \`CountUnpublishedEvents\` | 1 min | Final count for reporting |

**Safety caps**: Max 10 batches per run (5,000 events). If more remain, they're picked up on the next scheduled run.

### Event Prune (Scheduled: daily)

The pruner keeps the hot tier lean by deleting archived events beyond the retention period.

\`\`\`mermaid
flowchart LR
    A["Scheduled: daily"] --> B["Count prunable events<br/>published = true AND<br/>occurred_at < NOW() - 30d"]
    B --> C{"Count > 0?"}
    C -->|Yes| D["Batch DELETE<br/>LIMIT 10,000<br/>(avoid long locks)"]
    D --> C
    C -->|No| E["Done"]
\`\`\`

| Step | Activity | Timeout | Description |
|------|----------|---------|-------------|
| 1 | \`CountPrunableEvents\` | 1 min | Count events where \`published = true AND occurred_at < retention cutoff\` |
| 2 | \`PruneEvents\` | 10 min | Batch deletes using subquery to avoid table locks |

**Safety**: Only deletes events where \`published = true\` (confirmed archived). Uses \`ContinueAsNew\` if max batches reached.

**Configurable**: Default retention is 30 days, adjustable via \`EventPruneParams.RetentionDays\`.

## S3 Archive Layout

\`\`\`
s3://escrow/
  └── events/
      └── archive/
          └── 2026/
              └── 06/
                  └── 29/
                      ├── events-1719648000-500.jsonl.gz
                      ├── events-1719648300-500.jsonl.gz
                      └── events-1719648600-347.jsonl.gz
\`\`\`

- **Partitioned by day** for efficient date-range queries
- **JSONL format**: One JSON object per line, gzip compressed
- **File naming**: \`events-{unix-timestamp}-{count}.jsonl.gz\`
- **Bucket**: Its own bucket, isolated from escrow (configured via \`STORAGE_EVENT_LOGS_BUCKET\` env var — see [Bucket Storage Strategy](/docs/bucket-storage))

## Event Record Schema

Each event contains:

| Field | Type | Description |
|-------|------|-------------|
| \`id\` | string | Unique event identifier |
| \`source\` | string | Service that produced the event (e.g., \`DomainService\`) |
| \`type\` | string | Event type (e.g., \`domain.created\`, \`domain.transferred\`) |
| \`subject\` | string | Domain name the event relates to |
| \`description\` | string | Human-readable description |
| \`time\` | timestamp | When the event occurred |
| \`trace_id\` | string | Distributed tracing correlation |
| \`correlation_id\` | string | Business operation correlation |
| \`data\` | JSON | Event payload (varies by type) |
| \`command\` | JSON | The command that triggered this event |
| \`before_state\` | JSON | Entity state before the change |
| \`after_state\` | JSON | Entity state after the change |
| \`actor\` | string | Who or what triggered the event |

## Technology Choices

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Orchestration | **Temporal** (existing) | Same pattern as lifecycle workflows. No new infra. |
| Archive storage | **MinIO/S3** (existing) | Already deployed for escrow + snapshots. |
| Archive format | **JSONL.gz** → Parquet later | Simple to implement, easy to debug. Upgrade path clear. |
| Message broker | **None** | Temporal handles reliable execution. Single consumer doesn't justify a broker. |
| Search engine | **In-app Go code** | <100 queries/day on warm data. ClickHouse/ES would be over-engineering. |

> **Scaling note**: If query volume on warm/cold data grows beyond ~1K queries/day, **ClickHouse** becomes the natural next step. It can query Parquet files directly from S3 (no ETL needed) and provides sub-second aggregations on billions of rows.

## Bootstrap & Scheduling

Both workflows are registered in the Temporal bootstrap system (\`internal/infrastructure/bootstrap/ensure.go\`). They are automatically created or reconciled on every worker deploy — no manual \`tctl\` commands needed.

| Schedule | Interval | Offset | Catchup Window |
|----------|----------|--------|----------------|
| \`event-relay\` | 5 minutes | 0 | 5 minutes |
| \`event-prune\` | 24 hours | 6 hours | 24 hours |

## Related Files

### Workflows
- \`internal/application/workflows/eventRelay.go\` — Relay workflow
- \`internal/application/workflows/eventPrune.go\` — Prune workflow
- \`internal/application/workflows/eventRelay.doc.md\` — Relay sidecar doc
- \`internal/application/workflows/eventPrune.doc.md\` — Prune sidecar doc

### Activities
- \`internal/application/activities/eventRelayActivities.go\` — All relay + prune activities

### Infrastructure
- \`internal/infrastructure/bootstrap/ensure.go\` — Schedule definitions
- \`internal/infrastructure/db/postgres/domain_event.go\` — Event DB model
- \`internal/infrastructure/storage/s3.go\` — MinIO/S3 client
- \`cmd/workers/unified/main.go\` — Worker registration

### Database
- \`internal/infrastructure/db/postgres/connection.go\` — Index management + cleanup
`;
