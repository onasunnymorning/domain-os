export const DOMAIN_ARCHIVAL_DOC_MARKDOWN = `# Domain Archival & Lifecycle History

## Overview

When a domain is **purged** (removed from the registry), it historically vanished from the system — events became orphaned, audit trails broke, and operators lost visibility into past domain lifecycles. The Domain Archival system solves this by preserving a **tombstone** record for every purged domain, enabling full lifecycle history, ROID-based event linking, and archive-aware UI navigation.

## Architecture

\`\`\`mermaid
flowchart TB
    subgraph Purge["Domain Purge Flow"]
        PD["PurgeDomain()"] --> CT["Create Tombstone"]
        CT --> DD["Delete Domain"]
        DD --> PE["Emit domain.purged event"]
    end

    subgraph Storage["Tombstone Storage"]
        TB[("domain_tombstones<br/>PostgreSQL")]
    end

    subgraph UI["Frontend"]
        DP["Domain Detail Page"]
        HIS["/domains/{name}/history"]
        ARC["/domains/archive/{roid}"]
        CMD["⌘K Global Search"]
        EF["Event Feed Links"]
    end

    CT --> TB
    TB --> DP
    TB --> HIS
    TB --> ARC
    TB --> CMD
    EF -->|"always clickable"| DP
    DP -->|"404 fallback"| TB
\`\`\`

## Tombstone Entity

A \`DomainTombstone\` captures the essential metadata of a purged domain, keyed by **ROID** to ensure incarnation-specific isolation (the same domain name can be registered, purged, and re-registered multiple times).

| Field | Type | Description |
|-------|------|-------------|
| \`RoID\` | string (PK) | Registry Object Identifier — unique per incarnation |
| \`Name\` | string | Fully qualified domain name (e.g., \`example.com\`) |
| \`UName\` | string | Unicode/IDN form of the domain name |
| \`TLDName\` | string | Top-level domain (e.g., \`com\`) |
| \`RegistrarClID\` | string | Last registrar of record |
| \`RegisteredAt\` | time | When this incarnation was first registered |
| \`ExpiredAt\` | time (nullable) | When the domain expired (if applicable) |
| \`PurgedAt\` | time | When the domain was purged from the registry |
| \`PurgeReason\` | string | Why the domain was purged (\`expired\`, \`admin\`, \`registrar_request\`, \`policy\`) |
| \`DropCatch\` | bool | Whether this domain was flagged for drop-catch |
| \`LastSnapshot\` | JSON (nullable) | Full domain state at the moment of purge |
| \`CreatedAt\` | time | When the tombstone record was created |

### Purge Reasons

| Reason | Trigger |
|--------|---------|
| \`expired\` | Domain expired → redemption passed → auto-purged by lifecycle scheduler |
| \`admin\` | Registry operator manually deleted the domain |
| \`registrar_request\` | Registrar requested deletion via EPP \`<delete>\` command |
| \`policy\` | Domain purged due to policy enforcement (e.g., abuse, trademark) |

## REST API

All endpoints require authentication.

### Endpoints

| Method | Path | Description |
|--------|------|-------------|
| \`GET\` | \`/tombstones\` | List tombstones with cursor pagination and filters |
| \`GET\` | \`/tombstones/count\` | Count tombstones matching filters |
| \`GET\` | \`/tombstones/:roid\` | Get a single tombstone by ROID |
| \`GET\` | \`/tombstones/by-name/:name\` | Get all incarnations for a domain name |

### Query Parameters (List & Count)

| Param | Description | Example |
|-------|-------------|---------|
| \`name\` | Exact name match (case-insensitive) | \`?name=example.com\` |
| \`name_like\` | Fuzzy name search (ILIKE) | \`?name_like=example\` |
| \`tld\` | Filter by TLD | \`?tld=com\` |
| \`registrar\` | Filter by registrar client ID | \`?registrar=REG-001\` |
| \`purge_reason\` | Filter by purge reason | \`?purge_reason=expired\` |
| \`pagesize\` | Items per page (default 25, max 200) | \`?pagesize=50\` |
| \`cursor\` | Pagination cursor | \`?cursor=abc...\` |

## Frontend Integration

### Archive-Aware Domain Detail Page

When a user navigates to \`/domains/{name}\` and the domain no longer exists (404), the page automatically:

1. Calls \`GET /tombstones/by-name/{name}\`
2. If tombstones exist, renders the **ArchivedDomainView** instead of "Domain not found"
3. Shows an amber archive banner, tombstone metadata, incarnation picker (for multi-incarnation domains), and ROID-scoped event history

### Event Feed Links

All domain names in the EventFeed are **always clickable**, even for \`domain.purged\` and \`domain.admin_deleted\` events. Clicking a purged domain navigates to the domain detail page, which triggers the archive fallback above.

### ⌘K Global Search

Tombstones are searchable via ⌘K. They appear in a dedicated **"Archived Domains"** group with an amber "Archived" badge. Selecting a tombstone navigates to \`/domains/{name}\`, which shows the archive view.

### Domain History Page

\`/domains/{name}/history\` shows the **complete lifecycle** of a domain name across all incarnations:

- **Current incarnation** (if the domain is active) with a green "Active" badge
- **Past incarnations** (from tombstones) ordered by purge date, each with a lifecycle timeline visualizer

### Archive Deep-Link

\`/domains/archive/{roid}\` provides a permanent, stable link to a specific domain incarnation by ROID. This is useful for linking from external systems or audit logs.

## Data Flow

### On Domain Purge

\`\`\`mermaid
sequenceDiagram
    participant W as Lifecycle Worker
    participant DS as DomainService
    participant TR as TombstoneRepo
    participant DR as DomainRepo
    participant ER as EventRepo

    W->>DS: PurgeDomain(ctx, name)
    DS->>DS: derivePurgeReason()
    DS->>DS: Build DomainTombstone from domain state
    DS->>TR: CreateTombstone(ctx, tombstone)
    TR-->>DS: tombstone created (UPSERT)
    DS->>DR: DeleteDomain(ctx, name)
    DR-->>DS: domain deleted
    DS->>ER: CreateEvent("domain.purged", ...)
    ER-->>DS: event recorded
\`\`\`

### Key Design Decisions

1. **ROID as Primary Key** — Ensures each incarnation of a domain gets its own tombstone, even if the same name is re-registered and purged multiple times
2. **UPSERT Semantics** — \`CreateTombstone\` uses \`ON CONFLICT UPDATE ALL\` for idempotency (safe for replays and backfills)
3. **Nil-Safe Injection** — \`tombstoneRepo\` is injected via a setter (\`SetTombstoneRepo\`) rather than the constructor, so existing code paths (tests, CLI tools) that don't need tombstones aren't affected
4. **Non-Critical Path** — Tombstone creation failure logs a warning but does **not** abort the purge operation. The domain lifecycle is more important than the archive record
5. **LastSnapshot** — Stores the full domain state as JSONB for maximum flexibility. Not indexed, but available for deep inspection in the archive UI

## Backfill Workflow

The \`TombstoneBackfillWorkflow\` scans historical \`domain.purged\` events and creates tombstones for domains that were purged before the tombstone system was in place.

### Process

1. Query \`domain_events\` for events with type \`domain.purged\`
2. For each event, extract domain metadata from the event payload
3. Create a tombstone with the available data
4. Track progress and report completion

### Limitations

- Events that predate structured payloads may have incomplete metadata
- The \`LastSnapshot\` field will be \`null\` for backfilled tombstones (the full domain state wasn't captured in the event)

## Operational Notes

### Monitoring

- Tombstone count is available via \`GET /tombstones/count\`
- The backfill workflow reports progress through Temporal UI
- Frontend shows tombstone count in the ⌘K search results

### Storage

- Tombstones are stored in the \`domain_tombstones\` PostgreSQL table
- Indexed columns: \`name\`, \`tld_name\`, \`registrar_cl_id\`, \`purged_at\`
- \`last_snapshot\` uses JSONB type for flexible schema
`;
