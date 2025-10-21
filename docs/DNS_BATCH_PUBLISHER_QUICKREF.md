# DNS Batch Publisher - Quick Reference

## Quick Start

### 1. Start the Worker (main.go)

```go
import "github.com/onasunnymorning/domain-os/internal/infrastructure/dnsevents"

// In main()
batchPublisher := dnsevents.NewBatchPublisher(db, nil) // nil = use defaults
batchPublisher.Start()
defer batchPublisher.Stop()
```

### 2. Queue DNS Changes (Domain Service)

```go
// Single change
batchPublisher.QueueChange(ctx, &dnsevents.DNSChange{
    ZoneName:   "test.",
    ChangeType: dnsevents.DNSChangeTypeAdd,
    RecordType: dnsevents.DNSRecordTypeNS,
    RecordName: "example.test.",
    RecordData: "ns1.example.com.",
    TTL:        3600,
})

// Multiple changes
batchPublisher.QueueChanges(ctx, []*dnsevents.DNSChange{...})
```

### 3. Monitor Queue

```sql
-- Quick stats
SELECT * FROM dns_queue_stats;

-- Pending items
SELECT COUNT(*) FROM dns_change_queue WHERE published_at IS NULL;

-- Recent publishes
SELECT zone_name, COUNT(*) 
FROM dns_change_queue 
WHERE published_at >= NOW() - INTERVAL '1 hour'
GROUP BY zone_name;
```

## Common Operations

### Check Worker Status

```go
// In logs, look for:
// "DNS batch publisher started batch_interval=1m0s max_batch_size=10000"
// "DNS batch worker started interval=1m0s"
```

### Manual Flush (Testing)

```go
// Force immediate flush (bypasses timer)
batchPublisher.flushAll()
```

### Cleanup Old Items

```go
// Delete published items older than 7 days
deleted, err := batchPublisher.CleanupPublished(ctx, 7)
```

### Get Statistics

```go
stats, err := batchPublisher.GetQueueStats(ctx)
for _, stat := range stats {
    fmt.Printf("Zone: %s, Pending: %d, Published: %d\n", 
        stat.ZoneName, stat.PendingCount, stat.PublishedCount)
}
```

## Configuration Options

### Default Configuration
```go
config := &dnsevents.BatchPublisherConfig{
    BatchInterval: 1 * time.Minute,  // Flush every 1 minute
    MaxBatchSize:  10000,            // Max 10K changes per batch
}
```

### Development (Fast)
```go
config := &dnsevents.BatchPublisherConfig{
    BatchInterval: 5 * time.Second,  // Fast for testing
    MaxBatchSize:  1000,
}
```

### Production (High Volume)
```go
config := &dnsevents.BatchPublisherConfig{
    BatchInterval: 2 * time.Minute,  // Less frequent
    MaxBatchSize:  50000,            // Larger batches
}
```

## Monitoring Queries

### Queue Health
```sql
-- Zones with pending changes
SELECT zone_name, COUNT(*) as pending, MIN(queued_at) as oldest
FROM dns_change_queue
WHERE published_at IS NULL
GROUP BY zone_name
ORDER BY pending DESC;
```

### Error Tracking
```sql
-- Failed items
SELECT zone_name, record_name, error_count, last_error
FROM dns_change_queue
WHERE error_count > 0
ORDER BY error_count DESC;
```

### Publishing Rate
```sql
-- Changes published per minute (last hour)
SELECT 
    DATE_TRUNC('minute', published_at) as minute,
    COUNT(*) as changes
FROM dns_change_queue
WHERE published_at >= NOW() - INTERVAL '1 hour'
GROUP BY minute
ORDER BY minute DESC;
```

### Serial Verification
```sql
-- Verify batches got single serials
SELECT zone_name, batch_id, COUNT(DISTINCT serial) as serial_count
FROM dns_zone_journal
WHERE batch_id IS NOT NULL
GROUP BY zone_name, batch_id
HAVING COUNT(DISTINCT serial) > 1;
-- Should return no rows (each batch = 1 serial)
```

## Troubleshooting

### Problem: No Items Publishing

**Check:**
1. Worker running? Look for log message
2. Database connection? Check errors in logs
3. Function exists? `SELECT proname FROM pg_proc WHERE proname = 'get_next_serial';`

**Fix:**
- Restart application
- Verify migrations ran: `SELECT * FROM dns_zone_serials;`

### Problem: Items Stuck in Queue

**Check:**
```sql
-- Items stuck for > 5 minutes
SELECT zone_name, COUNT(*), MIN(queued_at) as oldest
FROM dns_change_queue
WHERE published_at IS NULL AND queued_at < NOW() - INTERVAL '5 minutes'
GROUP BY zone_name;
```

**Fix:**
- Check for errors: `SELECT last_error FROM dns_change_queue WHERE error_count > 0;`
- Manual flush: `batchPublisher.flushAll()`

### Problem: High Error Count

**Check:**
```sql
SELECT last_error, COUNT(*) 
FROM dns_change_queue 
WHERE error_count > 0 
GROUP BY last_error;
```

**Fix:**
- Fix underlying issue (DB connection, function error)
- Reset error count: `UPDATE dns_change_queue SET error_count = 0;`

## API Reference

