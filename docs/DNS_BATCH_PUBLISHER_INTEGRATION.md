# DNS Batch Publisher - Integration Guide

## Overview

The DNS Batch Publisher is now implemented and ready for integration into the domain-os application. This document provides step-by-step instructions for:

1. Starting the batch publisher worker in your application
2. Integrating with domain services
3. Testing the integration
4. Monitoring and operations

## Architecture Recap

```
Domain Create/Update/Delete
         |
         v
  BatchPublisher.QueueChange()  <- Writes to dns_change_queue table
         |
         |
     [Database]
         |
         v
  Worker (runs every 1 minute) <- Background goroutine
         |
         v
  flushZone() <- Processes all pending changes
         |
         v
  publishBatch() <- Single serial, writes to dns_zone_journal
```

**Key Benefits:**
- **Durability**: Changes survive crashes (queue persists in database)
- **Efficiency**: Single serial per batch (1000 changes = 1 serial)
- **Predictability**: Fixed 1-minute publishing cadence
- **Scalability**: Handles 20M domains without serial overflow

## Step 1: Worker Startup Integration

### Option A: Application Main (Recommended)

Add to your main.go or application initialization:

```go
package main

import (
    "github.com/onasunnymorning/domain-os/internal/infrastructure/dnsevents"
    // ... other imports
)

func main() {
    // ... existing initialization code
    
    // Initialize database connection
    db, err := postgres.NewConnection(cfg)
    if err != nil {
        log.Fatal().Err(err).Msg("Failed to connect to database")
    }
    
    // Create and start DNS batch publisher
    batchPublisher := dnsevents.NewBatchPublisher(db, nil) // nil = use defaults
    if err := batchPublisher.Start(); err != nil {
        log.Fatal().Err(err).Msg("Failed to start DNS batch publisher")
    }
    defer batchPublisher.Stop()
    
    log.Info().Msg("DNS batch publisher started successfully")
    
    // ... continue with server/service initialization
}
```

### Option B: Custom Configuration

For production environments with specific requirements:

```go
config := &dnsevents.BatchPublisherConfig{
    BatchInterval: 2 * time.Minute,  // Flush every 2 minutes instead of 1
    MaxBatchSize:  5000,             // Limit batch size to 5K changes
}

batchPublisher := dnsevents.NewBatchPublisher(db, config)
batchPublisher.Start()
defer batchPublisher.Stop()
```

### Option C: Dependency Injection

If using fx or similar DI framework:

```go
package di

import (
    "go.uber.org/fx"
    "github.com/onasunnymorning/domain-os/internal/infrastructure/dnsevents"
)

var DNSModule = fx.Module("dns",
    fx.Provide(func(db *gorm.DB) *dnsevents.BatchPublisher {
        return dnsevents.NewBatchPublisher(db, nil)
    }),
    fx.Invoke(func(lifecycle fx.Lifecycle, bp *dnsevents.BatchPublisher) {
        lifecycle.Append(fx.Hook{
            OnStart: func(ctx context.Context) error {
                return bp.Start()
            },
            OnStop: func(ctx context.Context) error {
                return bp.Stop()
            },
        })
    }),
)
```

## Step 2: Domain Service Integration

### Update DomainService Constructor

```go
// Before:
type DomainService struct {
    domainRepo  ports.DomainRepository
    hostRepo    ports.HostRepository
    dnsPublisher *dnsevents.EventPublisher
}

// After:
type DomainService struct {
    domainRepo     ports.DomainRepository
    hostRepo       ports.HostRepository
    dnsBatchPublisher *dnsevents.BatchPublisher  // Changed from EventPublisher
}

func NewDomainService(
    domainRepo ports.DomainRepository,
    hostRepo ports.HostRepository,
    batchPublisher *dnsevents.BatchPublisher,  // Inject BatchPublisher
) *DomainService {
    return &DomainService{
        domainRepo:        domainRepo,
        hostRepo:          hostRepo,
        dnsBatchPublisher: batchPublisher,
    }
}
```

### Update CreateDomain Method

