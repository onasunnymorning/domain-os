# DNS Events: Triggers vs Application Layer

## Side-by-Side Comparison

### Scenario: User Creates Domain "example.tld" with 2 Nameservers

---

## Option A: Database Triggers ❌

### Code (Application Layer)
```go
func (svc *DomainService) CreateDomain(ctx context.Context, domain *entities.Domain) error {
    // Just create the domain
    return svc.domainRepository.Create(ctx, domain)
}
```

**Looks simple!** But...

### What Actually Happens (Hidden in Database)

```
Application:
  db.Create(domain) ──────────┐
                              │
                              ▼
PostgreSQL:
  1. INSERT INTO domains       ✓
  2. TRIGGER FIRES             ← Hidden!
     ├─ PL/pgSQL execution
     ├─ Loop through hosts
     ├─ Call increment_serial()
     ├─ INSERT INTO dns_zone_journal × 2
     └─ Done
  3. COMMIT
```

### Debugging Flow

```
Issue: "DNS events not appearing in journal"

Developer's Mental Model:
  "I called CreateDomain, it should just work"
  
Reality:
  1. Check PostgreSQL logs
     tail -f /var/log/postgresql/postgresql.log
     
  2. Enable trigger logging
     ALTER TABLE domains ENABLE TRIGGER ALL;
     
  3. Add debug statements to trigger function
     RAISE NOTICE 'Trigger fired for domain %', NEW.name;
     
  4. Reload function
     DROP TRIGGER trg_domain_dns_change ON domains;
     CREATE TRIGGER trg_domain_dns_change...
     
  5. Test again
  
  6. Discover typo in trigger SQL
  
  7. Fix trigger
  
  8. Redeploy to all environments
     (dev, staging, prod - each needs trigger update)

Time to fix: 2-4 hours
```

### Testing

```go
func TestCreateDomain(t *testing.T) {
    // Need REAL PostgreSQL database
    db := setupRealPostgres(t)
    
    // Need to ensure triggers are installed
    installTriggers(db)
    
    svc := NewDomainService(db)
    domain := createTestDomain()
    
    err := svc.CreateDomain(context.Background(), domain)
    require.NoError(t, err)
    
    // Now query journal table to see if trigger fired
    var count int
    db.Raw("SELECT COUNT(*) FROM dns_zone_journal WHERE domain_name = ?", 
        domain.Name).Scan(&count)
    
    assert.Equal(t, 2, count) // Expected 2 NS records
    
    // If this fails, is it because:
    // - Trigger didn't fire?
    // - Trigger has a bug?
    // - Trigger was never installed?
    // - Test data doesn't match trigger conditions?
    
    // Good luck debugging! 🤷
}
```

---

## Option B: Application Events ✅

### Code (Application Layer)

```go
func (svc *DomainService) CreateDomain(ctx context.Context, domain *entities.Domain) error {
    return svc.db.Transaction(func(tx *gorm.DB) error {
        // 1. Create domain
        repo := NewDomainRepository(tx)
        createdDomain, err := repo.Create(ctx, domain)
        if err != nil {
            return err
        }
        
        // 2. Publish DNS events (EXPLICIT)
        if svc.isTLDDNSEnabled(ctx, domain.TLDName) {
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
                return fmt.Errorf("failed to publish DNS events: %w", err)
            }
        }
        
        return nil
    })
}
```

**More code, but EXPLICIT and VISIBLE**

### What Actually Happens (Transparent)

```
Application:
  db.Transaction(func(tx) {
    1. tx.Create(domain)                    ← Visible
    2. dnsPublisher.PublishNSRecords(tx)   ← Visible
       ├─ get_next_serial(zone)
       ├─ INSERT INTO dns_zone_journal × 2
       └─ return
    3. return nil
  })
  4. COMMIT
```

### Debugging Flow

