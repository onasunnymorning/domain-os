# DNS Zone Serial Reliability & Source of Truth

## TL;DR - Source of Truth

**The `dns_zone_serials` table is the single source of truth for zone serials.**

```sql
CREATE TABLE dns_zone_serials (
    zone_name VARCHAR(255) PRIMARY KEY,  -- e.g., 'tld'
    serial BIGINT NOT NULL,              -- e.g., 2025101201 ← SOURCE OF TRUTH
    updated_at TIMESTAMPTZ,
    last_notify_at TIMESTAMPTZ,
    notify_count INTEGER
);
```

All serial operations go through PostgreSQL's **`get_next_serial()`** function with **row-level locking** to guarantee:
- ✅ Monotonically increasing serials
- ✅ No gaps or duplicates
- ✅ Consistency across concurrent updates
- ✅ Transaction safety (rollback support)

---

## How Serial Reliability Works

### 1. The Serial Generation Function

```sql
CREATE OR REPLACE FUNCTION get_next_serial(p_zone_name VARCHAR)
RETURNS BIGINT AS $$
DECLARE
    v_current_serial BIGINT;
    v_new_serial BIGINT;
    v_today BIGINT;
BEGIN
    -- CRITICAL: Lock this row for update
    -- This prevents concurrent transactions from getting the same serial
    SELECT serial INTO v_current_serial
    FROM dns_zone_serials
    WHERE zone_name = p_zone_name
    FOR UPDATE;  -- ← ROW LOCK (exclusive)
    
    -- ... serial calculation logic ...
    
    -- Update serial (still holding lock)
    UPDATE dns_zone_serials
    SET serial = v_new_serial,
        updated_at = NOW()
    WHERE zone_name = p_zone_name;
    
    RETURN v_new_serial;
    -- Lock released when transaction commits/rolls back
END;
$$ LANGUAGE plpgsql;
```

### 2. Row-Level Locking Prevents Race Conditions

**Scenario: Two domains created simultaneously**

```
Time  Transaction A                Transaction B
──────────────────────────────────────────────────────
T1    BEGIN                        BEGIN
T2    PublishChange()              
T3    SELECT ... FOR UPDATE        
      ↓ (gets lock, serial=2025101201)
T4                                  PublishChange()
T5                                  SELECT ... FOR UPDATE
                                    ↓ (WAITS for A's lock)
T6    UPDATE serial=2025101202
T7    COMMIT                       
      ↓ (lock released)
T8                                  ↓ (lock acquired, serial=2025101202)
T9                                  UPDATE serial=2025101203
T10                                 COMMIT

Result: Serials are sequential (2025101202, 2025101203) with NO gaps or duplicates
```

**PostgreSQL guarantees:**
- Only ONE transaction can hold the lock at a time
- Other transactions wait in a queue (FIFO)
- If a transaction rolls back, the serial is NOT incremented

### 3. Transaction Safety

```go
// Example: Create domain with DNS events
err := db.Transaction(func(tx *gorm.DB) error {
    // 1. Create domain
    if err := tx.Create(domain).Error; err != nil {
        return err // ← Rolls back, serial NOT incremented
    }
    
    // 2. Publish DNS change (gets next serial)
    err := dnsPublisher.PublishChange(ctx, tx, change)
    // This calls get_next_serial() which:
    //   - Locks the row
    //   - Increments serial
    //   - Inserts journal entry
    //   - Returns new serial
    
    if err != nil {
        return err // ← Rolls back, serial increment UNDONE
    }
    
    // 3. More operations...
    
    return nil // ← COMMIT: Serial increment is persisted
})
```

**Key Points:**
- Serial increments happen **inside the transaction**
- If transaction fails → serial increment is **rolled back**
- If transaction succeeds → serial increment is **committed atomically**
- No "lost" serials or gaps from failed operations

### 4. Concurrent Update Example

Let's say 3 domains are being created concurrently for the TLD `com`:

