# DNS Serial Overflow & Operational Risk Analysis

## Executive Summary

**TL;DR:** The current implementation has **multiple safety mechanisms** but faces real risks with very high update frequencies. This document analyzes all overflow scenarios and provides mitigation strategies.

### Risk Level Assessment

| Risk Category | Current Risk Level | Mitigation Status |
|---------------|-------------------|-------------------|
| YYYYMMDDnn overflow (>99/day) | 🟡 MEDIUM | ✅ Has fallback |
| BIGINT overflow | 🟢 LOW | ✅ Won't happen in our lifetime |
| DNS uint32 overflow | 🔴 HIGH | ⚠️ Needs additional handling |
| Performance degradation | 🟡 MEDIUM | ⚠️ Needs monitoring |
| Journal bloat | 🟡 MEDIUM | ✅ Has cleanup function |

---

## 1. Integer Overflow Analysis

### A. PostgreSQL BIGINT Limits

```sql
-- BIGINT range in PostgreSQL
CREATE TABLE dns_zone_serials (
    serial BIGINT  -- Range: -9,223,372,036,854,775,808 to 9,223,372,036,854,775,807
);
```

**Maximum value:** `9,223,372,036,854,775,807`

#### Scenario 1: YYYYMMDDnn Format
```
Format: YYYYMMDDnn
Example: 2025101299 (Oct 12, 2025, 99th update)

Maximum possible value in this format:
9999123199 (Dec 31, 9999, 99th update)

9,999,123,199 << 9,223,372,036,854,775,807
```

**Verdict:** ✅ **Safe** - YYYYMMDDnn format will never overflow BIGINT (even in year 9999)

#### Scenario 2: Unix Timestamp Fallback
```
Unix timestamp format:
1728762000 (current epoch seconds)

Maximum Unix timestamp before BIGINT overflow:
9,223,372,036,854,775,807 seconds
= ~292 billion years from 1970

Current year 2025 timestamp: ~1,728,762,000
Even in year 3000: ~32,503,680,000
```

**Verdict:** ✅ **Safe** - Unix timestamps won't overflow BIGINT for 292 billion years

### B. DNS Protocol Limitations (THE REAL PROBLEM!)

```go
// DNS SOA record definition (RFC 1035)
type SOA struct {
    Serial  uint32  // ← ONLY 32 BITS!
    // ...
}
```

**Maximum uint32 value:** `4,294,967,295`

#### When Does This Break?

```
Current YYYYMMDDnn serials:
2025101201 ✅ (fits in uint32)
2099123199 ✅ (fits in uint32)
3000010101 ✅ (fits in uint32)
4294123199 ✅ (fits in uint32: 4,294,123,199 < 4,294,967,295)
4295010101 ❌ OVERFLOW! (4,295,010,101 > 4,294,967,295)

Unix timestamp fallback:
1728762000 ✅ (current, fits in uint32)
2147483647 ✅ (Year 2038, max int32)
4294967295 ✅ (Year 2106, max uint32)
4294967296 ❌ OVERFLOW! (Any timestamp after Feb 7, 2106)
```

**Critical Years:**
- **Year 4294** - YYYYMMDDnn format breaks DNS protocol
- **Year 2106** - Unix timestamp fallback breaks DNS protocol

**Verdict:** 🔴 **HIGH RISK** for long-running systems, but we have 69+ years to fix it

---

## 2. Daily Update Limit (99 Updates Per Day)

### Current Implementation

```sql
-- From get_next_serial() function
IF v_current_serial >= v_today AND v_current_serial < v_today + 100 THEN
    -- Same day, increment sequence
    v_new_serial := v_current_serial + 1;
    
    -- Check for overflow (max 99 per day)
    IF v_new_serial >= v_today + 100 THEN
        -- Fallback to Unix timestamp
        v_new_serial := EXTRACT(EPOCH FROM NOW())::BIGINT;
    END IF;
```

### What Happens With Heavy Load?

**Example: 1000 domain creations per second**

