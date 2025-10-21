# Time-Based Bulk DNS Updates - Design & Analysis

## Executive Summary

**Proposal:** Instead of publishing DNS changes immediately, batch them and publish at fixed intervals (e.g., every 1 minute or 5 minutes).

**Verdict:** 🟢 **HIGHLY RECOMMENDED** - This solves most operational risks while maintaining acceptable DNS propagation times.

---

## Architecture Comparison

### Current: Immediate Publishing

```
Domain Created ──────> Publish DNS Event ──────> Serial Increment
                       (instant)                  (instant)

Timeline:
00:00:00.001  Domain 1 created → Serial 2025101201
00:00:00.002  Domain 2 created → Serial 2025101202
00:00:00.003  Domain 3 created → Serial 2025101203
...
00:00:01.000  1000 domains → 1000 serial increments
```

### Proposed: Time-Based Batching

```
Domain Created ──────> Queue Change ──────> [Wait for batch window] ──────> Publish Batch
                       (instant)            (max 1 minute)                  (1 serial)

Timeline:
00:00:00.001  Domain 1 created → Queued
00:00:00.002  Domain 2 created → Queued
00:00:00.003  Domain 3 created → Queued
...
00:00:59.999  Domain 1000 created → Queued
00:01:00.000  [BATCH PUBLISH] → All 1000 changes → 1 serial increment (2025101201)
```

---

## Detailed Design

### Implementation Option 1: In-Memory Queue (Simple)

```go
// internal/infrastructure/dnsevents/batch_publisher.go

package dnsevents

import (
    "context"
    "sync"
    "time"
    
    "github.com/rs/zerolog/log"
    "gorm.io/gorm"
)

// BatchPublisher batches DNS changes and publishes at fixed intervals
type BatchPublisher struct {
    db                *gorm.DB
    publisher         *EventPublisher
    
    // Configuration
    batchInterval     time.Duration  // e.g., 1 minute
    maxBatchSize      int            // e.g., 10000 changes
    
    // State
    queue             map[string][]*DNSChange  // zone_name -> changes
    queueMu           sync.RWMutex
    
    // Control
    stopCh            chan struct{}
    wg                sync.WaitGroup
}

// NewBatchPublisher creates a new batch publisher
func NewBatchPublisher(db *gorm.DB, interval time.Duration) *BatchPublisher {
    bp := &BatchPublisher{
        db:            db,
        publisher:     NewEventPublisher(db),
        batchInterval: interval,
        maxBatchSize:  10000,
        queue:         make(map[string][]*DNSChange),
        stopCh:        make(chan struct{}),
    }
    
    // Start background worker
    bp.wg.Add(1)
    go bp.worker()
    
    return bp
}

// QueueChange adds a DNS change to the batch queue
// This is non-blocking and returns immediately
func (bp *BatchPublisher) QueueChange(ctx context.Context, change *DNSChange) error {
    bp.queueMu.Lock()
    defer bp.queueMu.Unlock()
    
    zoneName := change.ZoneName
    bp.queue[zoneName] = append(bp.queue[zoneName], change)
    
    log.Debug().
        Str("zone", zoneName).
        Int("queue_size", len(bp.queue[zoneName])).
        Msg("DNS change queued")
    
    // Check if batch is full (optional: flush early if queue too large)
    if len(bp.queue[zoneName]) >= bp.maxBatchSize {
        log.Warn().
            Str("zone", zoneName).
            Int("queue_size", len(bp.queue[zoneName])).
            Msg("Batch queue full, consider reducing interval or increasing max size")
    }
    
    return nil
}

// worker runs in background and publishes batches at fixed intervals
func (bp *BatchPublisher) worker() {
    defer bp.wg.Done()
    
    ticker := time.NewTicker(bp.batchInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            bp.flushBatches()
            
        case <-bp.stopCh:
            // Final flush before shutdown
            bp.flushBatches()
            return
        }
    }
}

// flushBatches publishes all queued changes as batches
func (bp *BatchPublisher) flushBatches() {
    bp.queueMu.Lock()
    
    // Snapshot current queue and clear it
    batches := bp.queue
    bp.queue = make(map[string][]*DNSChange)
    
    bp.queueMu.Unlock()
    
    // Process each zone's batch
    for zoneName, changes := range batches {
        if len(changes) == 0 {
            continue
        }
        
        err := bp.publishBatch(zoneName, changes)
        if err != nil {
            log.Error().
                Err(err).
                Str("zone", zoneName).
                Int("changes", len(changes)).
                Msg("Failed to publish DNS batch")
            
            // TODO: Retry logic or dead letter queue
        } else {
            log.Info().
                Str("zone", zoneName).
                Int("changes", len(changes)).
                Msg("DNS batch published")
        }
    }
}

// publishBatch publishes a batch of changes with a single serial increment
func (bp *BatchPublisher) publishBatch(zoneName string, changes []*DNSChange) error {
    ctx := context.Background()
    
    return bp.db.Transaction(func(tx *gorm.DB) error {
        // Get next serial ONCE for entire batch
        var serial int64
        err := tx.WithContext(ctx).Raw(
            "SELECT get_next_serial(?)",
            zoneName,
        ).Scan(&serial).Error
        if err != nil {
            return err
        }
        
        // Insert all changes with the SAME serial
        for _, change := range changes {
            err = tx.WithContext(ctx).Exec(`
                INSERT INTO dns_zone_journal (
                    zone_name, serial, change_type, record_type,
                    record_name, record_data, ttl,
                    source_operation, domain_name
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
                zoneName,
                serial,  // ← Same serial for ALL changes in batch
                string(change.ChangeType),
                string(change.RecordType),
                change.RecordName,
                change.RecordData,
                change.TTL,
                change.SourceOperation,
                change.DomainName,
            ).Error
            
            if err != nil {
                return err
            }
        }
        
        log.Info().
            Str("zone", zoneName).
            Int64("serial", serial).
            Int("batch_size", len(changes)).
            Msg("DNS batch committed")
        
        return nil
    })
}

