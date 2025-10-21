# DNS Zone Service MVP Implementation Plan

**Author:** Domain-OS Team  
**Date:** October 12, 2025  
**Status:** RFC / Implementation Guide  

---

## 🎯 Executive Summary

Build a **CoreDNS-based hidden primary DNS server** that:
- Provides AXFR (full zone transfers) for secondary DNS servers
- Provides IXFR (incremental zone transfers) for efficient updates
- Sends NOTIFY messages to trigger secondary refreshes
- Integrates with domain-os via **application-level events** (no database triggers)
- Starts simple with direct PostgreSQL queries, scales to CDC when needed

**Timeline:** 2 weeks MVP, 4 weeks to production-ready, 8 weeks to CDC (if needed)

**Infrastructure:** Zero new services for MVP (uses existing PostgreSQL)

---

## 📋 Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Phase 1: MVP (No Triggers)](#phase-1-mvp-no-triggers)
3. [Phase 2: Add Caching](#phase-2-add-caching)
4. [Phase 3: CDC Migration](#phase-3-cdc-migration)
5. [Testing Strategy](#testing-strategy)
6. [Operational Guide](#operational-guide)

---

## Architecture Overview

### The Problem We're Solving

**Current State:**
- Domain-OS manages 20M domains across 150 TLDs
- No automated DNS zone generation
- Manual zone file creation via scripts

**Desired State:**
- Real-time DNS zone updates when domains change
- Efficient zone transfers to secondary DNS servers
- Support for AXFR (full) and IXFR (incremental) transfers
- Automated NOTIFY to secondaries

### Solution: Hidden Primary Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         INTERNET                                 │
└────────────────────────────┬─────────────────────────────────────┘
                             │ DNS Queries
                             │ (from end users)
                             ▼
    ┌────────────────────────────────────────────────────┐
    │      Secondary DNS Servers (Public-Facing)         │
    │      • ns1.yourregistry.com (BIND/NSD)            │
    │      • ns2.yourregistry.com (PowerDNS)            │
    │      • ns3.yourregistry.com (Knot DNS)            │
    │                                                     │
    │      These answer all public DNS queries           │
    └────────────┬───────────────────────┬─────────────────┘
                 │                       │
                 │ NOTIFY ◄─────────────┤
                 │ IXFR/AXFR ───────────►
                 │                       │
    ┌────────────┴───────────────────────┴─────────────────┐
    │                                                        │
    │   YOUR DNS ZONE SERVICE (Hidden Primary)              │
    │   ┌────────────────────────────────────────┐         │
    │   │  CoreDNS + PostgreSQL Plugin           │         │
    │   │  • NOT in zone NS records              │         │
    │   │  • NOT accessible from Internet        │         │
    │   │  • Only secondaries connect            │         │
    │   └────────────────────────────────────────┘         │
    │                       │                                │
    └───────────────────────┼────────────────────────────────┘
                            │
                            │ PostgreSQL Connection
                            ▼
    ┌─────────────────────────────────────────────────────┐
    │              domain-os PostgreSQL                    │
    │  • domains, hosts, host_addresses                   │
    │  • dns_zone_serials                                 │
    │  • dns_zone_journal                                 │
    └─────────────────────────────────────────────────────┘
```

**Why Hidden Primary?**
- ✅ Protected from DDoS attacks
- ✅ No public query load
- ✅ Simpler security model
- ✅ Standard industry practice for registries

---

## Phase 1: MVP (No Triggers)

### Design Principles

**NO DATABASE TRIGGERS** - All events published from application code  
**Explicit is better than implicit** - Clear, testable code paths  
**Fail fast** - Errors in DNS events fail the entire transaction  
**Atomic** - DNS events in same TX as domain operations  

### Components

#### 1. Database Schema

**File:** `internal/infrastructure/db/postgres/dns_zone_schema.go`

Two simple tables:
- `dns_zone_serials` - Current serial per zone
- `dns_zone_journal` - Change history for IXFR

Three helper functions:
- `get_next_serial(zone)` - Increment and return serial
- `get_current_serial(zone)` - Read serial without incrementing
- `cleanup_dns_journal(keep_count)` - Prune old entries

**Serial Format:** `YYYYMMDDnn`
- Example: `2025101201` = October 12, 2025, sequence 01
- Allows 99 updates per day per zone
- Overflow to Unix timestamp if exceeded

#### 2. DNS Event Publisher

**File:** `internal/infrastructure/dnsevents/event_publisher.go`

```go
type EventPublisher struct {
    db *gorm.DB
}

// Core method - called from domain service
func PublishChange(ctx, tx, change *DNSChange) error

// Helper methods
func PublishDomainNSRecords(...)  // Batch NS records
func PublishGlueRecords(...)      // Batch A/AAAA records
```

**Key Feature:** Must be called within existing transaction!

```go
// Good - transaction ensures atomicity
db.Transaction(func(tx *gorm.DB) error {
    tx.Create(domain)                    // 1. Domain operation
    dnsPublisher.PublishChange(ctx, tx)  // 2. DNS event (same TX)
    return nil                           // Both commit together
})

// Bad - could have inconsistency
tx.Create(domain)
dnsPublisher.PublishChange(ctx, db)  // Different connection!
```

#### 3. Integration Points in Domain Service

**Modify these existing methods:**

| Method | DNS Events to Publish |
|--------|----------------------|
| `CreateDomain()` | ADD NS records for each host |
| `UpdateDomain()` | ADD/DELETE NS if inactive status changes |
| `DeleteDomain()` | DELETE all NS records |
| `AddHostToDomain()` | ADD NS record |
| `RemoveHostFromDomain()` | DELETE NS record |
| `AddHostAddress()` | ADD A/AAAA (if in-bailiwick) |
| `UpdateHostAddress()` | DELETE old + ADD new |
| `DeleteHostAddress()` | DELETE A/AAAA |

**Example Integration:**

```go
// Before (existing code)
func (svc *DomainService) CreateDomain(ctx context.Context, cmd commands.CreateDomainCommand) (*entities.Domain, error) {
    return svc.domainRepository.Create(ctx, domain)
}

// After (with DNS events)
func (svc *DomainService) CreateDomain(ctx context.Context, cmd commands.CreateDomainCommand) (*entities.Domain, error) {
    var createdDomain *entities.Domain
    
    err := svc.db.Transaction(func(tx *gorm.DB) error {
        // Create domain
        repo := NewDomainRepository(tx)
        domain, err := repo.Create(ctx, domain)
        if err != nil {
            return err
        }
        createdDomain = domain
        
        // Publish DNS events if TLD has DNS enabled
        if svc.isTLDDNSEnabled(ctx, domain.TLDName) && len(domain.Hosts) > 0 {
            hostNames := extractHostNames(domain.Hosts)
            err = svc.dnsPublisher.PublishDomainNSRecords(
                ctx, tx,
                domain.TLDName.String(),
                domain.Name.String(),
                hostNames,
                dnsevents.DNSChangeTypeAdd,
                "CreateDomain",
            )
            if err != nil {
                return err // Rolls back entire transaction
            }
        }
        
        return nil
    })
    
    return createdDomain, err
}
```

#### 4. CoreDNS PostgreSQL Plugin (Basic)

**Repository:** Separate repo `coredns-postgres-plugin`

**Core Interface:**
```go
// Implements CoreDNS plugin.Handler
func (p *PostgresBackend) ServeDNS(ctx, w, r) (int, error) {
    switch qtype {
    case dns.TypeAXFR:
        return p.handleAXFR(w, r)
    case dns.TypeIXFR:
        return p.handleIXFR(w, r)
    case dns.TypeNS, dns.TypeA, dns.TypeAAAA:
        return p.handleQuery(w, r)
    }
}
```

**Database Queries (reuse existing patterns):**
```go
// NS Records - exactly like your GetActiveDomainsWithHosts
SELECT dom.name, ho.name
FROM domains dom
JOIN domain_hosts dh ON dh.domain_ro_id = dom.ro_id
JOIN hosts ho ON dh.host_ro_id = ho.ro_id
WHERE dom.tld_name = ? AND dom.inactive = false

// Glue Records - exactly like your GetActiveDomainGlue  
SELECT ho.name, ha.address, ha.version
FROM domains dom
JOIN domain_hosts dh ON dh.domain_ro_id = dom.ro_id
JOIN hosts ho ON dh.host_ro_id = ho.ro_id
JOIN host_addresses ha ON ho.ro_id = ha.host_ro_id
WHERE dom.tld_name = ? AND ho.in_bailiwick = true

// IXFR Journal Query
SELECT change_type, record_type, record_name, record_data, ttl
FROM dns_zone_journal
WHERE zone_name = ? AND serial > ? AND serial <= ?
ORDER BY serial, id
```

### Testing Strategy

#### Unit Tests (Go)

```go
// Test DNS event publisher
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
        }
        
        return publisher.PublishChange(context.Background(), tx, change)
    })
    
    require.NoError(t, err)
    
    // Verify journal entry
    var count int64
    db.Raw("SELECT COUNT(*) FROM dns_zone_journal WHERE zone_name = 'tld'").Scan(&count)
    assert.Equal(t, int64(1), count)
    
    // Verify serial incremented
    var serial int64
    db.Raw("SELECT serial FROM dns_zone_serials WHERE zone_name = 'tld'").Scan(&serial)
    assert.Greater(t, serial, int64(0))
}