```
Time          Serial          Note
────────────────────────────────────────────────────────
00:00:01      2025101201      Update 1
00:00:02      2025101202      Update 2
...
00:01:39      2025101299      Update 99 (LAST in YYYYMMDDnn format)
00:01:40      1728762100      ← Switched to Unix timestamp!
00:01:41      1728762101      Unix timestamp
00:01:42      1728762102      Unix timestamp
...continues with Unix timestamps for rest of day...
```

### Problems With This Approach

#### Problem 1: Format Inconsistency
```sql
SELECT zone_name, serial FROM dns_zone_serials;

zone_name | serial
----------|------------
com       | 2025101215  ← YYYYMMDDnn format
net       | 1728762145  ← Unix timestamp format
org       | 2025101203  ← YYYYMMDDnn format
```

**Impact:**
- ❌ Serial format becomes inconsistent across zones
- ❌ Hard to tell "last update date" from serial alone
- ❌ Monitoring/alerting becomes complicated

#### Problem 2: No Return to YYYYMMDDnn
```
Day 1 (High load):
- Starts: 2025101201
- After 99 updates: 2025101299
- After 100th update: 1728762100 ← Switches to Unix timestamp
- Continues all day: 1728762101, 1728762102, ...

Day 2 (Low load):
- Current serial: 1728848500 (Unix timestamp from yesterday)
- v_today: 2025101300 (Oct 13, 2025)
- Condition check: 1728848500 >= 2025101300? FALSE
- New serial: 2025101301 ← Switches BACK to YYYYMMDDnn!
```

**Impact:** 
- ✅ Actually recovers the next day
- ⚠️ But creates a backwards jump in absolute serial value
  - Yesterday: 1728848500
  - Today: 2025101301
  - Appears to go backwards! (DNS serial comparison is tricky)

#### Problem 3: DNS Serial Comparison (RFC 1982)

DNS uses **serial number arithmetic** (RFC 1982):

```
Serial A is "greater than" Serial B if:
  ((A - B) & 0x80000000) == 0

This creates a circular number space with a maximum comparison distance of 2^31
```

**Example:**
```
Serial Yesterday: 1728848500
Serial Today:     2025101301

Difference: 2025101301 - 1728848500 = 296,252,801

Is this < 2^31 (2,147,483,648)? YES ✅
So DNS sees this as valid progression (not backwards)
```

**Verdict:** ✅ Actually safe for DNS, but confusing for humans

---

## 3. Operational Risk Scenarios

### Scenario A: Bulk Domain Import

**Context:** You import 5 million domains for a new TLD in one day

```bash
# Bulk import script
for i in {1..5000000}; do
    create_domain "domain-$i.tld"
done
```

**What Happens:**
```
Serial progression:
2025101201    (domain 1)
2025101202    (domain 2)
...
2025101299    (domain 99)
1728762100    (domain 100) ← Switches to Unix timestamp
1728762101    (domain 101)
...
1728767099    (domain 5,000,000)

Time taken: ~1.4 hours at 1000 domains/second
Serial range: 1728762100 → 1728767099 (5000 serial increments)
```

**Impacts:**
1. ✅ Works fine (no overflow)
2. ⚠️ Serial format switches to Unix timestamp
3. ⚠️ Journal grows by 5M entries (needs cleanup)
4. ⚠️ NOTIFY storm to secondaries (5000 NOTIFYs in 1.4 hours)

**Mitigation:**
```sql
-- Option 1: Batch updates within single serial
-- Instead of one serial per domain, group them:
BEGIN;
  -- Insert 1000 domains
  SELECT get_next_serial('tld');  -- Only ONE serial increment
  INSERT INTO dns_zone_journal (serial, ...) VALUES
    (current_serial, ...),  -- domain 1
    (current_serial, ...),  -- domain 2
    ...
    (current_serial, ...);  -- domain 1000
COMMIT;

-- This reduces 5M serial increments to 5K
```

### Scenario B: Hot TLD (Continuous High Traffic)