// Stop gracefully shuts down the batch publisher
func (bp *BatchPublisher) Stop() {
    close(bp.stopCh)
    bp.wg.Wait()
}

// GetQueueSize returns current queue size for monitoring
func (bp *BatchPublisher) GetQueueSize(zoneName string) int {
    bp.queueMu.RLock()
    defer bp.queueMu.RUnlock()
    return len(bp.queue[zoneName])
}
```

### Implementation Option 2: Database-Backed Queue (Reliable)

For production systems that need durability and crash recovery:

```go
// Use a staging table as the queue

CREATE TABLE IF NOT EXISTS dns_change_queue (
    id BIGSERIAL PRIMARY KEY,
    zone_name VARCHAR(255) NOT NULL,
    change_type VARCHAR(10) NOT NULL,
    record_type VARCHAR(10) NOT NULL,
    record_name VARCHAR(255) NOT NULL,
    record_data TEXT NOT NULL,
    ttl INTEGER NOT NULL,
    source_operation VARCHAR(50),
    domain_name VARCHAR(255),
    
    -- Queue metadata
    queued_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    published_at TIMESTAMPTZ,
    batch_id BIGINT,
    
    -- Indexes
    INDEX idx_queue_pending (zone_name, queued_at) WHERE published_at IS NULL
);

// Worker query
SELECT id, zone_name, change_type, record_type, record_name, record_data, ttl
FROM dns_change_queue
WHERE published_at IS NULL
AND queued_at <= NOW() - INTERVAL '1 minute'
ORDER BY zone_name, queued_at
FOR UPDATE SKIP LOCKED;  -- Prevent concurrent workers from processing same items
```

**Advantages:**
- ✅ Survives application crashes
- ✅ Can inspect queue state (useful for debugging)
- ✅ Natural backpressure mechanism
- ✅ Supports distributed workers

**Disadvantages:**
- ⚠️ More database I/O
- ⚠️ Needs cleanup/archival

---

## Advantages of Time-Based Batching

### 1. Massive Reduction in Serial Churn

**Before (immediate publishing):**
```
5M domain bulk import:
- 5,000,000 serial increments
- Serial: 2025101201 → 1728762000 (switches to Unix timestamp)
- Takes: ~1.4 hours
- Journal entries: 5,000,000 rows
```

**After (1-minute batching):**
```
5M domain bulk import (same scenario):
- ~84 serial increments (5M domains / 60,000 domains per minute)
- Serial: 2025101201 → 2025101284 (stays in YYYYMMDDnn format!)
- Takes: ~1.4 hours (same)
- Journal entries: 5,000,000 rows (same, but grouped by serial)
```

**Improvement:** 59,524x fewer serial increments!

### 2. Predictable Secondary DNS Update Cadence

**Current (immediate):**
```
00:00:01  NOTIFY (serial 2025101201)
00:00:02  NOTIFY (serial 2025101202)
00:00:03  NOTIFY (serial 2025101203)
...continuous NOTIFYs...