// Test transactional rollback
func TestEventPublisher_RollbackOnError(t *testing.T) {
    db := setupTestDB(t)
    publisher := dnsevents.NewEventPublisher(db)
    
    err := db.Transaction(func(tx *gorm.DB) error {
        // Publish change
        change := &dnsevents.DNSChange{/* ... */}
        publisher.PublishChange(context.Background(), tx, change)
        
        // Force error to trigger rollback
        return fmt.Errorf("intentional error")
    })
    
    require.Error(t, err)
    
    // Verify NO journal entry (rolled back)
    var count int64
    db.Raw("SELECT COUNT(*) FROM dns_zone_journal").Scan(&count)
    assert.Equal(t, int64(0), count)
}
```

#### Integration Tests (DNS Protocol)

```bash
# Test AXFR
dig @localhost -p 5353 tld. AXFR

# Expected output:
# tld.  3600  IN  SOA  ns1.tld. hostmaster.tld. 2025101201 3600 600 604800 86400
# example.tld. 3600 IN NS ns1.example.com.
# example.tld. 3600 IN NS ns2.example.com.
# ns1.example.tld. 3600 IN A 192.0.2.1
# tld.  3600  IN  SOA  ns1.tld. hostmaster.tld. 2025101201 3600 600 604800 86400

