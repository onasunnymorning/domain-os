export const DATABASE_INDEX_STRATEGY_DOC_MARKDOWN = `# Database Index Strategy

## Overview

This document describes the PostgreSQL indexing strategy for domain-os, designed to scale from 8M to 80M+ domains without query performance degradation. The strategy balances index storage cost against query speed for the system's most critical operations.

## Architecture

\`\`\`mermaid
graph TB
    subgraph "Hot Tables (indexed)"
        D["domains<br/>80M rows"]
        DH["domain_hosts<br/>160M rows"]
        DE["domain_events<br/>500M+ rows"]
        C["contacts<br/>40M rows"]
        H["hosts<br/>20M rows"]
    end

    subgraph "Small Tables (no extra indexes needed)"
        T["tlds ~50 rows"]
        R["registrars ~5K rows"]
        P["phases ~5K rows"]
    end

    D -->|tld_name, cl_id| T
    D -->|registrant_id, admin_id, tech_id, billing_id| C
    DH -->|host_ro_id| H
    H -->|cl_id| R
    C -->|cl_id| R
\`\`\`

## Index Tiers

### Critical Indexes (domains table)

These prevent full table scans on the largest table in the system.

| Column | Type | Est. Size | Queries That Benefit |
|--------|------|-----------|---------------------|
| \`tld_name\` | B-tree | ~1.5 GB | TLD domain count, DNS zone generation, domain list/count by TLD |
| \`cl_id\` | B-tree | ~1.5 GB | Registrar domain count, domain list by registrar, lifecycle workflows |
| \`registrant_id\` | B-tree | ~1.5 GB | FK integrity on contact DELETE — prevents 80M-row scan |
| \`admin_id\` | B-tree | ~1.5 GB | FK integrity on contact DELETE |
| \`tech_id\` | B-tree | ~1.5 GB | FK integrity on contact DELETE |
| \`billing_id\` | B-tree | ~1.5 GB | FK integrity on contact DELETE |

> **Why FK indexes matter**: PostgreSQL does NOT auto-create indexes on foreign key columns. Without indexes on \`registrant_id\`, \`admin_id\`, \`tech_id\`, and \`billing_id\`, deleting a single contact requires PostgreSQL to scan the entire 80M-row \`domains\` table four times to verify no referencing rows exist. This causes timeouts and lock contention.

### Join Table Indexes

| Table | Column | Type | Queries That Benefit |
|-------|--------|------|---------------------|
| \`domain_hosts\` | \`host_ro_id\` | B-tree | Host association count, orphan host detection, DNS glue JOINs |
| \`accreditations\` | \`tld_name\` | B-tree | TLD registrar count GROUP BY (second in composite PK, can't be used alone) |

### Composite Indexes

| Table | Columns | Purpose |
|-------|---------|---------|
| \`domain_events\` | \`(subject, occurred_at DESC)\` | Event timeline per domain — replaces 2 separate indexes |
| \`phases\` | \`(tld_name, type, starts)\` | \`activeGAPhaseFilter\` EXISTS subquery on every lifecycle query |
| \`domains\` | \`(tld_name, expiry_date)\` | TLD-only expiring domain queries (e.g. global workflows) |
| \`domains\` | \`(tld_name, purge_date)\` | TLD-only purgeable domain queries (e.g. global workflows) |

### Supporting Indexes

| Table | Column | Queries |
|-------|--------|---------|
| \`contacts\` | \`cl_id\` | Contact list by registrar |
| \`hosts\` | \`cl_id\` | Host list by registrar (composite unique has cl_id second, unusable alone) |
| \`nndns\` | \`tld_name\` | NNDN list per TLD |

## Domain Events Index Optimization

The \`domain_events\` table is append-only and grows to 500M+ rows. The original schema had **7 individual B-tree indexes (~90 GB at scale)**, 4 of which had zero matching queries in the codebase.

### Before (7 indexes, ~90 GB)

| Index | Had matching query? |
|-------|-------------------|
| \`source\` | ❌ No |
| \`type\` | ❌ No |
| \`subject\` | ✅ Yes |
| \`occurred_at\` | ✅ Yes |
| \`trace_id\` | ❌ No |
| \`correlation_id\` | ❌ No |
| \`published\` | ✅ Yes (outbox relay) |

### After (5 indexes, ~30 GB)

| Index | Purpose |
|-------|---------|
| \`(subject, occurred_at DESC)\` composite | Event timeline query — single index covers \`WHERE subject = ? ORDER BY occurred_at DESC\` |
| \`occurred_at DESC\` | Global timeline query — single index covers \`ORDER BY occurred_at DESC LIMIT ?\` (e.g. ListRecentEvents) |
| \`published\` | Outbox relay flag for future event consumer |
| \`id\` (PK) | Primary key |
| \`id\` (Partial) | Partial index \`WHERE type = 'domain.purged' AND ro_id != ''\` to optimize tombstone backfill pagination |

**Net savings: ~60 GB** of index storage at scale, plus reduced write amplification on every event INSERT.

## Storage Budget

| Tier | Tables | Est. Storage at 80M Domains |
|------|--------|-----------------------------|
| Critical (domains) | \`tld_name\`, \`cl_id\`, FK cols × 4 | ~9 GB |
| Join tables | \`domain_hosts\`, \`accreditations\` | ~2.5 GB |
| Composite | \`domain_events\`, \`phases\` | ~8 GB |
| Supporting | \`contacts\`, \`hosts\`, \`nndns\` | ~2.8 GB |
| **Total added** | | **~22 GB** |
| **Event cleanup savings** | | **−70 GB** |
| **Net change** | | **−48 GB** |

## Implementation Details

### GORM Tag Indexes

Indexes on model fields are declared via GORM struct tags and created automatically during \`AutoMigrate\`:

\`\`\`go
// domains table
TLDName      string  \\\`gorm:"not null;foreignKey;index"\\\`
ClID         string  \\\`gorm:"index"\\\`
RegistrantID *string \\\`gorm:"index"\\\`
AdminID      *string \\\`gorm:"index"\\\`
TechID       *string \\\`gorm:"index"\\\`
BillingID    *string \\\`gorm:"index"\\\`
\`\`\`

### Manual Migration Indexes

Indexes that can't be expressed via GORM tags (M2M join tables, composite indexes) are created in \`connection.go\` using \`CREATE INDEX IF NOT EXISTS\`:

\`\`\`sql
-- domain_hosts reverse lookup
CREATE INDEX IF NOT EXISTS idx_domain_hosts_host_ro_id
  ON domain_hosts (host_ro_id);

-- phases composite for lifecycle subquery
CREATE INDEX IF NOT EXISTS idx_phases_tld_type_starts
  ON phases (tld_name, type, starts);

-- accreditations by TLD
CREATE INDEX IF NOT EXISTS idx_accreditations_tld_name
  ON accreditations (tld_name);

-- domain_events partial index for tombstone backfill pagination
CREATE INDEX IF NOT EXISTS idx_domain_events_purged_id
  ON domain_events (id) WHERE type = 'domain.purged' AND ro_id != '';

-- domains composite index for TLD-only expiring domains
CREATE INDEX IF NOT EXISTS idx_domains_tld_expiry
  ON domains (tld_name, expiry_date);

-- domains composite index for TLD-only purgeable domains
CREATE INDEX IF NOT EXISTS idx_domains_tld_purge
  ON domains (tld_name, purge_date);
\`\`\`

### Legacy Index Cleanup

Unused indexes are dropped during migration:

\`\`\`sql
DROP INDEX IF EXISTS idx_domain_event_records_source;
DROP INDEX IF EXISTS idx_domain_event_records_type;
DROP INDEX IF EXISTS idx_domain_event_records_trace_id;
DROP INDEX IF EXISTS idx_domain_event_records_correlation_id;
DROP INDEX IF EXISTS idx_domain_event_records_subject;
\`\`\`

## Query Patterns Covered

### TLD Page — Domain Count
**Before**: N+1 HTTP requests, each running \`COUNT(*) WHERE tld_name = ?\` with sequential scan.
**After**: Single \`GROUP BY tld_name\` JOIN in the TLD list query, using \`tld_name\` B-tree index.

### Registrar Page — Domain Count
**Before**: Subquery \`SELECT cl_id, COUNT(*) FROM domains GROUP BY cl_id\` with sequential scan.
**After**: Same query, now using \`cl_id\` B-tree index.

### DNS Zone Generation
Queries \`GetActiveDomainsWithHosts\` and \`GetActiveDomainGlue\` filter by \`tld_name\` and join through \`domain_hosts\`. Both indexes (\`domains.tld_name\` and \`domain_hosts.host_ro_id\`) are now available.

### Lifecycle Workflows
\`ListExpiringDomains\`, \`ListPurgeableDomains\`, \`ListRestoredDomains\` all filter by \`cl_id\` and/or \`tld_name\`, and use an EXISTS subquery on \`phases(tld_name, type, starts)\`. All three dimensions are now indexed.

### Contact/Host Deletion (FK Integrity)
Deleting a contact or registrar triggers FK constraint checks on \`domains.registrant_id\`, \`admin_id\`, \`tech_id\`, \`billing_id\`, and \`cl_id\`. All now have B-tree indexes.

## Related Files

- \`internal/infrastructure/db/postgres/domain.go\` — Domain model with index tags
- \`internal/infrastructure/db/postgres/domain_event.go\` — Event model with optimized indexes
- \`internal/infrastructure/db/postgres/connection.go\` — AutoMigrate + manual index creation/cleanup
- \`internal/infrastructure/db/postgres/contact.go\` — Contact model
- \`internal/infrastructure/db/postgres/host.go\` — Host model
- \`internal/infrastructure/db/postgres/nndn.go\` — NNDN model
`;