**Context:** .com receives 200 domain operations per second (17.3M per day)

```
Updates per day: 17,280,000
YYYYMMDDnn limit: 99
Overflow time: After update 99 (< 1 second!)
```

**What Happens:**
```
Day 1:
00:00:00.001  2025101201
00:00:00.002  2025101202
...
00:00:00.099  2025101299
00:00:00.100  1728762000  ← Unix timestamp for rest of day
...
23:59:59.999  1728848399  (end of day)

Day 2:
00:00:00.001  2025101301  ← Back to YYYYMMDDnn!
00:00:00.002  2025101302
...
00:00:00.099  2025101399
00:00:00.100  1728934400  ← Unix timestamp again
```

**Impacts:**
1. ✅ Technically works (no crashes)
2. ❌ Serial format flips daily between YYYYMMDDnn and Unix timestamp
3. ❌ Monitoring becomes impossible (can't extract date from serial)
4. ⚠️ Secondary DNS servers receive constant NOTIFYs
5. ⚠️ Journal table grows by 17M rows per day (244 GB/day at ~15KB/row)

### Scenario C: Journal Table Bloat

**Size Calculation:**

```sql
-- Average journal entry size
SELECT 
    pg_size_pretty(pg_total_relation_size('dns_zone_journal')) AS total_size,
    COUNT(*) AS row_count,
    pg_size_pretty(pg_total_relation_size('dns_zone_journal') / NULLIF(COUNT(*), 0)) AS bytes_per_row
FROM dns_zone_journal;

-- Typical row: ~500 bytes (with indexes)

-- 1 million updates:
1,000,000 rows × 500 bytes = 500 MB

-- 17 million updates per day (.com):
17,000,000 rows × 500 bytes = 8.5 GB per day
```

**Without cleanup:**
```
1 month: 255 GB
1 year: 3.1 TB
5 years: 15.5 TB
```

**With cleanup (keep last 100 serials):**
```
100 serials × avg entries per serial = manageable size

But if serials increment per domain:
100 most recent serials might only cover last 100 domains (useless for IXFR!)
```

### Scenario D: Lock Contention

**Problem:** `get_next_serial()` uses `FOR UPDATE` lock

```sql
SELECT serial FROM dns_zone_serials 
WHERE zone_name = ? 
FOR UPDATE;  -- ← Exclusive lock
```

**Under high concurrency:**
```
1000 concurrent domain creations for .com
→ All 1000 transactions queue for the same lock
→ Serial execution (no parallelism)
→ Throughput limited by PostgreSQL serial transaction speed
```

**Measurement:**
```bash
# Test lock contention
pgbench -c 100 -j 10 -T 60 << EOF
BEGIN;
SELECT get_next_serial('com');
INSERT INTO dns_zone_journal (...) VALUES (...);
COMMIT;
EOF

# Expected results:
# Without lock: ~10,000 TPS
# With lock: ~2,000 TPS (5x slower)
```

**Impact:** Becomes a bottleneck at scale

---

## 4. Proposed Solutions

### Solution 1: Increase Daily Limit (Quick Fix)

```sql
-- Change from YYYYMMDDnn (99/day) to YYYYMMDDnnnn (9999/day)
CREATE OR REPLACE FUNCTION get_next_serial(p_zone_name VARCHAR)
RETURNS BIGINT AS $$
DECLARE
    v_current_serial BIGINT;
    v_new_serial BIGINT;
    v_today BIGINT;
BEGIN
    SELECT serial INTO v_current_serial
    FROM dns_zone_serials
    WHERE zone_name = p_zone_name
    FOR UPDATE;
    
    -- YYYYMMDDnnnn format (10000 updates per day)
    v_today := TO_CHAR(NOW(), 'YYYYMMDD')::BIGINT * 10000;
    
    IF v_current_serial >= v_today AND v_current_serial < v_today + 10000 THEN
        v_new_serial := v_current_serial + 1;
        
        IF v_new_serial >= v_today + 10000 THEN
            -- Fallback to Unix timestamp
            v_new_serial := EXTRACT(EPOCH FROM NOW())::BIGINT;
        END IF;
    ELSE
        v_new_serial := v_today + 1;
    END IF;
    
    UPDATE dns_zone_serials
    SET serial = v_new_serial, updated_at = NOW()
    WHERE zone_name = p_zone_name;
    
    RETURN v_new_serial;
END;
$$ LANGUAGE plpgsql;
```

**Format examples:**
```
20251012001   (Oct 12, 2025, 1st update)
20251012002   (Oct 12, 2025, 2nd update)
...
20251019999   (Oct 12, 2025, 9999th update)
20251010000   Would overflow, switches to Unix timestamp
```

**Pros:**
- ✅ 100x more headroom (9999 vs 99 updates/day)
- ✅ Simple change (one function)
- ✅ Still human-readable

**Cons:**
- ⚠️ Still has a daily limit
- ⚠️ Format still switches to Unix timestamp after limit

### Solution 2: Batch Serial Increments (Recommended)

**Concept:** Group multiple changes under a single serial

```go
// internal/infrastructure/dnsevents/event_publisher.go

// PublishChangesAsSingleSerial publishes multiple changes with one serial increment
func (ep *EventPublisher) PublishChangesAsSingleSerial(
    ctx context.Context, 
    tx *gorm.DB, 
    changes []*DNSChange,
) error {
    if len(changes) == 0 {
        return nil
    }
    
    // Get next serial ONCE for all changes
    var serial int64
    err := tx.WithContext(ctx).Raw(
        "SELECT get_next_serial(?)",
        changes[0].ZoneName,
    ).Scan(&serial).Error
    if err != nil {
        return fmt.Errorf("failed to get next serial: %w", err)
    }
    
    // Insert all changes with the SAME serial
    for _, change := range changes {
        err = tx.WithContext(ctx).Exec(`
            INSERT INTO dns_zone_journal (
                zone_name, serial, change_type, record_type,
                record_name, record_data, ttl,
                source_operation, domain_name
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
            change.ZoneName,
            serial,  // ← Same serial for all
            string(change.ChangeType),
            string(change.RecordType),
            change.RecordName,
            change.RecordData,
            change.TTL,
            change.SourceOperation,
            change.DomainName,
        ).Error
        
        if err != nil {
            return fmt.Errorf("failed to insert journal entry: %w", err)
        }
    }
    
    return nil
}
```

**Usage:**
```go
// Bulk import example
func BulkImportDomains(ctx context.Context, domains []*entities.Domain) error {
    // Process in batches of 1000
    batchSize := 1000
    
    for i := 0; i < len(domains); i += batchSize {
        end := min(i+batchSize, len(domains))
        batch := domains[i:end]
        
        db.Transaction(func(tx *gorm.DB) error {
            changes := make([]*dnsevents.DNSChange, 0, len(batch)*2)
            
            // Create all domains
            for _, domain := range batch {
                tx.Create(domain)
                
                // Collect DNS changes
                for _, host := range domain.Hosts {
                    changes = append(changes, &dnsevents.DNSChange{
                        ZoneName:   domain.TLDName.String(),
                        RecordType: dnsevents.DNSRecordTypeNS,
                        RecordName: domain.Name.String() + ".",
                        RecordData: host.Name.String() + ".",
                        // ...
                    })
                }
            }
            
            // ONE serial for entire batch
            return dnsPublisher.PublishChangesAsSingleSerial(ctx, tx, changes)
        })
    }
}
```

**Results:**
```
5M domain import:
- Old way: 5M serial increments
- New way: 5K serial increments (1000 domains per serial)
- Reduction: 1000x fewer serials
```

**Pros:**
- ✅ Dramatically reduces serial churn
- ✅ Stays in YYYYMMDDnn format much longer
- ✅ Reduces NOTIFY frequency to secondaries
- ✅ Smaller journal (fewer distinct serials)

**Cons:**
- ⚠️ Less granular IXFR (secondary gets batch, not individual changes)
- ⚠️ Requires code changes in domain service

### Solution 3: Pure Unix Timestamp (Simpler, Less Pretty)

```sql
CREATE OR REPLACE FUNCTION get_next_serial(p_zone_name VARCHAR)
RETURNS BIGINT AS $$
DECLARE
    v_current_serial BIGINT;
    v_new_serial BIGINT;