# Test IXFR
dig @localhost -p 5353 tld. IXFR=2025101200

# Test NOTIFY reception (from your server)
# Use tcpdump to verify NOTIFY messages sent
```

### Deployment Steps

**1. Apply Database Migration**
```bash
# Development
make migrate-dev

# Or manually
psql -h localhost -U domain_os -d domain_registry -f internal/infrastructure/db/postgres/dns_zone_schema.sql
```

**2. Update Domain Service**
```go
// In your domain service initialization
type DomainService struct {
    domainRepository repositories.DomainRepository
    dnsPublisher     *dnsevents.EventPublisher  // NEW
    // ... other fields
}

func NewDomainService(db *gorm.DB, ...) *DomainService {
    return &DomainService{
        domainRepository: postgres.NewDomainRepository(db),
        dnsPublisher:     dnsevents.NewEventPublisher(db), // NEW
        // ...
    }
}
```

**3. Add DNS Events to Methods**

See `domain_service_dns_integration_example.go` for patterns.

**4. Build CoreDNS Plugin**
```bash
# Create separate repository
git clone https://github.com/coredns/coredns.git
cd coredns

# Add your plugin
echo "postgres:github.com/yourorg/coredns-postgres" >> plugin.cfg

go generate
go build
```

**5. Configure and Run**
```bash
# Corefile
cat > Corefile <<EOF
tld example demo {
    postgres {
        connection "postgres://coredns:pass@localhost:5432/domain_registry"
    }
    transfer {
        to * 192.0.2.0/24
    }
    log
}
EOF

