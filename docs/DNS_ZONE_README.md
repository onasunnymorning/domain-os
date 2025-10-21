# DNS Zone Service - Implementation Files

This directory contains the implementation for the DNS Zone Service MVP using **application-layer events** (no database triggers).

## 📁 Files Created

### Core Implementation

1. **`internal/infrastructure/db/postgres/dns_zone_schema.go`**
   - Database schema for DNS zone management
   - Tables: `dns_zone_serials`, `dns_zone_journal`
   - Functions: `get_next_serial()`, `get_current_serial()`, `cleanup_dns_journal()`
   - Views: `dns_zone_status`, `dns_zone_latest_changes`

2. **`internal/infrastructure/dnsevents/event_publisher.go`**
   - `EventPublisher` service for publishing DNS changes
   - Methods: `PublishChange()`, `PublishDomainNSRecords()`, `PublishGlueRecords()`
   - Helper utilities for DNS event management

3. **`internal/application/services/domain_service_dns_integration_example.go`**
   - Examples of how to integrate DNS events into existing Domain Service methods
   - Patterns for CreateDomain, AddHost, DeleteDomain, etc.

### Documentation

4. **`docs/DNS_ZONE_MVP_PLAN.md`**
   - Complete MVP implementation plan
   - Phase 1: Application events (weeks 1-2)
   - Phase 2: Add caching (week 3)
   - Phase 3: CDC migration (months 2-3)
   - Testing strategy, operational guide, monitoring queries

5. **`docs/DNS_ZONE_TRIGGER_VS_EVENTS.md`**
   - Detailed comparison: Database Triggers vs Application Events
   - Side-by-side code examples
   - Why application events are superior
   - Real-world scenarios and debugging flows

6. **`docs/DNS_ZONE_README.md`** (this file)
   - Overview and quick start guide

## 🚀 Quick Start

### 1. Apply Database Migration

The migration runs automatically with `AutoMigrate`:

```bash
# Start your application with AutoMigrate enabled
# The DNS zone schema will be created automatically
```

Or manually:
```bash
psql -h localhost -U domain_os -d domain_registry
# Then in psql:
\i internal/infrastructure/db/postgres/dns_zone_schema.go
```

### 2. Verify Schema

```sql
-- Check tables exist
\dt dns_zone_*

-- Check functions exist
\df get_next_serial
\df get_current_serial
\df cleanup_dns_journal

-- Check views exist
\dv dns_zone_*

-- View zone status
SELECT * FROM dns_zone_status;
```

### 3. Integrate into Domain Service

```go
// Add DNS publisher to your DomainService
type DomainService struct {
    db              *gorm.DB
    domainRepo      repositories.DomainRepository
    dnsPublisher    *dnsevents.EventPublisher  // ADD THIS
    // ... other fields
}

// Initialize in constructor
func NewDomainService(db *gorm.DB, ...) *DomainService {
    return &DomainService{
        db:           db,
        domainRepo:   postgres.NewDomainRepository(db),
        dnsPublisher: dnsevents.NewEventPublisher(db),  // ADD THIS
        // ...
    }
}
```

### 4. Publish DNS Events

See `domain_service_dns_integration_example.go` for complete examples. Basic pattern:

```go
func (svc *DomainService) CreateDomain(ctx context.Context, domain *entities.Domain) error {
    return svc.db.Transaction(func(tx *gorm.DB) error {
        // 1. Domain operation
        repo := NewDomainRepository(tx)
        created, err := repo.Create(ctx, domain)
        if err != nil {
            return err
        }
        
        // 2. DNS events (if DNS enabled)
        if svc.isDNSEnabled(domain.TLDName) {
            hostNames := getHostNames(domain.Hosts)
            err = svc.dnsPublisher.PublishDomainNSRecords(
                ctx, tx,              // Pass transaction!
                domain.TLDName.String(),
                domain.Name.String(),
                hostNames,
                dnsevents.DNSChangeTypeAdd,
                "CreateDomain",
            )
            if err != nil {
                return err
            }
        }
        
        return nil
    })
}
```

### 5. Test It

```sql
-- Create a test domain (trigger DNS events in your app)

-- Check serial incremented
SELECT * FROM dns_zone_serials WHERE zone_name = 'tld';

-- Check journal entries
SELECT * FROM dns_zone_journal 
WHERE zone_name = 'tld' 
ORDER BY serial DESC 
LIMIT 10;

-- Check zone status view
SELECT * FROM dns_zone_status;
```

## 📊 Monitoring Queries

### Check Recent DNS Activity

```sql
-- Activity in last hour per zone
SELECT 
    zone_name,
    COUNT(*) as changes,
    COUNT(DISTINCT serial) as serial_increments,
    MIN(timestamp) as first_change,
    MAX(timestamp) as last_change
FROM dns_zone_journal
WHERE timestamp > NOW() - INTERVAL '1 hour'
GROUP BY zone_name
ORDER BY changes DESC;
```

### View Zone Status

```sql
-- Using the built-in view
SELECT * FROM dns_zone_status ORDER BY zone_name;

-- Or manually
SELECT 
    zs.zone_name,
    zs.serial as current_serial,
    zs.updated_at,
    COUNT(j.id) as total_journal_entries
FROM dns_zone_serials zs
LEFT JOIN dns_zone_journal j ON j.zone_name = zs.zone_name
GROUP BY zs.zone_name, zs.serial, zs.updated_at;
```

### Audit Specific Domain

```sql
-- All DNS changes for a domain
SELECT 
    serial,
    timestamp,
    change_type,
    record_type,
    record_name,
    record_data,
    source_operation
FROM dns_zone_journal
WHERE domain_name = 'example.tld'
ORDER BY timestamp DESC;
```

### Journal Size Management