Secondary DNS servers:
- Constantly receiving NOTIFYs
- Constantly requesting IXFRs
- High network traffic
- Hard to predict load
```

**With batching:**
```
00:00:00  (1000 domains created, queued)
00:01:00  NOTIFY (serial 2025101201, 1000 changes)
00:02:00  NOTIFY (serial 2025101202, 850 changes)
00:03:00  (no changes, no NOTIFY)
00:04:00  NOTIFY (serial 2025101203, 500 changes)

Secondary DNS servers:
- NOTIFY every 1 minute (max)
- Predictable IXFR size
- Can schedule around batch times
- Better resource planning
```

### 3. Eliminates Lock Contention

**Current:**
```
1000 concurrent domain creations
→ 1000 concurrent get_next_serial() calls
→ All wait for FOR UPDATE lock
→ Serial execution
→ Throughput: ~2,000 TPS
```

**With batching:**
```
1000 concurrent domain creations
→ 1000 concurrent QueueChange() calls (no locks!)
→ All return immediately
→ Throughput: ~50,000 TPS (queue writes)

Every 1 minute:
→ 1 get_next_serial() call
→ No contention
```

**Improvement:** 25x better write throughput

### 4. Natural Rate Limiting

```go
// Automatic backpressure
if len(bp.queue[zoneName]) >= bp.maxBatchSize {
    // Option 1: Flush early
    bp.flushBatches()
    
    // Option 2: Return error (backpressure)
    return ErrQueueFull
    
    // Option 3: Block until next flush
    <-bp.nextFlush
}
```

Prevents runaway updates from overwhelming the system.

### 5. Easier Monitoring and Alerting

```go
// Prometheus metrics
dns_queue_size{zone="com"} 1523
dns_batch_size_last{zone="com"} 1500
dns_batch_interval_seconds 60
dns_batches_published_total{zone="com"} 144  // 144 batches in 24h

// Simple alerts
if dns_queue_size > 50000:
    alert: "DNS queue backing up"

if dns_batch_size_last > 10000:
    alert: "Unusually large batch"
```

### 6. Graceful Degradation

```
Normal load:
- Batch every 1 minute
- Small batches (10-100 changes)

High load:
- Batch every 1 minute
- Large batches (1000-10000 changes)
- Still only 1 serial per minute

Extreme load:
- Queue fills up
- Backpressure kicks in
- System stays stable
```

---

## Disadvantages & Risks

### 1. DNS Propagation Delay

**Current:** 
```
Domain created 00:00:00.001
DNS published 00:00:00.001  (instant)
Secondary refreshed 00:00:00.100  (within 100ms)
```

**With 1-minute batching:**
```
Domain created 00:00:00.001
DNS published 00:01:00.000  (up to 1 minute delay)
Secondary refreshed 00:01:00.100  (1 minute + 100ms)
```

**Impact:**
- ⚠️ New domain not resolvable for up to 1 minute
- ⚠️ For registrar operations (immediate delegation checks), this might be too slow
- ✅ For normal operations, 1 minute is acceptable

**Mitigation:**
```go
// Configurable per operation type
type PublishMode int
const (
    PublishBatched   PublishMode = iota  // Queue for next batch (normal)
    PublishImmediate                     // Bypass queue (urgent)
)