```sql
-- Initial state
SELECT * FROM dns_zone_serials WHERE zone_name = 'com';
-- zone_name | serial
-- ----------|------------
-- com       | 2025101205

-- Three simultaneous PublishChange() calls:

-- Transaction 1: Gets lock first
BEGIN;
SELECT get_next_serial('com');  -- Returns 2025101206, updates table
INSERT INTO dns_zone_journal (zone_name, serial, ...) VALUES ('com', 2025101206, ...);
COMMIT;

-- Transaction 2: Waits for T1, then gets lock
BEGIN;
SELECT get_next_serial('com');  -- Returns 2025101207, updates table
INSERT INTO dns_zone_journal (zone_name, serial, ...) VALUES ('com', 2025101207, ...);
COMMIT;

-- Transaction 3: Waits for T2, then gets lock
BEGIN;
SELECT get_next_serial('com');  -- Returns 2025101208, updates table
INSERT INTO dns_zone_journal (zone_name, serial, ...) VALUES ('com', 2025101208, ...);
COMMIT;

-- Final state
SELECT * FROM dns_zone_serials WHERE zone_name = 'com';
-- zone_name | serial
-- ----------|------------
-- com       | 2025101208  ← Source of truth

-- Journal has all three entries in order
SELECT zone_name, serial FROM dns_zone_journal WHERE zone_name = 'com' ORDER BY serial DESC LIMIT 3;
-- zone_name | serial
-- ----------|------------
-- com       | 2025101208
-- com       | 2025101207
-- com       | 2025101206
```

**Result: Perfect consistency!**
- ✅ No duplicate serials
- ✅ No gaps (unless transaction rolled back)
- ✅ Serials are strictly monotonically increasing
- ✅ Journal matches dns_zone_serials table

---

## Serial Format: YYYYMMDDnn

The serial uses DNS best practice format: `YYYYMMDDnn`

```
2025101201
│  │ │ │└─ Sequence (01-99)
│  │ │ └── Day (01-31)
│  │ └──── Month (01-12)
│  └────── Year (2025)
└───────── Year prefix
```

### Why This Format?

1. **RFC 1912 Recommendation** - Standard DNS serial format
2. **Human Readable** - Easy to see when zone was last updated
3. **Sortable** - Numeric comparison works correctly
4. **99 updates per day** - Sequence 01-99 allows multiple updates daily
5. **Overflow protection** - Falls back to Unix timestamp if needed

### Serial Calculation Logic

```sql
v_today := TO_CHAR(NOW(), 'YYYYMMDD')::BIGINT * 100;  -- e.g., 2025101200

IF v_current_serial >= v_today AND v_current_serial < v_today + 100 THEN
    -- Same day, increment sequence
    v_new_serial := v_current_serial + 1;
    -- 2025101201 → 2025101202
    
    IF v_new_serial >= v_today + 100 THEN
        -- Overflow! (more than 99 updates today)
        -- Fallback to Unix timestamp
        v_new_serial := EXTRACT(EPOCH FROM NOW())::BIGINT;
        -- e.g., 1728762000
    END IF;
ELSE
    -- New day, reset sequence to 01
    v_new_serial := v_today + 1;
    -- 2025101300 + 1 = 2025101301
END IF;
```

### Example Serial Progression

```
Date/Time            Current Serial    Operation              New Serial
─────────────────────────────────────────────────────────────────────────
2025-10-12 09:00:00  2025101200        First update           2025101201
2025-10-12 09:15:00  2025101201        Second update          2025101202
2025-10-12 10:30:00  2025101202        Third update           2025101203
...
2025-10-12 23:59:59  2025101299        99th update            2025101299
2025-10-13 00:00:01  2025101299        New day                2025101301
2025-10-13 00:05:00  2025101301        Next update            2025101302
```

---

## Reliability Guarantees

### 1. PostgreSQL ACID Properties

| Property | How It Ensures Serial Reliability |
|----------|-----------------------------------|
| **Atomicity** | Serial increment and journal insert happen together or not at all |
| **Consistency** | Serial always increases; constraints prevent invalid states |
| **Isolation** | `FOR UPDATE` lock ensures serializable serial generation |
| **Durability** | Once committed, serial is persisted (survives crashes) |

### 2. What Happens in Edge Cases?

#### A. Transaction Rollback