```sql
-- Check journal size per zone
SELECT 
    zone_name,
    COUNT(*) as entries,
    pg_size_pretty(pg_relation_size('dns_zone_journal')) as table_size,
    MIN(serial) as oldest_serial,
    MAX(serial) as newest_serial
FROM dns_zone_journal
GROUP BY zone_name;

-- Cleanup old entries (keep last 100 serials)
SELECT * FROM cleanup_dns_journal(100);
```

## 🧪 Testing

### Unit Tests

```go
func TestEventPublisher_PublishChange(t *testing.T) {
    db := setupTestDB(t)
    publisher := dnsevents.NewEventPublisher(db)
    
    err := db.Transaction(func(tx *gorm.DB) error {
        change := &dnsevents.DNSChange{
            ZoneName:   "tld",
            ChangeType: dnsevents.DNSChangeTypeAdd,
            RecordType: dnsevents.DNSRecordTypeNS,
            RecordName: "example.tld.",
            RecordData: "ns1.example.com.",
            TTL:        3600,
            SourceOperation: "TestCreate",
        }
        return publisher.PublishChange(context.Background(), tx, change)
    })
    
    assert.NoError(t, err)
    
    // Verify journal entry
    var count int64
    db.Raw("SELECT COUNT(*) FROM dns_zone_journal WHERE zone_name = 'tld'").Scan(&count)
    assert.Equal(t, int64(1), count)
}
```

### Integration Tests

```go
func TestDomainService_CreateDomain_PublishesDNSEvents(t *testing.T) {
    db := setupTestDB(t)
    svc := NewDomainService(db, ...)
    
    domain := &entities.Domain{
        Name:    "example.tld",
        TLDName: "tld",
        Hosts: []*entities.Host{
            {Name: "ns1.example.com"},
            {Name: "ns2.example.com"},
        },
    }
    
    err := svc.CreateDomain(context.Background(), domain)
    require.NoError(t, err)
    
    // Verify DNS events published
    var events []struct {
        RecordName string
        RecordData string
    }
    db.Raw(`
        SELECT record_name, record_data 
        FROM dns_zone_journal 
        WHERE domain_name = 'example.tld'
    `).Scan(&events)
    
    assert.Len(t, events, 2)
    assert.Equal(t, "example.tld.", events[0].RecordName)
}
```

## 🔧 Troubleshooting

### Events Not Appearing in Journal

**Check 1: Is TLD DNS-enabled?**
```sql
SELECT name, enable_dns FROM tlds WHERE name = 'tld';
```

**Check 2: Was transaction committed?**
```go
// Make sure you're using the transaction!
return db.Transaction(func(tx *gorm.DB) error {
    // Use 'tx' not 'db'!
    publisher.PublishChange(ctx, tx, change)  // ✓ Correct
    publisher.PublishChange(ctx, db, change)  // ✗ Wrong!
})
```

**Check 3: Check for errors**
```go
// Don't ignore errors!
err := publisher.PublishChange(ctx, tx, change)
if err != nil {
    log.Error().Err(err).Msg("Failed to publish DNS event")
    return err  // This rolls back the transaction
}
```

### Serial Not Incrementing

```sql
-- Check if get_next_serial function exists
\df get_next_serial

-- Test the function directly
SELECT get_next_serial('tld');

-- Check dns_zone_serials table
SELECT * FROM dns_zone_serials WHERE zone_name = 'tld';
```

### Journal Growing Too Large

```sql
-- Check current size
SELECT 
    pg_size_pretty(pg_total_relation_size('dns_zone_journal')) as total_size,
    COUNT(*) as total_entries
FROM dns_zone_journal;

-- Cleanup old entries (run weekly)
SELECT * FROM cleanup_dns_journal(100);

-- Or set up automatic cleanup with pg_cron
SELECT cron.schedule(
    'cleanup-dns-journal',
    '0 2 * * *',  -- Daily at 2 AM
    'SELECT cleanup_dns_journal(100)'
);
```

## 📚 Additional Resources

- **[DNS_ZONE_MVP_PLAN.md](./DNS_ZONE_MVP_PLAN.md)** - Complete implementation guide
- **[DNS_ZONE_TRIGGER_VS_EVENTS.md](./DNS_ZONE_TRIGGER_VS_EVENTS.md)** - Why we chose application events
- RFC 5936 - DNS Zone Transfer Protocol (AXFR)
- RFC 1995 - Incremental Zone Transfer in DNS (IXFR)
- RFC 1996 - A Mechanism for Prompt Notification of Zone Changes (NOTIFY)

## 🎯 Next Steps

1. **Phase 1 Complete (Weeks 1-2)**
   - ✅ Database schema created
   - ✅ Event publisher implemented
   - ✅ Integration examples provided
   - 🔲 Integrate into actual Domain Service methods
   - 🔲 Write comprehensive tests
   - 🔲 Deploy to dev/staging

2. **Build CoreDNS Plugin (Weeks 3-4)**
   - Create `coredns-postgres` repository
   - Implement query handlers
   - Implement AXFR/IXFR handlers
   - Test with secondary DNS servers

3. **Production Deployment (Week 5)**
   - Configure secondary DNS servers
   - Set up monitoring and alerts
   - Load testing
   - Production rollout

4. **Future Enhancements**
   - Phase 2: Add caching (week 6)
   - Phase 3: CDC migration (months 2-3)
   - DNSSEC support
   - Multi-region deployment

## 💬 Questions?

Check the detailed documentation:
- Architecture questions → `DNS_ZONE_MVP_PLAN.md`
- "Why not triggers?" → `DNS_ZONE_TRIGGER_VS_EVENTS.md`
- Integration help → `domain_service_dns_integration_example.go`

Happy DNS-ing! 🚀