func (svc *DomainService) CreateDomain(ctx context.Context, cmd *commands.CreateDomainCommand) error {
    // Normal domain creation: batched
    dnsPublisher.QueueChange(ctx, change)
    
    // Premium/urgent domain: immediate
    if cmd.Urgent {
        dnsPublisher.PublishChange(ctx, tx, change)
    }
}
```

### 2. Memory Usage (In-Memory Queue)

**Calculation:**
```go
type DNSChange struct {
    ZoneName        string  // ~10 bytes
    ChangeType      string  // ~6 bytes
    RecordType      string  // ~4 bytes
    RecordName      string  // ~50 bytes
    RecordData      string  // ~50 bytes
    TTL             uint32  // 4 bytes
    SourceOperation string  // ~20 bytes
    DomainName      string  // ~50 bytes
}
// Total: ~200 bytes per change
```

**At scale:**
```
10,000 changes/minute × 200 bytes = 2 MB in memory
100,000 changes/minute = 20 MB
1,000,000 changes/minute = 200 MB
```

**Mitigation:**
- Use database-backed queue for high-volume scenarios
- Set `maxBatchSize` limit
- Monitor queue depth

### 3. Crash Recovery (In-Memory Queue)

**Problem:**
```
00:00:00  Domain 1 created → Queued in memory
00:00:30  Domain 1000 created → Queued in memory
00:00:45  Application crashes! ⚡
         → All queued changes LOST

00:01:00  Application restarts
         → Domains exist in DB but no DNS events published
         → DNS zones out of sync!
```

**Solutions:**

#### Option A: Database-Backed Queue (Recommended)
```go
// All changes persisted immediately
func (bp *BatchPublisher) QueueChange(ctx context.Context, change *DNSChange) error {
    return bp.db.WithContext(ctx).Exec(`
        INSERT INTO dns_change_queue (zone_name, change_type, ...)
        VALUES (?, ?, ...)
    `, change.ZoneName, ...).Error
    
    // Survives crashes!
}
```

#### Option B: Write-Ahead Log (WAL)
```go
// Append to disk before queuing
func (bp *BatchPublisher) QueueChange(ctx context.Context, change *DNSChange) error {
    // 1. Write to WAL file
    bp.wal.Append(change)
    
    // 2. Queue in memory
    bp.queue[change.ZoneName] = append(bp.queue[change.ZoneName], change)
    
    return nil
}

// On startup, replay WAL
func (bp *BatchPublisher) Start() {
    unprocessed := bp.wal.ReadUnprocessed()
    for _, change := range unprocessed {
        bp.queue[change.ZoneName] = append(bp.queue[change.ZoneName], change)
    }
}
```

#### Option C: Accept Data Loss (For Non-Critical Systems)
```
- Queue is cleared on crash
- Run reconciliation job on startup:
  - Compare domain table with DNS journal
  - Publish missing changes
```

### 4. IXFR Size Variability

**With immediate publishing:**
```
IXFR from serial 2025101200 to 2025101210:
- 10 changes (predictable)
```

**With batching:**
```
IXFR from serial 2025101200 to 2025101201:
- Could be 1 change (slow period)
- Could be 10,000 changes (busy period)
```

**Impact:**
- ⚠️ Secondary DNS servers receive variable-sized IXFR responses
- ⚠️ Large IXFR could timeout or fail
- ✅ Secondary can fall back to AXFR

**Mitigation:**
```go
// Split large batches across multiple serials
const MaxChangesPerSerial = 5000

func (bp *BatchPublisher) publishBatch(zoneName string, changes []*DNSChange) error {
    for i := 0; i < len(changes); i += MaxChangesPerSerial {
        end := min(i+MaxChangesPerSerial, len(changes))
        batch := changes[i:end]
        
        // Each sub-batch gets its own serial
        bp.publishSubBatch(zoneName, batch)
    }
}
```

### 5. Testing Complexity

**Immediate publishing:**
```go
func TestCreateDomain(t *testing.T) {
    domain := createDomain(ctx, "example.com")
    
    // Verify DNS event immediately
    var count int64
    db.Raw("SELECT COUNT(*) FROM dns_zone_journal WHERE domain_name = ?", "example.com").Scan(&count)
    assert.Equal(t, 1, count)
}
```

**With batching:**
```go
func TestCreateDomain(t *testing.T) {
    domain := createDomain(ctx, "example.com")
    
    // DNS event NOT published yet (queued)
    var count int64
    db.Raw("SELECT COUNT(*) FROM dns_zone_journal WHERE domain_name = ?", "example.com").Scan(&count)
    assert.Equal(t, 0, count)  // ← Queued, not published
    
    // Wait for batch or manually trigger flush
    batchPublisher.flushBatches()
    
    // Now verify
    db.Raw("SELECT COUNT(*) FROM dns_zone_journal WHERE domain_name = ?", "example.com").Scan(&count)
    assert.Equal(t, 1, count)
}
```

**Mitigation:**
```go
// Test helper
func (bp *BatchPublisher) FlushForTesting() {
    bp.flushBatches()
}