BEGIN
    SELECT serial INTO v_current_serial
    FROM dns_zone_serials
    WHERE zone_name = p_zone_name
    FOR UPDATE;
    
    -- Always use Unix timestamp
    v_new_serial := EXTRACT(EPOCH FROM NOW())::BIGINT;
    
    -- Ensure it's greater than current (handle clock drift)
    IF v_new_serial <= v_current_serial THEN
        v_new_serial := v_current_serial + 1;
    END IF;
    
    UPDATE dns_zone_serials
    SET serial = v_new_serial, updated_at = NOW()
    WHERE zone_name = p_zone_name;
    
    RETURN v_new_serial;
END;
$$ LANGUAGE plpgsql;
```

**Pros:**
- ✅ No daily limit
- ✅ No format switching
- ✅ Simple implementation
- ✅ Works until year 2106 (81 years)

**Cons:**
- ❌ Not human-readable (can't tell date from serial)
- ❌ Still has uint32 overflow in year 2106
- ⚠️ Multiple updates in same second need increment logic

### Solution 4: Hybrid with Subsecond Precision

```sql
-- Format: Unix timestamp with milliseconds
-- Example: 1728762123456 (timestamp in milliseconds)

CREATE OR REPLACE FUNCTION get_next_serial(p_zone_name VARCHAR)
RETURNS BIGINT AS $$
DECLARE
    v_current_serial BIGINT;
    v_new_serial BIGINT;