./coredns -conf Corefile
```

---

## Phase 2: Add Caching (Week 3)

### When to Upgrade
- Query rate > 500 QPS
- Database connection pool saturated
- Query latency > 50ms p95

### Changes Required

**Corefile only:**
```
tld example demo {
    cache 300 {
        success 9984 30 300  # Cache successful responses 30-300s
        denial 9984 30 60    # Cache NXDOMAIN 30-60s
    }
    postgres {
        connection "postgres://..."
    }
    transfer {
        to * 192.0.2.0/24
    }
}
```

**Result:**
- 10-100x reduction in database queries
- Sub-millisecond response time for cached queries
- Handles 5,000-10,000 QPS

---

## Phase 3: CDC Migration (Month 2-3)

### When to Upgrade
- Total domains > 5M
- Updates > 1000/hour
- Need multi-region deployment
- Database CPU > 70%

### Migration Steps

**1. Enable PostgreSQL Logical Replication**
```sql
ALTER SYSTEM SET wal_level = logical;
-- Restart PostgreSQL

CREATE PUBLICATION coredns_pub FOR TABLE domains, hosts, host_addresses, domain_hosts;
SELECT pg_create_logical_replication_slot('coredns_slot', 'pgoutput');
```

**2. Update CoreDNS Plugin**
- Add CDC consumer component
- Build in-memory zone trees
- Subscribe to WAL stream

**3. Update Corefile**
```
tld {
    postgres {
        connection "postgres://..."
        enable_cdc true              # NEW
        replication_slot "coredns_slot"
    }
}
```

**4. Benefits After Migration**
- Zero query latency (in-memory)
- 100K+ QPS capacity
- Multi-region deployment ready
- Minimal database load

---

## Testing Strategy

### Unit Tests

**DNS Event Publisher:**
```bash
go test ./internal/infrastructure/dnsevents/... -v
```

Tests:
- ✅ Serial increment (same day, new day, overflow)
- ✅ Journal entry creation
- ✅ Transaction rollback (no orphaned events)
- ✅ Batch publishing
- ✅ Validation logic

**Domain Service Integration:**
```bash
go test ./internal/application/services/... -run TestDomain.*DNS -v
```

Tests:
- ✅ CreateDomain publishes NS records
- ✅ AddHost publishes NS record
- ✅ DeleteDomain removes NS records
- ✅ Inactive domain doesn't publish events

### Integration Tests

**DNS Protocol Tests:**
```go
func TestDNS_AXFR(t *testing.T) {
    // Start CoreDNS
    // Create test domain via domain-os API
    // Query via AXFR
    // Verify zone data
}

func TestDNS_IXFR(t *testing.T) {
    // Get initial serial
    // Create domain (serial++)
    // Query IXFR with old serial
    // Verify delta only returned
}

func TestDNS_NOTIFY(t *testing.T) {
    // Setup mock secondary
    // Create domain
    // Verify NOTIFY received
}
```

### Load Testing

```bash
# Generate load
dnsperf -s localhost -p 5353 -d queries.txt -l 60

# Monitor
watch -n 1 'psql -c "SELECT * FROM dns_zone_status"'
```

---

## Operational Guide

### Monitoring Queries

**Check zone serials:**
```sql
SELECT * FROM dns_zone_status ORDER BY zone_name;
```

**Check recent changes:**
```sql
SELECT * FROM dns_zone_latest_changes LIMIT 20;
```

**Find high-activity zones:**
```sql
SELECT zone_name, COUNT(*) as changes
FROM dns_zone_journal
WHERE timestamp > NOW() - INTERVAL '1 hour'
GROUP BY zone_name
ORDER BY changes DESC;
```

**Check journal size:**
```sql
SELECT 
    zone_name,
    COUNT(*) as entries,
    MIN(serial) as oldest_serial,
    MAX(serial) as newest_serial,
    pg_size_pretty(pg_total_relation_size('dns_zone_journal')) as total_size