// Or use immediate mode in tests
func TestCreateDomain(t *testing.T) {
    publisher := NewEventPublisher(db)  // Use immediate publisher
    // ... test without batching delays
}
```

---

## Recommended Configuration

### Development
```go
batchInterval := 10 * time.Second  // Fast feedback
maxBatchSize := 1000
queueType := "in-memory"  // Simple
```

### Production (Normal Load)
```go
batchInterval := 1 * time.Minute  // Good balance
maxBatchSize := 10000
queueType := "database"  // Reliable
```

### Production (High Load)
```go
batchInterval := 5 * time.Minute  // Reduce serial churn
maxBatchSize := 50000
queueType := "database"
workers := 3  // Parallel batch processing
```

### Production (Critical/Urgent Operations)
```go
// Mixed mode: batched for normal, immediate for urgent
func (svc *DomainService) CreateDomain(cmd *CreateDomainCommand) error {
    if cmd.Priority == "urgent" {
        return svc.immediatePublisher.PublishChange(...)
    } else {
        return svc.batchPublisher.QueueChange(...)
    }
}
```

---

## Metrics & Monitoring

```go
// Expose Prometheus metrics
type BatchPublisherMetrics struct {
    QueueSize         prometheus.GaugeVec    // Current queue size per zone
    BatchSize         prometheus.HistogramVec // Distribution of batch sizes
    BatchDuration     prometheus.HistogramVec // Time to publish batch
    BatchErrors       prometheus.CounterVec   // Failed batches
    ChangesPublished  prometheus.CounterVec   // Total changes published
    QueueWaitTime     prometheus.HistogramVec // Time change spent in queue
}

// Example queries
// Average queue size (Grafana)
avg_over_time(dns_queue_size{zone="com"}[5m])

// Batch size distribution
histogram_quantile(0.95, dns_batch_size_bucket{zone="com"})

// Changes published per minute
rate(dns_changes_published_total{zone="com"}[1m])
```

---

## Migration Strategy

### Phase 1: Deploy Batching (Opt-In)

```go
// Add feature flag
type DomainServiceConfig struct {
    UseBatchedDNS bool  // Default: false
}

func (svc *DomainService) publishDNSChange(...) {
    if svc.config.UseBatchedDNS {
        return svc.batchPublisher.QueueChange(...)
    } else {
        return svc.immediatePublisher.PublishChange(...)
    }
}
```

### Phase 2: Enable for Low-Traffic TLDs

```
Week 1: Enable batching for .test, .dev (low risk)
Week 2: Monitor, adjust interval
Week 3: Enable for .net, .org
Week 4: Enable for .com (if no issues)
```

### Phase 3: Make Default

```go
UseBatchedDNS: true  // Default for all new TLDs
```

---

## Comparison Matrix

| Aspect | Immediate Publishing | 1-Minute Batching | 5-Minute Batching |
|--------|---------------------|-------------------|-------------------|
| **DNS Propagation** | <100ms | <1 minute | <5 minutes |
| **Serial Increments (5M domains/day)** | 5,000,000 | ~3,500 | ~700 |
| **Lock Contention** | 🔴 High | 🟢 Minimal | 🟢 Minimal |
| **Secondary NOTIFY Rate** | 🔴 Continuous | 🟡 60/hour | 🟢 12/hour |
| **Journal Bloat** | 🔴 High | 🟡 Medium | 🟢 Low |
| **Memory Usage** | 🟢 None | 🟡 ~20MB | 🟡 ~100MB |
| **Crash Recovery** | 🟢 N/A | 🔴 Needs DB queue | 🔴 Needs DB queue |
| **Suitable For** | Low traffic | Normal traffic | High traffic |

---

## Real-World Example: .com Registry

**.com statistics:**
- ~160M domains total
- ~200K new registrations per day
- ~500K updates per day (renewals, NS changes, etc.)

**With immediate publishing:**
```
500,000 changes/day
= 20,833 changes/hour
= 347 changes/minute
= 5.8 changes/second