```go
func (s *DomainService) CreateDomain(ctx context.Context, cmd *CreateDomainCommand) (*entities.Domain, error) {
    // ... existing domain creation logic
    
    // After domain is saved to database:
    domain, err := s.domainRepo.Save(ctx, domain)
    if err != nil {
        return nil, err
    }
    
    // Queue DNS changes (non-blocking)
    if len(domain.Hosts) > 0 {
        changes := make([]*dnsevents.DNSChange, 0, len(domain.Hosts)*2)
        
        for _, host := range domain.Hosts {
            // NS record
            changes = append(changes, &dnsevents.DNSChange{
                ZoneName:        domain.TLD.String + ".",
                ChangeType:      dnsevents.DNSChangeTypeAdd,
                RecordType:      dnsevents.DNSRecordTypeNS,
                RecordName:      domain.Name.FullyQualifiedDomainName(),
                RecordData:      host.Name,
                TTL:             3600,
                SourceOperation: "CreateDomain",
                DomainName:      domain.Name.FullyQualifiedDomainName(),
            })
            
            // A/AAAA glue records
            for _, addr := range host.HostAddresses {
                recordType := dnsevents.DNSRecordTypeA
                if strings.Contains(addr.String, ":") {
                    recordType = dnsevents.DNSRecordTypeAAAA
                }
                
                changes = append(changes, &dnsevents.DNSChange{
                    ZoneName:        domain.TLD.String + ".",
                    ChangeType:      dnsevents.DNSChangeTypeAdd,
                    RecordType:      recordType,
                    RecordName:      host.Name,
                    RecordData:      addr.String,
                    TTL:             3600,
                    SourceOperation: "CreateDomain",
                    DomainName:      domain.Name.FullyQualifiedDomainName(),
                })
            }
        }
        
        // Queue all changes at once
        if err := s.dnsBatchPublisher.QueueChanges(ctx, changes); err != nil {
            log.Error().Err(err).Msg("Failed to queue DNS changes")
            // Don't fail domain creation, but log the error
        }
    }
    
    return domain, nil
}
```

### Update UpdateDomain Method

```go
func (s *DomainService) UpdateDomain(ctx context.Context, cmd *UpdateDomainCommand) error {
    // ... existing update logic
    
    // Queue DNS changes for modifications
    if hostsChanged {
        // Queue DELETE for old hosts
        for _, oldHost := range oldHosts {
            s.dnsBatchPublisher.QueueChange(ctx, &dnsevents.DNSChange{
                ZoneName:        domain.TLD.String + ".",
                ChangeType:      dnsevents.DNSChangeTypeDelete,
                RecordType:      dnsevents.DNSRecordTypeNS,
                RecordName:      domain.Name.FullyQualifiedDomainName(),
                RecordData:      oldHost.Name,
                TTL:             3600,
                SourceOperation: "UpdateDomain",
                DomainName:      domain.Name.FullyQualifiedDomainName(),
            })
        }
        
        // Queue ADD for new hosts
        for _, newHost := range newHosts {
            s.dnsBatchPublisher.QueueChange(ctx, &dnsevents.DNSChange{
                ZoneName:        domain.TLD.String + ".",
                ChangeType:      dnsevents.DNSChangeTypeAdd,
                RecordType:      dnsevents.DNSRecordTypeNS,
                RecordName:      domain.Name.FullyQualifiedDomainName(),
                RecordData:      newHost.Name,
                TTL:             3600,
                SourceOperation: "UpdateDomain",
                DomainName:      domain.Name.FullyQualifiedDomainName(),
            })
        }
    }
    
    return nil
}
```

### Update DeleteDomain Method

```go
func (s *DomainService) DeleteDomain(ctx context.Context, domainName string) error {
    // ... existing deletion logic
    
    // Queue DNS deletion
    for _, host := range domain.Hosts {
        s.dnsBatchPublisher.QueueChange(ctx, &dnsevents.DNSChange{
            ZoneName:        domain.TLD.String + ".",
            ChangeType:      dnsevents.DNSChangeTypeDelete,
            RecordType:      dnsevents.DNSRecordTypeNS,
            RecordName:      domain.Name.FullyQualifiedDomainName(),
            RecordData:      host.Name,
            TTL:             3600,
            SourceOperation: "DeleteDomain",
            DomainName:      domain.Name.FullyQualifiedDomainName(),
        })
    }
    
    return nil
}
```

## Step 3: Testing the Integration

### Manual Testing Steps

1. **Start the application with batch publisher:**
   ```bash
   go run cmd/api/main.go
   ```
   
   Look for log message:
   ```
   INFO DNS batch publisher started batch_interval=1m0s max_batch_size=10000
   INFO DNS batch worker started interval=1m0s
   ```

2. **Create a test domain:**
   ```bash
   curl -X POST http://localhost:8080/domains \
     -H "Content-Type: application/json" \
     -d '{
       "name": "example.test",
       "hosts": ["ns1.example.test", "ns2.example.test"]
     }'
   ```