```
Issue: "DNS events not appearing in journal"

Developer's Mental Model:
  "CreateDomain calls PublishDomainNSRecords"
  
Reality:
  1. Set breakpoint in CreateDomain()
  
  2. Step through:
     ✓ Domain created
     ✓ Check isTLDDNSEnabled() 
       → Oh! TLD has enable_dns = false
  
  3. Fix in test data
  
  4. Done!

Time to fix: 5 minutes
```

### Testing

```go
func TestCreateDomain_PublishesDNSEvents(t *testing.T) {
    // Can use in-memory SQLite or mocked publisher
    db := setupTestDB(t)
    
    // Option 1: Real publisher (integration test)
    publisher := dnsevents.NewEventPublisher(db)
    svc := NewDomainService(db, publisher)
    
    domain := createTestDomain()
    err := svc.CreateDomain(context.Background(), domain)
    require.NoError(t, err)
    
    // Verify journal entry
    var count int64
    db.Raw("SELECT COUNT(*) FROM dns_zone_journal WHERE domain_name = ?", 
        domain.Name).Scan(&count)
    assert.Equal(t, int64(2), count)
    
    // Option 2: Mocked publisher (unit test)
    mockPublisher := new(MockEventPublisher)
    mockPublisher.On("PublishDomainNSRecords", 
        mock.Anything, mock.Anything, 
        "tld", "example.tld", []string{"ns1.example.com", "ns2.example.com"},
        dnsevents.DNSChangeTypeAdd, "CreateDomain",
    ).Return(nil)
    
    svc := NewDomainService(db, mockPublisher)
    err := svc.CreateDomain(context.Background(), domain)
    
    mockPublisher.AssertExpectations(t)
    // Validates the exact parameters passed!
}
```

**Testing is EASY:**
- ✅ Unit test with mocks
- ✅ Integration test with real DB
- ✅ Clear assertions
- ✅ Fast execution (<100ms)

---

## Code Maintenance

### Adding a New DNS Event Type

**With Triggers:**
```sql
-- 1. Write new trigger function (200 lines of PL/pgSQL)
CREATE OR REPLACE FUNCTION trg_new_feature_func() ...

-- 2. Create trigger
CREATE TRIGGER trg_new_feature ...

-- 3. Test in dev environment
-- 4. Apply to staging
-- 5. Apply to production
-- 6. No version control for trigger code (maybe in .sql file?)
-- 7. Hard to review (SQL in migration file)
```

**With Application Events:**
```go
// 1. Add method to EventPublisher (10 lines of Go)
func (ep *EventPublisher) PublishNewFeature(...) error {
    return ep.PublishChange(ctx, tx, &DNSChange{...})
}

// 2. Call from service method
svc.dnsPublisher.PublishNewFeature(ctx, tx, ...)

// 3. Write test
func TestPublishNewFeature(t *testing.T) { ... }

// 4. Git commit, PR review, merge
// 5. Deploy (same as any code change)
```

✅ Version controlled  
✅ Code reviewed  
✅ Tested in CI/CD  
✅ IDE support  

---

## Error Handling

### Triggers: Opaque Errors

```go
err := db.Create(domain)
// Error: "pq: trigger error: invalid zone name"

// What failed?
// - Which trigger?
// - Which line in trigger?
// - What was the zone name?
// - Stack trace? Nope!
```

### Application Events: Clear Errors

```go
err := db.Transaction(func(tx) {
    tx.Create(domain)
    
    err := dnsPublisher.PublishChange(ctx, tx, change)
    // Error: "failed to publish DNS event: zone name is required"
    //        at event_publisher.go:45
    //        validateChange() returned error
    //        ZoneName was empty string
    //        SourceOperation: "CreateDomain"
    //        Domain: "example.tld"
    
    return err
})

// Full context! Clear error message! Stack trace!
```

---

## Observability

### Triggers: Limited

```sql
-- PostgreSQL logs (if you're lucky)
2025-10-12 10:15:34 UTC [12345] LOG: duration: 45.123 ms statement: INSERT INTO domains...

-- No context about DNS events
-- No structured logging
-- No distributed tracing
```

### Application Events: Rich