Serial increments: 500,000/day
Format: Stays in YYYYMMDDnn (within 99/day limit) ✅
But: Constant NOTIFY to secondaries
Lock contention: Moderate
```

**With 1-minute batching:**
```
500,000 changes/day
÷ 1440 minutes/day
= 347 changes per batch (average)

Serial increments: 1440/day
Format: Stays in YYYYMMDDnn ✅
NOTIFY rate: 60/hour (manageable)
Lock contention: Minimal ✅
```

**With 5-minute batching:**
```
500,000 changes/day
÷ 288 batches/day (24h × 12 per hour)
= 1736 changes per batch

Serial increments: 288/day
NOTIFY rate: 12/hour ✅
Propagation delay: Up to 5 minutes (acceptable for most operations)
```

---

## Conclusion & Recommendation

### ✅ Strongly Recommend Time-Based Batching

**Reasons:**
1. Solves the 99/day serial limit problem
2. Eliminates lock contention
3. Predictable load on secondary DNS servers
4. Stays in human-readable YYYYMMDDnn format
5. Natural rate limiting
6. Better operational visibility

**Optimal Configuration:**
```go
Production Config:
- Batch interval: 1 minute (good balance)
- Max batch size: 10,000 changes
- Queue type: Database-backed (crash recovery)
- Flush mode: Automatic (on interval) + Manual (on demand for urgent)
- Monitoring: Queue depth, batch size, publish duration
```

**Acceptable Trade-offs:**
- DNS propagation delay: Up to 1 minute (vs instant)
- Memory usage: 20-100 MB (vs zero)
- Complexity: Moderate (vs simple)

**Unacceptable Trade-offs (Mitigated):**
- ❌ Data loss on crash → ✅ Use database queue
- ❌ Unpredictable IXFR size → ✅ Split large batches
- ❌ Can't handle urgent updates → ✅ Hybrid mode (batched + immediate)

### Implementation Priority

**Week 1:**
- Implement database-backed queue
- Add batch publisher with 1-minute interval
- Add monitoring/metrics

**Week 2:**
- Deploy with feature flag (opt-in)
- Test with low-traffic TLD
- Monitor queue depth and batch sizes

**Week 3:**
- Gradual rollout to all TLDs
- Tune interval based on metrics
- Document operational procedures

**Result:** Scalable DNS publishing that handles millions of domains per day while staying within DNS protocol limits! 🚀

---

## Code Integration Example

```go
// internal/application/services/domain_service.go

type DomainService struct {
    domainRepository repositories.DomainRepository
    dnsPublisher     *dnsevents.BatchPublisher  // ← Changed from EventPublisher
    // ...
}

func NewDomainService(...) *DomainService {
    return &DomainService{
        domainRepository: dRepo,
        dnsPublisher:     dnsevents.NewBatchPublisher(db, 1*time.Minute),  // ← 1-minute batching
        // ...
    }
}

func (s *DomainService) Create(ctx context.Context, cmd *commands.CreateDomainCommand) error {
    return db.Transaction(func(tx *gorm.DB) error {
        // Create domain
        domain, err := s.domainRepository.Create(ctx, domain)
        if err != nil {
            return err
        }
        
        // Queue DNS change (non-blocking, returns immediately)
        if s.isDNSEnabled(domain.TLDName) {
            changes := buildDNSChanges(domain)
            for _, change := range changes {
                if err := s.dnsPublisher.QueueChange(ctx, change); err != nil {
                    return err
                }
            }
        }
        
        return nil
    })
}

// Graceful shutdown
func (s *DomainService) Shutdown() {
    s.dnsPublisher.Stop()  // Flush remaining batches
}
```

This approach gives you **the best of both worlds**: scalability of batching with acceptable propagation latency for DNS! 🎯