FROM dns_zone_journal
GROUP BY zone_name;
```

### Maintenance Tasks

**Cleanup old journal entries (weekly):**
```sql
SELECT * FROM cleanup_dns_journal(100);  -- Keep last 100 serials
```

**Reset a zone serial (emergency):**
```sql
UPDATE dns_zone_serials 
SET serial = (TO_CHAR(NOW(), 'YYYYMMDD')::BIGINT * 100) + 1
WHERE zone_name = 'tld';
```

**View journal for specific domain:**
```sql
SELECT serial, change_type, record_type, record_name, record_data, source_operation
FROM dns_zone_journal
WHERE domain_name = 'example.tld'
ORDER BY serial DESC
LIMIT 20;
```

### Debugging

**Trace a domain's DNS changes:**
```sql
-- See all DNS events for a domain
SELECT 
    j.serial,
    j.timestamp,
    j.change_type,
    j.record_type,
    j.record_name,
    j.record_data,
    j.source_operation
FROM dns_zone_journal j
WHERE j.domain_name = 'example.tld'
ORDER BY j.timestamp DESC;
```

**Compare journal vs. actual zone state:**
```sql
-- What journal says domain should have
SELECT DISTINCT record_data as nameserver
FROM dns_zone_journal
WHERE domain_name = 'example.tld'
  AND record_type = 'NS'
  AND change_type = 'ADD'
  AND serial = (SELECT MAX(serial) FROM dns_zone_serials WHERE zone_name = 'tld')

EXCEPT

-- What domain actually has
SELECT h.name
FROM domains d
JOIN domain_hosts dh ON d.ro_id = dh.domain_ro_id
JOIN hosts h ON h.ro_id = dh.host_ro_id
WHERE d.name = 'example.tld';
```

### Performance Tuning

**Database:**
```sql
-- Add partial indexes for active domains only
CREATE INDEX idx_domains_dns_active 
    ON domains(tld_name, name) 
    WHERE inactive = false AND pending_delete = false;

-- Add index for in-bailiwick glue lookup
CREATE INDEX idx_hosts_in_bailiwick 
    ON hosts(name) 
    WHERE in_bailiwick = true;
```

**Connection Pooling:**
```go
// CoreDNS plugin config
postgres {
    connection "postgres://..."
    max_connections 50
    max_idle_connections 10
    connection_max_lifetime "1h"
}
```

---

## Comparison: Triggers vs Application Events

| Aspect | Database Triggers | Application Events ✓ |
|--------|------------------|---------------------|
| **Debugging** | Hard - no stack traces | Easy - step through code |
| **Testing** | Hard - need DB integration | Easy - unit testable |
| **Observability** | Limited - DB logs only | Full - app logs + traces |
| **Rollback** | Automatic | Explicit in transaction |
| **Performance** | Slightly faster | Negligible overhead |
| **Maintainability** | SQL in database | Go in codebase |
| **Code Review** | Difficult | Standard PR process |
| **IDE Support** | None | Full (autocomplete, refactor) |

---

## Migration Checklist

### Week 1: Database & Events

- [ ] Create `dns_zone_schema.go` migration file
- [ ] Add schema to `AutoMigrate()` function
- [ ] Create `dnsevents` package with `EventPublisher`
- [ ] Write unit tests for `EventPublisher`
- [ ] Run migration on dev database
- [ ] Verify functions work: `SELECT get_next_serial('test')`

### Week 2: Domain Service Integration

- [ ] Add `dnsPublisher` field to `DomainService`
- [ ] Integrate into `CreateDomain()`
- [ ] Integrate into `AddHostToDomain()`
- [ ] Integrate into `RemoveHostFromDomain()`
- [ ] Integrate into `DeleteDomain()`
- [ ] Integrate into host address methods
- [ ] Write integration tests
- [ ] Test with real domain operations

### Week 3: CoreDNS Plugin (Basic)

- [ ] Create `coredns-postgres` repository
- [ ] Implement `ServeDNS()` handler
- [ ] Implement query methods (NS, A, AAAA)
- [ ] Implement `handleAXFR()`
- [ ] Implement `handleIXFR()`
- [ ] Test with `dig` and `drill`
- [ ] Document Corefile configuration

### Week 4: NOTIFY & Production

- [ ] Implement NOTIFY manager in plugin
- [ ] Configure secondary servers
- [ ] Test full flow: domain update → NOTIFY → IXFR
- [ ] Add monitoring queries
- [ ] Write operational runbook
- [ ] Deploy to staging
- [ ] Load test
- [ ] Deploy to production

---

## Success Metrics

**Functional:**
- ✅ AXFR successfully transfers all zone data
- ✅ IXFR returns only deltas
- ✅ NOTIFY triggers secondary refreshes
- ✅ Serial increments on every change
- ✅ No DNS events lost on failures (rollback works)

**Performance (MVP Target):**
- 📊 Query latency: < 50ms p95
- 📊 Update latency: < 100ms (domain change → serial increment)
- 📊 AXFR time: < 10s for 5M domain zone
- 📊 IXFR time: < 1s for typical delta (100 records)
- 📊 Database overhead: < 5% CPU increase

**Operational:**
- 📈 Zero DNS events lost
- 📈 100% journal-to-reality consistency
- 📈 Journal cleanup runs successfully
- 📈 Clear audit trail for all DNS changes

---

## Future Enhancements (Post-MVP)

**Phase 2: Caching (Week 5)**
- Add CoreDNS cache plugin
- 10-100x query performance improvement

**Phase 3: CDC (Month 2-3)**
- In-memory zone trees
- Sub-millisecond query latency
- 100K+ QPS capacity

**Beyond:**
- DNSSEC support
- Multi-region deployment
- Anycast DNS
- Rate limiting per client
- TSIG authentication for transfers
- Zone signing

---

## Decision: Application Events vs Triggers

### Why Application Events Win

**Development Experience:**
```go
// Explicit and clear
func CreateDomain(...) error {
    return db.Transaction(func(tx) error {
        tx.Create(domain)              // Visible
        publisher.PublishDNS(tx, ...)  // Visible
        return nil
    })
}