```json
{
  "timestamp": "2025-10-12T10:15:34Z",
  "level": "info",
  "message": "DNS change published",
  "zone": "tld",
  "serial": 2025101201,
  "change_type": "ADD",
  "record_type": "NS",
  "record_name": "example.tld.",
  "record_data": "ns1.example.com.",
  "operation": "CreateDomain",
  "domain": "example.tld",
  "tx_id": "a1b2c3d4",
  "trace_id": "e5f6g7h8",
  "user": "registrar@example.com",
  "duration_ms": 12.5
}
```

✅ Structured logging  
✅ Distributed tracing  
✅ Metrics (Prometheus)  
✅ Correlation IDs  

---

## Rollback & Recovery

### Triggers: Coupled

```
Scenario: Database restored from backup (2 hours ago)

Problem:
  - Domains table: restored to 2h ago ✓
  - Triggers: still defined ✓
  - dns_zone_journal: restored to 2h ago ✓
  - dns_zone_serials: restored to 2h ago ✓
  
But serials might be inconsistent if:
  - Secondary already transferred newer serial
  - Journal entries lost
  - Serial counter state lost

Fix:
  - Manually recalculate serials
  - Rebuild journal from current state
  - Force AXFR on all secondaries
```

### Application Events: Explicit Recovery

```go
// Reconciliation job (runs nightly or on-demand)
func ReconcileDNSJournal(ctx context.Context, zoneName string) error {
    // 1. Get current zone state from database
    actualRecords := getCurrentZoneState(zoneName)
    
    // 2. Get what journal says state should be
    journalRecords := getJournalState(zoneName)
    
    // 3. Compare and fix
    missing := actualRecords - journalRecords
    extra := journalRecords - actualRecords
    
    // 4. Publish missing events
    for _, record := range missing {
        dnsPublisher.PublishChange(...)
    }
    
    // 5. Mark extra events as deleted
    for _, record := range extra {
        dnsPublisher.PublishChange(..., DNSChangeTypeDelete)
    }
    
    return nil
}
```

✅ Observable  
✅ Testable  
✅ Automated recovery  

---

## Performance: The Reality Check

### Overhead Measurement

**Trigger:**
```
INSERT INTO domains + 2 NS records
  Domain INSERT:       5ms
  Trigger execution:   3ms  ← Trigger overhead
  Journal INSERTs:     2ms
  Total:              10ms
```

**Application Event:**
```
INSERT INTO domains + explicit DNS events
  Domain INSERT:       5ms
  PublishChange call:  0.1ms ← Function call overhead
  Journal INSERTs:     2ms
  Total:              7.1ms
```

**Application events are actually FASTER!**
- No PL/pgSQL interpreter overhead
- No trigger dispatch overhead
- Direct SQL execution

### At Scale (1000 operations/hour)

**Trigger Overhead:**
- 3ms × 1000 = 3,000ms = 3 seconds/hour
- Negligible

**Application Event Overhead:**
- 0.1ms × 1000 = 100ms = 0.1 seconds/hour
- Even more negligible

**Conclusion:** Performance is NOT a reason to choose triggers.

---

## The Verdict

### Use Application Events Because:

1. **Debuggable** - Step through code, set breakpoints
2. **Testable** - Unit tests with mocks, integration tests with real DB
3. **Observable** - Structured logs, metrics, tracing
4. **Maintainable** - Version controlled, code reviewed, refactorable
5. **Explicit** - Clear what happens when
6. **Flexible** - Easy to add conditions, logging, metrics
7. **Safe** - Transactional guarantees same as triggers
8. **Fast** - Actually faster than triggers
9. **Portable** - Not PostgreSQL-specific
10. **Modern** - Event-driven architecture pattern

### Don't Use Triggers Because:

1. ❌ Hard to debug (no debugger, no stack traces)
2. ❌ Hard to test (need full DB integration)
3. ❌ Hard to observe (limited logging)
4. ❌ Hard to maintain (SQL in database, not version controlled well)
5. ❌ Implicit (magic happens, where?)
6. ❌ Inflexible (changing trigger = DDL migration)
7. ❌ Database-specific (PL/pgSQL locks you to PostgreSQL)
8. ❌ Risk of trigger storms (triggers triggering triggers)

---

## Migration Path: Triggers → Application Events

If you already have triggers:

```sql
-- 1. Disable triggers (don't drop yet)
ALTER TABLE domains DISABLE TRIGGER trg_domain_dns_change;
ALTER TABLE domain_hosts DISABLE TRIGGER trg_domain_host_dns_change;

-- 2. Deploy application with event publishing

-- 3. Run in parallel for 1 week (verify both produce same results)

-- 4. Compare outputs:
SELECT * FROM dns_zone_journal 
WHERE source_operation LIKE 'trigger:%'  -- From triggers
EXCEPT
SELECT * FROM dns_zone_journal 
WHERE source_operation NOT LIKE 'trigger:%'  -- From app events

-- 5. If match, drop triggers
DROP TRIGGER trg_domain_dns_change ON domains;
DROP TRIGGER trg_domain_host_dns_change ON domain_hosts;
```

---

## Real-World Examples

### Scenario 1: Add Conditional Logic

**Requirement:** "Don't publish DNS events for premium domains during sunrise phase"

**With Triggers:**
```sql
-- Update trigger function
CREATE OR REPLACE FUNCTION trg_domain_dns_change_func() ...
  -- Add 50 lines of PL/pgSQL
  IF is_premium AND in_sunrise_phase THEN
    RETURN NEW;  -- Skip DNS event
  END IF;
  ...
```
- Need to write PL/pgSQL
- Hard to test conditions
- Deploy trigger update to all environments

**With Application Events:**
```go
// Add 3 lines in service
if domain.IsPremium() && tld.InSunrisePhase() {
    return nil // Skip DNS events
}
```
- Native Go code
- Unit testable
- Deploy with normal code release

### Scenario 2: Add Metrics

**Requirement:** "Track DNS event publishing time and success rate"

**With Triggers:**
```sql
-- Can't easily emit metrics from PL/pgSQL
-- Maybe write to a metrics table?
-- Then poll that table from application?
-- Complex!
```

**With Application Events:**
```go
func (ep *EventPublisher) PublishChange(...) error {
    start := time.Now()
    defer func() {
        metrics.RecordDNSEventDuration(time.Since(start))
    }()
    
    err := ep.publishToJournal(...)
    
    if err != nil {
        metrics.IncrementDNSEventErrors()
    } else {
        metrics.IncrementDNSEventSuccess()
    }
    
    return err
}
```
- Prometheus metrics
- Grafana dashboards
- Alerts on failures

### Scenario 3: Add Audit Trail

**Requirement:** "Log who triggered each DNS change"

**With Triggers:**
```sql
-- Triggers don't have access to application context
-- No user info, no request ID, no trace ID
-- Would need to:
--   1. Add columns to domains table for temporary context
--   2. Application sets context before INSERT
--   3. Trigger reads context
--   4. Complex and error-prone
```

**With Application Events:**
```go
type DNSChange struct {
    // ... existing fields
    TriggeredBy  string  // User email
    RequestID    string  // For correlation
    TraceID      string  // For distributed tracing
}

// Automatically populated from context
func (ep *EventPublisher) PublishChange(ctx, ...) error {
    user := ctx.Value("user").(string)
    requestID := ctx.Value("request_id").(string)
    
    change.TriggeredBy = user
    change.RequestID = requestID
    
    // Insert with full context
}
```

---

## Conclusion

**Application events are superior in every measurable way:**
- Easier to develop
- Easier to test
- Easier to debug
- Easier to maintain
- Easier to observe
- Just as fast
- Just as safe

**The only advantage of triggers:** Less code in application layer

**But that's a false economy** - you pay for it in:
- Debugging time
- Testing complexity
- Production incidents
- Developer frustration

**Recommendation: Application Events 🏆**