3. **Verify queue table:**
   ```sql
   SELECT zone_name, record_name, record_data, queued_at, published_at 
   FROM dns_change_queue 
   ORDER BY queued_at DESC 
   LIMIT 10;
   ```
   
   Should show:
   - `published_at IS NULL` (not yet published)
   - Multiple entries for the domain

4. **Wait 1 minute (or trigger manual flush)**

5. **Verify publishing:**
   ```sql
   -- Check queue marked as published
   SELECT COUNT(*) FROM dns_change_queue WHERE published_at IS NOT NULL;
   
   -- Check journal entries
   SELECT zone_name, serial, record_name, record_data, timestamp 
   FROM dns_zone_journal 
   ORDER BY timestamp DESC 
   LIMIT 10;
   ```
   
   Should show:
   - Queue items marked as published
   - Journal entries with same serial

6. **Check serial increment:**
   ```sql
   SELECT zone_name, serial, updated_at 
   FROM dns_zone_serials;
   ```

### Automated Integration Test

```go
func TestDomainServiceDNSIntegration(t *testing.T) {
    // Setup
    db := setupTestDB()
    defer cleanupTestDB(db)
    
    batchPublisher := dnsevents.NewBatchPublisher(db, &dnsevents.BatchPublisherConfig{
        BatchInterval: 100 * time.Millisecond, // Fast for testing
        MaxBatchSize:  100,
    })
    batchPublisher.Start()
    defer batchPublisher.Stop()
    
    domainService := NewDomainService(domainRepo, hostRepo, batchPublisher)
    
    // Test
    domain, err := domainService.CreateDomain(ctx, &CreateDomainCommand{
        Name: "example.test",
        Hosts: []string{"ns1.example.test", "ns2.example.test"},
    })
    require.NoError(t, err)
    
    // Verify queued
    var queueCount int64
    db.Raw("SELECT COUNT(*) FROM dns_change_queue WHERE domain_name = ?", 
        "example.test.").Scan(&queueCount)
    assert.Greater(t, queueCount, int64(0))
    
    // Wait for batch processing
    time.Sleep(200 * time.Millisecond)
    
    // Verify published
    var publishedCount int64
    db.Raw("SELECT COUNT(*) FROM dns_change_queue WHERE domain_name = ? AND published_at IS NOT NULL", 
        "example.test.").Scan(&publishedCount)
    assert.Equal(t, queueCount, publishedCount)
    
    // Verify journal
    var journalCount int64
    db.Raw("SELECT COUNT(*) FROM dns_zone_journal WHERE domain_name = ?", 
        "example.test.").Scan(&journalCount)
    assert.Equal(t, queueCount, journalCount)
}
```

## Step 4: Monitoring and Operations

### Monitoring Queries

**Queue Health:**
```sql
-- Pending changes by zone
SELECT zone_name, COUNT(*) as pending_count, MIN(queued_at) as oldest
FROM dns_change_queue
WHERE published_at IS NULL
GROUP BY zone_name
ORDER BY pending_count DESC;
```

**Error Tracking:**
```sql
-- Failed queue items
SELECT zone_name, record_name, error_count, last_error, last_error_at
FROM dns_change_queue
WHERE error_count > 0
ORDER BY last_error_at DESC;
```

**Publishing Rate:**
```sql
-- Publishing rate (last hour)
SELECT 
    DATE_TRUNC('minute', published_at) as minute,
    COUNT(*) as changes_published
FROM dns_change_queue
WHERE published_at >= NOW() - INTERVAL '1 hour'
GROUP BY minute
ORDER BY minute DESC;
```

**Queue Stats View:**
```sql
-- Use built-in stats view
SELECT * FROM dns_queue_stats 
ORDER BY pending_count DESC;
```

### Operational Commands

**Manual Flush (for testing):**
```go
// In application code or admin endpoint
batchPublisher.flushAll()
```

**Cleanup Old Published Items:**
```go
// Delete published items older than 7 days
deleted, err := batchPublisher.CleanupPublished(ctx, 7)
log.Info().Int64("deleted", deleted).Msg("Cleaned up published queue items")
```

**Check Queue Statistics:**
```go
stats, err := batchPublisher.GetQueueStats(ctx)
for _, stat := range stats {
    log.Info().
        Str("zone", stat.ZoneName).
        Int64("pending", stat.PendingCount).
        Int64("published", stat.PublishedCount).
        Int64("errors", stat.ErrorCount).
        Msg("Queue stats")
}
```

### Alerting Recommendations

Set up alerts for:

1. **Queue Depth**: Alert if pending count > 10,000 for any zone
2. **Old Items**: Alert if oldest pending item > 5 minutes
3. **Error Rate**: Alert if error_count > 0 for any items
4. **Worker Health**: Alert if no items published in last 2 minutes (during normal operation)

Example Prometheus metrics (if you add instrumentation):

```
dns_queue_pending_total{zone="test"} 1234
dns_queue_published_total{zone="test"} 56789
dns_batch_flush_duration_seconds{quantile="0.99"} 0.125
dns_batch_size_total 542
```

## Step 5: Graceful Shutdown

Ensure graceful shutdown in your application:

```go
func main() {
    // ... initialization
    
    batchPublisher := dnsevents.NewBatchPublisher(db, nil)
    batchPublisher.Start()
    
    // Setup signal handling
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    // Wait for shutdown signal
    <-sigChan
    log.Info().Msg("Shutting down gracefully...")
    
    // Stop batch publisher (includes final flush)
    if err := batchPublisher.Stop(); err != nil {
        log.Error().Err(err).Msg("Error stopping batch publisher")
    }
    
    // ... other cleanup
}
```

The `Stop()` method:
- Stops the worker goroutine
- Performs a final flush of all pending changes
- Waits for in-flight batches to complete
- Returns when shutdown is complete

## Configuration Examples

### Development (Fast Publishing)
```go
config := &dnsevents.BatchPublisherConfig{
    BatchInterval: 5 * time.Second,  // Publish every 5 seconds
    MaxBatchSize:  1000,
}
```

### Production (Balanced)
```go
config := &dnsevents.BatchPublisherConfig{
    BatchInterval: 1 * time.Minute,  // Default: 1 minute
    MaxBatchSize:  10000,            // Default: 10K changes
}
```

### High-Volume Bulk Imports
```go
config := &dnsevents.BatchPublisherConfig{
    BatchInterval: 5 * time.Minute,  // Less frequent, larger batches
    MaxBatchSize:  50000,            // Allow larger batches
}
```

## Troubleshooting

### Problem: Changes Not Publishing

**Symptoms:**
- Items stuck in queue with `published_at IS NULL`
- No journal entries being created

**Check:**
1. Worker running: Look for "DNS batch worker started" log message
2. Database connectivity: Verify batch publisher can query database
3. Errors in logs: Search for "Failed to flush" or "Failed to publish batch"
4. PostgreSQL functions: Verify `get_next_serial()` function exists

**Solution:**
```sql
-- Verify function exists
SELECT proname FROM pg_proc WHERE proname = 'get_next_serial';

-- Manually trigger flush (if worker stopped)
-- Restart application or manually flush via admin endpoint
```

### Problem: Serial Not Incrementing

**Symptoms:**
- Multiple batches with same serial
- `dns_zone_serials` not updating

**Check:**
```sql
-- Check serial table
SELECT * FROM dns_zone_serials;

-- Check journal for duplicate serials
SELECT serial, COUNT(*) 
FROM dns_zone_journal 
WHERE zone_name = 'test' 
GROUP BY serial 
HAVING COUNT(*) > 1;
```

**Solution:**
- Verify `get_next_serial()` function logic
- Check for transaction rollbacks in logs

### Problem: Worker Consuming Too Much CPU

**Symptoms:**
- High CPU usage from batch publisher
- Frequent database queries

**Check:**
- BatchInterval too short (< 5 seconds)
- Large number of zones with pending changes

**Solution:**
- Increase BatchInterval
- Reduce MaxBatchSize to process in smaller chunks
- Add rate limiting or backoff

## Next Steps

After integration is complete:

1. **Phase 2**: Implement CoreDNS plugin to serve zones from `dns_zone_journal`
2. **Phase 3**: Add NOTIFY support to signal secondary nameservers
3. **Phase 4**: Implement IXFR support in CoreDNS plugin
4. **Phase 5**: Add monitoring dashboard and metrics

## Summary

✅ **Completed:**
- Database schema with queue table
- Batch publisher implementation
- Unit tests
- Integration patterns

⏳ **Next:**
- Integrate into main application startup
- Update domain service to use batch publisher
- Test end-to-end flow
- Set up monitoring

**Key Benefits of This Implementation:**
- **Durability**: Queue survives crashes
- **Efficiency**: 1 serial per batch (not per change)
- **Scalability**: Handles 20M domains without overflow
- **Predictability**: Fixed 1-minute publishing cadence
- **Observability**: Queue stats and monitoring built-in