// vs. Trigger (hidden magic)
func CreateDomain(...) error {
    return db.Create(domain)  // DNS events happen... somewhere? 🤷
}
```

**Debugging:**
```
Application Events:
  ✓ Set breakpoint in PublishChange()
  ✓ See full stack trace
  ✓ Inspect transaction state
  ✓ Step through logic

Triggers:
  ✗ Enable PostgreSQL logging
  ✗ Parse logs
  ✗ Hope you logged enough context
  ✗ No debugger
```

**Testing:**
```go
// Application Events - easy to test
func TestCreateDomainPublishesDNS(t *testing.T) {
    // Arrange
    svc := NewDomainService(mockDB, mockPublisher)
    
    // Act
    svc.CreateDomain(cmd)
    
    // Assert
    assert.Called(t, mockPublisher, "PublishChange")
}

// Triggers - need full DB integration test
func TestTriggerWorks(t *testing.T) {
    // Need real PostgreSQL
    // Insert domain
    // Query journal table
    // Hope trigger fired
}
```

**The Winner:** Application Events 🏆

---

## Questions & Answers

**Q: Won't this add overhead to every domain operation?**  
A: Minimal (<1ms). The journal INSERT is part of the same transaction, and PostgreSQL is optimized for this. If it becomes an issue, Phase 3 (CDC) eliminates this entirely.

**Q: What if DNS event publishing fails?**  
A: The entire transaction rolls back - domain operation and DNS event are atomic. Either both succeed or both fail.

**Q: How do we ensure we didn't miss publishing events?**  
A: Write a reconciliation job that compares actual zone state vs. journal, runs nightly.

**Q: Can we add events to existing domains?**  
A: Yes! Write a one-time migration script that scans all active domains and publishes their current state to the journal.

**Q: What about performance at 20M domains?**  
A: Phase 1 handles 1-5M easily. At 20M, you're on Phase 3 (CDC) anyway, which handles 100M+.

---

## Next Steps

**Ready to implement?**

1. Review and approve this plan
2. Create branch: `feature/dns-zone-service-mvp`
3. Start with database schema (30 minutes)
4. Build EventPublisher (2 hours)
5. Integrate into one DomainService method as POC (1 hour)
6. Test and iterate

**Want to start now?** I can create the full implementation!