BEGIN
    SELECT serial INTO v_current_serial
    FROM dns_zone_serials
    WHERE zone_name = p_zone_name
    FOR UPDATE;
    
    -- Unix timestamp in milliseconds
    v_new_serial := (EXTRACT(EPOCH FROM NOW()) * 1000)::BIGINT;
    
    -- Ensure monotonic increase
    IF v_new_serial <= v_current_serial THEN
        v_new_serial := v_current_serial + 1;
    END IF;
    
    UPDATE dns_zone_serials
    SET serial = v_new_serial, updated_at = NOW()
    WHERE zone_name = p_zone_name;
    
    RETURN v_new_serial;
END;
$$ LANGUAGE plpgsql;
```

**Serial examples:**
```
1728762123456  (Oct 12, 2025, 15:02:03.456)
1728762123457  (Oct 12, 2025, 15:02:03.457)
...
```

**DNS uint32 compatibility:**
```
Current (2025): 1,728,762,123,456
DNS uint32 max: 4,294,967,295

OVERFLOW! Doesn't fit in uint32!
```

**Solution: Use only last 32 bits for DNS**
```go
// In CoreDNS plugin
func (p *PostgresBackend) buildSOA(zone string, serial int64) *dns.SOA {
    // Truncate to uint32 for DNS compatibility
    dnsSerial := uint32(serial & 0xFFFFFFFF)
    
    return &dns.SOA{
        Serial: dnsSerial,  // Last 32 bits
        // ...
    }
}
```

**Pros:**
- ✅ No daily limit
- ✅ Supports 1000+ updates/second
- ✅ Monotonic even with clock drift

**Cons:**
- ❌ Loses high-order bits when converting to DNS uint32
- ❌ Could cause serial comparison issues if bits wrap

---

## 5. DNS uint32 Overflow Handling (Long-term)

### Problem
```
Year 4294: YYYYMMDDnn = 4,294,123,199 > uint32 max (4,294,967,295)
Year 2106: Unix timestamp > uint32 max
```

### Solution: Modulo Arithmetic

```go
// In CoreDNS plugin (when year > 4294 or 2106)
func convertSerialToDNS(serial int64) uint32 {
    // Use modulo to wrap around uint32 space
    dnsSerial := uint32(serial % 4294967296)
    
    return dnsSerial
}
```

**How it works:**
```
PostgreSQL Serial: 4,295,123,456
DNS Serial: 4,295,123,456 % 4,294,967,296 = 156,160