### Types

```go
type DNSChange struct {
    ZoneName        string
    ChangeType      DNSChangeType  // DNSChangeTypeAdd or DNSChangeTypeDelete
    RecordType      DNSRecordType  // DNSRecordTypeNS, DNSRecordTypeA, etc.
    RecordName      string         // FQDN (e.g., "example.test.")
    RecordData      string         // NS hostname, IP address, etc.
    TTL             uint32         // Default: 3600
    SourceOperation string         // Optional: "CreateDomain", "UpdateDomain"
    DomainName      string         // Optional: for tracking
}

type QueueStats struct {
    ZoneName       string
    PendingCount   int64
    PublishedCount int64
    ErrorCount     int64
    OldestPending  *time.Time
}
```

### Constants

```go
// Change Types
DNSChangeTypeAdd    = "add"
DNSChangeTypeDelete = "delete"

// Record Types
DNSRecordTypeNS   = "NS"
DNSRecordTypeA    = "A"
DNSRecordTypeAAAA = "AAAA"
DNSRecordTypeCNAME = "CNAME"
DNSRecordTypeMX   = "MX"
DNSRecordTypeTXT  = "TXT"
```

### Methods

```go
// Create publisher
NewBatchPublisher(db *gorm.DB, config *BatchPublisherConfig) *BatchPublisher

// Lifecycle
Start() error                          // Start worker
Stop() error                           // Graceful shutdown with final flush

// Queue operations
QueueChange(ctx, *DNSChange) error     // Queue single change
QueueChanges(ctx, []*DNSChange) error  // Queue multiple changes

// Monitoring
GetQueueStats(ctx) ([]QueueStats, error)       // Get stats from view
CleanupPublished(ctx, retentionDays) (int64, error)  // Cleanup old items
```

## Performance Guidelines

### Batch Sizing

| Scenario | BatchInterval | MaxBatchSize | Rationale |
|----------|---------------|--------------|-----------|
| Development | 5 seconds | 1,000 | Fast feedback |
| Production | 1 minute | 10,000 | Balanced |
| Bulk Import | 5 minutes | 50,000 | Efficiency |
| Low Volume | 30 seconds | 5,000 | Responsiveness |

### Queue Depth Targets

- **Normal**: < 1,000 pending items per zone
- **Warning**: 1,000 - 10,000 pending
- **Critical**: > 10,000 pending (may indicate worker issues)

### Serial Rate

- **Ideal**: 1 serial/minute (1 batch/minute)
- **Maximum**: 1,440 serials/day (if flushing every minute for 24 hours)
- **Overflow**: 99 serials/day (old YYYYMMDDnn limit) - **avoided with batching**

## Database Schema

### Queue Table
```sql
dns_change_queue
  - id (PK)
  - zone_name
  - change_type (add/delete)
  - record_type (NS/A/AAAA/etc.)
  - record_name (FQDN)
  - record_data
  - ttl
  - queued_at
  - published_at (NULL = pending)
  - batch_id
  - error_count
  - last_error
```

### Journal Table
```sql
dns_zone_journal
  - id (PK)
  - zone_name
  - serial (from get_next_serial())
  - change_type
  - record_type
  - record_name
  - record_data
  - ttl
  - source_operation
  - domain_name
  - timestamp
```

### Serials Table
```sql
dns_zone_serials
  - zone_name (PK)
  - serial (YYYYMMDDnn format)
  - updated_at
```

## Key Concepts

### Time-Based Batching
- Changes queued → Database table
- Worker runs every interval (default: 1 minute)
- All pending changes for a zone → single batch
- Batch published → single serial
- Queue items marked as published

### Single Serial Per Batch
- Traditional: 1 change = 1 serial increment
- Batching: 1000 changes = 1 serial increment
- Efficiency: Reduces serial churn 1000x
- Prevents: YYYYMMDDnn overflow (99/day limit)

### Crash Recovery
- Queue in database (not memory)
- Application crash → queue persists
- Restart → worker picks up pending items
- No data loss

### Row-Level Locking
- `FOR UPDATE SKIP LOCKED`
- Multiple workers → no conflicts
- Each worker processes different zones
- No blocking waits

## Default Values

```go
BatchInterval:    1 * time.Minute
MaxBatchSize:     10000
DefaultTTL:       3600 (if not specified)
CleanupRetention: 7 days
```

## Files

- Implementation: `/internal/infrastructure/dnsevents/batch_publisher.go`
- Tests: `/internal/infrastructure/dnsevents/batch_publisher_test.go`
- Schema: `/internal/infrastructure/db/postgres/dns_queue_schema.go`
- Integration Guide: `/docs/DNS_BATCH_PUBLISHER_INTEGRATION.md`
- This Reference: `/docs/DNS_BATCH_PUBLISHER_QUICKREF.md`

## See Also

- Full integration guide: `DNS_BATCH_PUBLISHER_INTEGRATION.md`
- Phase 1 completion: `DNS_PHASE1_COMPLETE.md`
- Architecture overview: `DNS_ZONE_MVP_PLAN.md`
- Serial reliability: `DNS_ZONE_SERIAL_RELIABILITY.md`
- Time-based batching: `DNS_ZONE_TIME_BASED_BATCHING.md`