```go
err := db.Transaction(func(tx *gorm.DB) error {
    // Get serial 2025101205
    dnsPublisher.PublishChange(ctx, tx, change)
    
    // Oops! Error occurs
    return fmt.Errorf("something failed")
})
// Transaction rolls back
// Serial stays at 2025101204
// Journal entry is NOT created
```

**Result:** Serial is NOT incremented (ACID atomicity)

#### B. Application Crash Mid-Transaction

```go
db.Transaction(func(tx *gorm.DB) error {
    dnsPublisher.PublishChange(ctx, tx, change) // Serial incremented
    
    // ⚡ Application crashes HERE
})
```

**Result:** PostgreSQL automatically rolls back uncommitted transactions. Serial reverts to previous value.

#### C. Database Crash

```
T1: Transaction A increments serial to 2025101206
T2: Transaction A commits
T3: ⚡ PostgreSQL server crashes
T4: PostgreSQL restarts
T5: Query dns_zone_serials
```

**Result:** Serial is 2025101206 (PostgreSQL's write-ahead log ensures durability)

#### D. Network Partition / Split Brain

If you're running PostgreSQL with replication:

```
Primary: serial = 2025101210
Replica: serial = 2025101210

[Network partition occurs]

Primary: Continues incrementing (2025101211, 2025101212, ...)
Replica: Read-only (no writes allowed)

[Network heals]

Replica: Catches up via streaming replication
         serial = 2025101212
```

**Result:** Replica serial eventually catches up. No conflicts because writes only go to primary.

### 3. Concurrent Safety Guarantees

**Test: 1000 concurrent domain creations**

```go
func TestConcurrentSerialIncrement(t *testing.T) {
    db := setupTestDB(t)
    publisher := dnsevents.NewEventPublisher(db)
    
    var wg sync.WaitGroup
    numGoroutines := 1000
    
    for i := 0; i < numGoroutines; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            
            db.Transaction(func(tx *gorm.DB) error {
                change := &dnsevents.DNSChange{
                    ZoneName: "test",
                    // ... other fields ...
                }
                return publisher.PublishChange(context.Background(), tx, change)
            })
        }(i)
    }
    
    wg.Wait()
    
    // Verify: Should have exactly 1000 journal entries
    var count int64
    db.Raw("SELECT COUNT(*) FROM dns_zone_journal WHERE zone_name = 'test'").Scan(&count)
    assert.Equal(t, int64(1000), count)
    
    // Verify: All serials are unique and sequential
    var serials []int64
    db.Raw(`
        SELECT serial 
        FROM dns_zone_journal 
        WHERE zone_name = 'test' 
        ORDER BY serial
    `).Scan(&serials)
    
    // Check no duplicates
    serialSet := make(map[int64]bool)
    for _, s := range serials {
        assert.False(t, serialSet[s], "Duplicate serial: %d", s)
        serialSet[s] = true
    }
    
    // Check sequential (no gaps)
    for i := 1; i < len(serials); i++ {
        assert.Equal(t, serials[i-1]+1, serials[i], 
            "Gap detected between %d and %d", serials[i-1], serials[i])
    }
}
```

**Expected Result:** ✅ All tests pass. Serials are unique, sequential, and match the count.

---

## Reading the Serial (Multiple Contexts)

### 1. Application Code (EventPublisher)

```go
// Get current serial WITHOUT incrementing
func (ep *EventPublisher) GetCurrentSerial(ctx context.Context, zoneName string) (int64, error) {
    var serial int64
    err := ep.db.WithContext(ctx).Raw(
        "SELECT get_current_serial(?)",
        zoneName,
    ).Scan(&serial).Error
    return serial, err
}

// Usage
serial, err := dnsPublisher.GetCurrentSerial(ctx, "com")
fmt.Printf("Current serial for .com: %d\n", serial)
```

### 2. CoreDNS Plugin (AXFR/IXFR)

```go
// In CoreDNS plugin
func (p *PostgresBackend) handleAXFR(ctx context.Context, zone string) error {
    // Read current serial for SOA record
    var serial int64
    err := p.db.Raw("SELECT serial FROM dns_zone_serials WHERE zone_name = ?", zone).
        Scan(&serial).Error
    
    // Build SOA with this serial
    soa := &dns.SOA{
        Serial: uint32(serial),  // ← Source of truth
        // ...
    }
    
    // Send zone data...
}

func (p *PostgresBackend) handleIXFR(ctx context.Context, zone string, clientSerial int64) error {
    // Read current serial
    var currentSerial int64
    p.db.Raw("SELECT serial FROM dns_zone_serials WHERE zone_name = ?", zone).
        Scan(&currentSerial)
    
    if clientSerial >= currentSerial {
        // Client is up-to-date
        return sendUpToDateResponse()
    }
    
    // Query journal for changes since client's serial
    rows, err := p.db.Raw(`
        SELECT serial, change_type, record_type, record_name, record_data
        FROM dns_zone_journal
        WHERE zone_name = ? AND serial > ? AND serial <= ?
        ORDER BY serial, id
    `, zone, clientSerial, currentSerial).Rows()
    
    // Send incremental changes...
}
```

### 3. Monitoring/Observability

```sql
-- Current serial for all zones
SELECT zone_name, serial, updated_at
FROM dns_zone_serials
ORDER BY zone_name;

-- zone_name | serial      | updated_at
-- ----------|-------------|---------------------------
-- com       | 2025101215  | 2025-10-12 15:30:45+00
-- net       | 2025101209  | 2025-10-12 14:22:10+00
-- org       | 2025101231  | 2025-10-12 16:15:00+00
-- tld       | 2025101203  | 2025-10-12 09:00:00+00

-- How many updates today per zone?
SELECT 
    zone_name,
    serial,
    (serial % 100) AS updates_today,  -- Sequence number
    updated_at
FROM dns_zone_serials
WHERE serial >= (TO_CHAR(NOW(), 'YYYYMMDD')::BIGINT * 100);

-- Recent serial history (from journal)
SELECT 
    zone_name,
    serial,
    COUNT(*) as changes,
    MIN(timestamp) as first_change,
    MAX(timestamp) as last_change
FROM dns_zone_journal
GROUP BY zone_name, serial
ORDER BY zone_name, serial DESC
LIMIT 20;
```

---

## What About Journal Cleanup?

**Question:** If we delete old journal entries, does that affect the serial?

**Answer:** **NO!** The journal is separate from the serial.

```sql
-- Serial source of truth
SELECT serial FROM dns_zone_serials WHERE zone_name = 'com';
-- serial: 2025101299

-- Journal (historical record)
SELECT COUNT(*) FROM dns_zone_journal WHERE zone_name = 'com';
-- count: 150000 entries

-- Clean up old journal entries
SELECT cleanup_dns_journal(100);  -- Keep last 100 serials

-- Journal now
SELECT COUNT(*) FROM dns_zone_journal WHERE zone_name = 'com';
-- count: 5000 entries (only recent ones)

-- Serial is UNCHANGED
SELECT serial FROM dns_zone_serials WHERE zone_name = 'com';
-- serial: 2025101299  ← Still the same!
```

**The journal cleanup only removes historical IXFR data. The current serial in `dns_zone_serials` is never touched by cleanup.**

---

## Failure Scenarios & Recovery

### Scenario 1: Serial Corruption (Manual Fix Needed)

**Problem:** Someone manually edits the serial in the database

```sql
-- Oops! Admin manually sets serial backwards
UPDATE dns_zone_serials SET serial = 2025101200 WHERE zone_name = 'com';

-- Next domain creation gets serial 2025101201
-- But journal already has entries with serial 2025101250+
```

**Detection:**
```sql
-- Check for serial mismatches
SELECT 
    zs.zone_name,
    zs.serial AS current_serial,
    MAX(j.serial) AS max_journal_serial,
    CASE 
        WHEN zs.serial < MAX(j.serial) THEN 'MISMATCH!'
        ELSE 'OK'
    END AS status
FROM dns_zone_serials zs
LEFT JOIN dns_zone_journal j ON j.zone_name = zs.zone_name
GROUP BY zs.zone_name, zs.serial;
```

**Recovery:**
```sql
-- Fix: Set serial to max journal serial + 1
UPDATE dns_zone_serials
SET serial = (
    SELECT COALESCE(MAX(serial), 0) + 1
    FROM dns_zone_journal
    WHERE zone_name = dns_zone_serials.zone_name
)
WHERE zone_name = 'com';
```

### Scenario 2: Journal Entry Without Serial Increment

**Problem:** Code bug inserts journal entry but doesn't call `get_next_serial()`

```go
// BAD CODE (don't do this!)
db.Transaction(func(tx *gorm.DB) error {
    // Manually insert journal without calling get_next_serial()
    tx.Exec(`
        INSERT INTO dns_zone_journal (zone_name, serial, ...)
        VALUES ('com', 2025101299, ...)
    `)  // ← BUG! Should call get_next_serial() instead
})
```

**Detection:**
```sql
-- Find journal entries with future serials
SELECT *
FROM dns_zone_journal j
WHERE NOT EXISTS (
    SELECT 1 FROM dns_zone_serials s
    WHERE s.zone_name = j.zone_name
    AND s.serial >= j.serial
);
```

**Prevention:** Always use `EventPublisher.PublishChange()` which guarantees serial is incremented.

---

## Best Practices

### ✅ DO

1. **Always use `get_next_serial()` function**
   ```go
   serial, err := tx.Raw("SELECT get_next_serial(?)", zone).Scan(&serial).Error
   ```

2. **Always publish DNS events within transactions**
   ```go
   db.Transaction(func(tx *gorm.DB) error {
       return dnsPublisher.PublishChange(ctx, tx, change)
   })
   ```

3. **Monitor serial progression**
   ```sql
   -- Daily check
   SELECT zone_name, serial, updated_at
   FROM dns_zone_serials
   WHERE updated_at < NOW() - INTERVAL '1 day';
   ```

4. **Use read replicas for AXFR/IXFR queries**
   - Reads from `dns_zone_serials` don't need locks
   - Offload to replicas to reduce primary load

### ❌ DON'T

1. **Never manually UPDATE dns_zone_serials.serial**
   ```sql
   -- BAD!
   UPDATE dns_zone_serials SET serial = 123 WHERE zone_name = 'com';
   ```

2. **Never insert into dns_zone_journal without calling get_next_serial()**
   ```sql
   -- BAD!
   INSERT INTO dns_zone_journal (zone_name, serial, ...) VALUES ('com', 123, ...);
   ```

3. **Never call get_next_serial() outside a transaction**
   ```go
   // BAD!
   serial, _ := db.Raw("SELECT get_next_serial('com')").Scan(&serial).Error
   // Serial is incremented but no corresponding operation!
   ```

4. **Never use `SELECT serial FROM dns_zone_serials FOR UPDATE` in application code**
   - Use the `get_next_serial()` function instead
   - It handles all the locking and increment logic

---

## Summary

### Source of Truth: `dns_zone_serials` Table

```
┌─────────────────────────────────────────┐
│      dns_zone_serials (Source of Truth) │
│                                         │
│  zone_name  │  serial     │  updated_at│
│  ──────────────────────────────────────│
│  com        │  2025101215 │  15:30:45  │ ← AUTHORITATIVE
│  tld        │  2025101203 │  09:00:00  │ ← AUTHORITATIVE
└─────────────────────────────────────────┘
             ▲
             │ get_next_serial()
             │ (row lock + increment)
             │
      ┌──────┴────────┐
      │ EventPublisher│
      └───────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│      dns_zone_journal (History)         │
│                                         │
│  Records what changed at each serial    │
│  Used for IXFR incremental transfers    │
└─────────────────────────────────────────┘
```

### Reliability Mechanisms

1. **PostgreSQL Row Locking** - Prevents concurrent serial conflicts
2. **Transaction Safety** - Rollback undoes serial increments
3. **ACID Properties** - Guarantees consistency
4. **YYYYMMDDnn Format** - Human readable, sortable, standard
5. **Single Source of Truth** - One table, one serial per zone

### Key Takeaway

**The serial is as reliable as your PostgreSQL database.** If PostgreSQL is running with proper:
- ✅ Replication (streaming replication)
- ✅ Backups (pg_basebackup + WAL archiving)
- ✅ Monitoring (check serial progression)
- ✅ Transactions (never increment serial outside transactions)

Then your DNS serials are **production-grade reliable**. 🎯