PostgreSQL Serial: 4,295,123,457
DNS Serial: 4,295,123,457 % 4,294,967,296 = 156,161

Still monotonically increasing in DNS uint32 space!
```

**Caveat:** Loses absolute value, but DNS only cares about relative ordering (RFC 1982)

---

## 6. Recommended Implementation

### Phase 1: Immediate (This Week)

**1. Increase daily limit to 9999**
```sql
-- Apply Solution 1 (YYYYMMDDnnnn format)
-- Change in dns_zone_schema.go
```

**2. Add monitoring**
```sql
-- Alert if serial changes > 1000/hour
CREATE VIEW dns_serial_velocity AS
SELECT 
    zone_name,
    COUNT(*) as changes_last_hour,
    COUNT(*) / 60.0 as changes_per_minute,
    CASE 
        WHEN COUNT(*) > 1000 THEN 'HIGH'
        WHEN COUNT(*) > 100 THEN 'MEDIUM'
        ELSE 'NORMAL'
    END as velocity_status
FROM dns_zone_journal
WHERE timestamp > NOW() - INTERVAL '1 hour'
GROUP BY zone_name;

-- Alert query
SELECT * FROM dns_serial_velocity WHERE velocity_status = 'HIGH';
```

**3. Add journal cleanup cron job**
```sql
-- Run daily
SELECT cron.schedule(
    'dns-journal-cleanup',
    '0 3 * * *',  -- 3 AM daily
    'SELECT cleanup_dns_journal(1000)'  -- Keep last 1000 serials
);
```

### Phase 2: Next Sprint (2 weeks)

**4. Implement batch serial updates**
```go
// Add PublishChangesAsSingleSerial() method (Solution 2)
// Update bulk import scripts to use batching
```

**5. Add rate limiting**
```go
// Prevent runaway serial increments
type SerialRateLimiter struct {
    limit int  // max serials per minute
    window time.Duration
}

func (ep *EventPublisher) PublishChange(...) error {
    // Check rate limit
    if ep.rateLimiter.Exceeded(change.ZoneName) {
        return ErrTooManyUpdates
    }
    
    // Proceed with publish...
}
```

### Phase 3: Future (3-6 months)

**6. Consider pure Unix timestamp**
```sql
-- Apply Solution 3 if YYYYMMDDnnnn still problematic
-- Simpler, no daily limits
```

**7. Implement DNS uint32 conversion**
```go
// Handle year 2106+ gracefully
func buildSOA(serial int64) uint32 {
    if serial > 4294967295 {
        return uint32(serial % 4294967296)
    }
    return uint32(serial)
}
```

---

## 7. Monitoring & Alerting

### Critical Metrics

```sql
-- 1. Serial increment velocity
SELECT 
    zone_name,
    COUNT(*) as increments_today,
    MAX(serial) - MIN(serial) as serial_range
FROM dns_zone_journal
WHERE timestamp::date = CURRENT_DATE
GROUP BY zone_name;

-- Alert if > 5000/day per zone

-- 2. Format detection
SELECT 
    zone_name,
    serial,
    CASE 
        WHEN serial > 20000101 AND serial < 99991231 THEN 'YYYYMMDDnn'
        WHEN serial > 1000000000 AND serial < 4294967295 THEN 'Unix timestamp'
        ELSE 'Unknown'
    END as serial_format
FROM dns_zone_serials;

-- Alert if format switches

-- 3. Journal size
SELECT 
    pg_size_pretty(pg_total_relation_size('dns_zone_journal')) as size,
    COUNT(*) as rows,
    (EXTRACT(EPOCH FROM NOW() - MIN(timestamp)) / 86400)::int as days_of_data
FROM dns_zone_journal;

-- Alert if > 100 GB or > 30 days of data

-- 4. Lock wait time
SELECT 
    pid,
    wait_event,
    state,
    query_start,
    NOW() - query_start as wait_duration,
    query
FROM pg_stat_activity
WHERE wait_event_type = 'Lock'
AND query LIKE '%get_next_serial%';

-- Alert if wait_duration > 5 seconds
```

### Grafana Dashboard

```yaml
panels:
  - title: "Serial Increments/Hour"
    query: |
      SELECT zone_name, COUNT(*) 
      FROM dns_zone_journal 
      WHERE timestamp > NOW() - INTERVAL '1 hour'
      GROUP BY zone_name
      
  - title: "Current Serial Format"
    query: |
      SELECT zone_name, 
        CASE 
          WHEN serial < 99991231 THEN 'Date Format'
          ELSE 'Unix Timestamp'
        END as format
      FROM dns_zone_serials
      
  - title: "Journal Growth Rate (MB/day)"
    query: |
      SELECT 
        zone_name,
        pg_size_pretty(pg_total_relation_size('dns_zone_journal'))
      FROM dns_zone_journal
```

---

## 8. Load Testing

### Test 1: Overflow Threshold

```bash
#!/bin/bash
# Test serial overflow at 99 updates/day

psql -h localhost -U domain_os << EOF
TRUNCATE dns_zone_journal;
UPDATE dns_zone_serials SET serial = (TO_CHAR(NOW(), 'YYYYMMDD')::BIGINT * 100) + 1;

DO \$\$
BEGIN
    FOR i IN 1..150 LOOP
        PERFORM get_next_serial('test');
    END LOOP;
END \$\$;

SELECT serial FROM dns_zone_serials WHERE zone_name = 'test';
-- Should show Unix timestamp after 99 iterations

SELECT serial, COUNT(*) 
FROM dns_zone_journal 
WHERE zone_name = 'test'
GROUP BY serial 
ORDER BY serial;
EOF
```

### Test 2: Concurrent Load

```bash
# Simulate 1000 concurrent domain creations
pgbench -c 100 -j 10 -t 10 << EOF
BEGIN;
SELECT get_next_serial('load-test');
INSERT INTO dns_zone_journal (zone_name, serial, change_type, record_type, record_name, record_data, ttl, source_operation)
VALUES ('load-test', (SELECT serial FROM dns_zone_serials WHERE zone_name = 'load-test'), 'ADD', 'NS', 'test.load-test.', 'ns1.test.', 3600, 'LoadTest');
COMMIT;
EOF

# Measure:
# - Transactions per second
# - Average lock wait time
# - Serial consistency
```

---

## Summary

### Current Risks

| Risk | Likelihood | Impact | Timeline |
|------|-----------|--------|----------|
| Daily limit (99) exceeded | 🔴 High (with bulk imports) | 🟡 Medium (format switch) | Immediate |
| BIGINT overflow | 🟢 None | N/A | Never (292B years) |
| DNS uint32 overflow | 🟢 Low | 🔴 High (breaks DNS) | Year 2106 (81 years) |
| Lock contention | 🟡 Medium (high traffic) | 🟡 Medium (slow writes) | < 1 year at scale |
| Journal bloat | 🔴 High (without cleanup) | 🟡 Medium (disk space) | 3-6 months |

### Recommended Actions (Priority Order)

1. **[This Week]** Increase daily limit to 9999 (Solution 1)
2. **[This Week]** Enable journal cleanup cron job
3. **[This Week]** Add monitoring/alerting
4. **[Next Sprint]** Implement batch serial updates (Solution 2)
5. **[3 months]** Load test at expected scale
6. **[6 months]** Consider pure Unix timestamp (Solution 3)
7. **[Year 2100]** Plan for DNS uint32 overflow mitigation 😄

The system is **safe for immediate production use** with proper monitoring, but needs **batching for high-volume scenarios**. 🎯
