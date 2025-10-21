# DNS Hold Status Implementation

## Overview

This document describes the implementation of DNS-aware hold status handling for domains. When a domain is placed on `clientHold` or `serverHold`, all DNS delegation records (NS and glue A/AAAA records) are removed from DNS. When the hold is lifted, all delegation records are restored.

## Implementation Date

October 12, 2025

## RFC Compliance

According to RFC 5731 Section 2.3, both `clientHold` and `serverHold` statuses:
> "The domain object has been placed on hold and is not activated in the DNS."

This implementation ensures that DNS records are properly managed to enforce this semantic.

## Architecture

### Components Modified

1. **`domain_service.go`** - Main business logic
   - `SetStatus()` - Handles setting domain statuses
   - `UnSetStatus()` - Handles unsetting domain statuses
   - `queueDNSChangesForHost()` - Updated to respect hold status
   - `queueRemoveAllDomainDelegation()` - New helper (removes all DNS records)
   - `queueAddAllDomainDelegation()` - New helper (restores all DNS records)

### Implementation Strategy

#### 1. Prevent DNS Changes on Held Domains

The `queueDNSChangesForHost()` method now checks if a domain has hold status before queueing any DNS changes:

```go
// Don't queue DNS changes if domain is on hold
if dom.Status.HasHold() {
    s.logger.Debug("Skipping DNS changes for domain on hold",
        zap.String("domain", dom.Name.String()),
        zap.String("host", host.Name.String()))
    return
}
```

**Impact**: When adding/removing hosts to/from a domain that is on hold, no DNS changes are queued.

#### 2. Remove DNS Records When Hold is Set

When `SetStatus()` is called with `clientHold` or `serverHold`:

```go
// Check if setting a hold status - we need to remove DNS records BEFORE setting the status
isSettingHold := (status == entities.DomainStatusClientHold || status == entities.DomainStatusServerHold)
wasOnHold := previousDom.Status.HasHold()

if isSettingHold && !wasOnHold {
    // Domain is being placed on hold - remove all DNS delegation
    s.queueRemoveAllDomainDelegation(ctx, dom)
}
```

**Flow**:
1. Check if the new status is a hold status
2. Check if domain was already on hold
3. If transitioning from non-hold to hold, queue DELETE for all delegation records
4. Set the status
5. Save to database

#### 3. Restore DNS Records When Hold is Removed

When `UnSetStatus()` is called to remove `clientHold` or `serverHold`:

```go
// Check if unsetting a hold status
isUnsettingHold := (status == entities.DomainStatusClientHold || status == entities.DomainStatusServerHold)
wasOnHold := previousDom.Status.HasHold()

// Unset the status
err = dom.UnSetStatus(status)
if err != nil {
    return nil, errors.Join(ErrCannotSetDomainStatus, err)
}

// Check if domain is no longer on hold after this change
isStillOnHold := dom.Status.HasHold()

if isUnsettingHold && wasOnHold && !isStillOnHold {
    // Domain was on hold and now it's not - restore all DNS delegation
    s.queueAddAllDomainDelegation(ctx, dom)
}
```

**Flow**:
1. Capture previous hold status
2. Unset the requested status
3. Check if domain is still on hold (might have both client and server hold)
4. If domain was on hold and is no longer on hold, queue ADD for all delegation records
5. Save to database

**Important**: If a domain has both `clientHold` and `serverHold`, DNS records are only restored when BOTH are removed.

## Helper Methods

### `queueRemoveAllDomainDelegation()`

Queues DELETE operations for all NS and glue records for a domain.

**Special Handling**: Temporarily clears hold status to bypass the hold check in `queueDNSChangesForHost()`, then restores it:

```go
// Temporarily disable hold check to allow queueing DELETE operations
holdStatus := dom.Status.DeepCopy()
dom.Status.ClientHold = false
dom.Status.ServerHold = false

// Queue DELETE for all hosts
for _, host := range dom.Hosts {
    s.queueDNSChangesForHost(ctx, dom, host, dnsevents.DNSChangeTypeDelete)
}

// Restore original status
dom.Status = holdStatus
```

This is necessary because we need to queue deletions even though the domain is (or is about to be) on hold.

### `queueAddAllDomainDelegation()`

Queues ADD operations for all NS and glue records for a domain.

**Note**: This method is only called when the domain is being released from hold, so no special handling is needed.

## DNS Records Affected

For each host associated with the domain:

1. **NS Record**: `domain.tld. IN NS nameserver.example.com.`
2. **Glue A Record**: `nameserver.example.com. IN A 192.0.2.1` (if IPv4 address)
3. **Glue AAAA Record**: `nameserver.example.com. IN AAAA 2001:db8::1` (if IPv6 address)

## Scenarios and Behaviors

### Scenario 1: Domain Placed on Hold

**Initial State**: `example.tld` has 2 nameservers with glue records

**Action**: `SetStatus(ctx, "example.tld", "clientHold")`

**Result**:
- 6 DNS DELETE operations queued (2 NS + 4 glue records)
- Domain status updated to include `clientHold`
- Next batch publish will remove records from zone journal

### Scenario 2: Host Added to Domain on Hold

**Initial State**: `example.tld` has `clientHold` status

**Action**: `AddHostToDomain(ctx, "example.tld", "ns3.example.com")`

**Result**:
- Host successfully added to domain in database
- **NO** DNS changes queued (silently skipped due to hold status)
- NS and glue records will only appear in DNS when hold is lifted

### Scenario 3: Hold Removed from Domain

**Initial State**: `example.tld` has `clientHold` status and 2 nameservers

**Action**: `UnSetStatus(ctx, "example.tld", "clientHold")`

**Result**:
- 6 DNS ADD operations queued (2 NS + 4 glue records)
- Domain status updated (clientHold removed)
- Next batch publish will add all delegation records to zone journal

### Scenario 4: Domain with Dual Hold

**Initial State**: `example.tld` has both `clientHold` AND `serverHold`

**Action 1**: `UnSetStatus(ctx, "example.tld", "clientHold")`

**Result 1**:
- `clientHold` removed from status
- Domain still has `serverHold` → `HasHold()` returns true
- **NO** DNS changes queued (domain still on hold)

**Action 2**: `UnSetStatus(ctx, "example.tld", "serverHold")`

**Result 2**:
- `serverHold` removed from status
- Domain no longer on hold → `HasHold()` returns false
- All DNS delegation records queued for ADD

### Scenario 5: Multiple Hold Status Changes

**State 1**: Domain active, has 2 NS records in DNS

**Action 1**: Set `clientHold`
- Result: 6 DELETE operations queued

**Action 2**: Add 3rd nameserver while on hold
- Result: Host added to DB, NO DNS changes

**Action 3**: Unset `clientHold`
- Result: 9 ADD operations queued (3 NS records with glue)
- All 3 nameservers now appear in DNS

## Testing

### Manual Test Procedure

#### Test 1: Basic Hold/Unhold

```bash
# Setup: Create domain with hosts
curl -X POST /api/domains \
  -d '{"name": "test1.tld", "hosts": ["ns1.test1.tld", "ns2.test1.tld"]}'

# Verify DNS records queued
psql -c "SELECT * FROM dns_change_queue WHERE domain_name = 'test1.tld.' AND published_at IS NULL;"
# Expected: 4-6 pending records (2 NS + glue)

# Place on hold
curl -X POST /api/domains/test1.tld/status \
  -d '{"status": "clientHold"}'

# Verify DELETE records queued
psql -c "SELECT change_type, record_type, record_name FROM dns_change_queue 
         WHERE domain_name = 'test1.tld.' AND published_at IS NULL ORDER BY id;"
# Expected: DELETE operations for NS and glue records

# Wait for batch publish (1 minute)
sleep 61

# Verify records published
psql -c "SELECT serial, change_type, record_type FROM dns_zone_journal 
         WHERE domain_name = 'test1.tld.' ORDER BY serial DESC LIMIT 10;"
# Expected: Latest entries are DELETE operations with same serial

# Remove hold
curl -X DELETE /api/domains/test1.tld/status/clientHold

# Verify ADD records queued
psql -c "SELECT change_type, record_type, record_name FROM dns_change_queue 
         WHERE domain_name = 'test1.tld.' AND published_at IS NULL ORDER BY id;"
# Expected: ADD operations for NS and glue records

# Wait for batch publish
sleep 61

# Verify restored in journal
psql -c "SELECT serial, change_type, record_type FROM dns_zone_journal 
         WHERE domain_name = 'test1.tld.' ORDER BY serial DESC LIMIT 10;"
# Expected: Latest entries are ADD operations
```

#### Test 2: Modify Domain While on Hold

```bash
# Setup: Domain on hold
curl -X POST /api/domains/test2.tld/status \
  -d '{"status": "serverHold"}'

# Add host while on hold
curl -X POST /api/domains/test2.tld/hosts \
  -d '{"hostname": "ns3.test2.tld", "addresses": ["192.0.2.3"]}'

# Verify NO DNS changes queued for the new host
psql -c "SELECT COUNT(*) FROM dns_change_queue 
         WHERE domain_name = 'test2.tld.' 
         AND record_data LIKE '%ns3%' 
         AND published_at IS NULL;"
# Expected: 0

# Remove hold
curl -X DELETE /api/domains/test2.tld/status/serverHold

# Verify ALL hosts (including ns3) queued for ADD
psql -c "SELECT record_type, record_name, record_data FROM dns_change_queue 
         WHERE domain_name = 'test2.tld.' AND published_at IS NULL;"
# Expected: NS and glue records for ALL hosts including ns3
```

#### Test 3: Dual Hold Status

```bash
# Setup: Set both holds
curl -X POST /api/domains/test3.tld/status \
  -d '{"status": "clientHold"}'
curl -X POST /api/domains/test3.tld/status \
  -d '{"status": "serverHold"}'

# Verify DELETE only queued once (from first hold)
psql -c "SELECT COUNT(*) FROM dns_change_queue 
         WHERE domain_name = 'test3.tld.' 
         AND change_type = 'DELETE' 
         AND published_at IS NULL;"

# Remove clientHold
curl -X DELETE /api/domains/test3.tld/status/clientHold

# Verify NO ADD operations (still has serverHold)
psql -c "SELECT COUNT(*) FROM dns_change_queue 
         WHERE domain_name = 'test3.tld.' 
         AND change_type = 'ADD' 
         AND published_at IS NULL;"
# Expected: 0

# Remove serverHold
curl -X DELETE /api/domains/test3.tld/status/serverHold

# Verify ADD operations queued
psql -c "SELECT COUNT(*) FROM dns_change_queue 
         WHERE domain_name = 'test3.tld.' 
         AND change_type = 'ADD' 
         AND published_at IS NULL;"
# Expected: Number of NS + glue records
```

### SQL Verification Queries

```sql
-- Check pending changes for a specific domain
SELECT 
    id,
    change_type,
    record_type,
    record_name,
    record_data,
    created_at
FROM dns_change_queue
WHERE domain_name = 'example.tld.'
  AND published_at IS NULL
ORDER BY created_at DESC;

-- Check if domain has hold status
SELECT 
    name,
    status->>'ClientHold' as client_hold,
    status->>'ServerHold' as server_hold
FROM domains
WHERE name = 'example.tld';

-- View published journal entries for domain
SELECT 
    serial,
    change_type,
    record_type,
    record_name,
    record_data,
    created_at
FROM dns_zone_journal
WHERE domain_name = 'example.tld.'
ORDER BY serial DESC
LIMIT 20;

-- Count ADD vs DELETE operations per domain
SELECT 
    domain_name,
    change_type,
    COUNT(*) as count
FROM dns_change_queue
WHERE published_at IS NULL
GROUP BY domain_name, change_type
ORDER BY domain_name;
```

## Edge Cases

### 1. Domain with No Hosts

**Behavior**: Both `queueRemoveAllDomainDelegation()` and `queueAddAllDomainDelegation()` check for hosts and return early if none exist.

**Result**: No DNS operations queued (correct, as there are no records to remove/add).

### 2. Rapid Status Changes

**Scenario**: Hold set and unset within the same batch interval (< 1 minute)

**Behavior**: 
- DELETE operations queued when hold is set
- ADD operations queued when hold is removed
- Both sets of operations will be in the same batch
- Net effect depends on DNS journal processing order

**Mitigation**: The serial number ensures proper ordering in the journal.

### 3. DNS Publisher Disabled

**Behavior**: All DNS queueing methods check `if s.dnsBatchPublisher == nil` and return early.

**Result**: Status changes work, but no DNS operations occur (graceful degradation).

### 4. Database Transaction Failure

**Scenario**: Status update succeeds but database save fails

**Behavior**: The entire `SetStatus()`/`UnSetStatus()` transaction rolls back.

**Result**: DNS changes are queued but domain status is not updated. On next attempt, the logic will re-evaluate and queue appropriate changes.

## Logging

### Debug Logs

```
Skipping DNS changes for domain on hold
  domain: example.tld
  host: ns1.example.com
```

### Info Logs

```
Queueing removal of all DNS delegation records for domain on hold
  domain: example.tld
  host_count: 2
```

```
Queueing addition of all DNS delegation records for domain released from hold
  domain: example.tld
  host_count: 3
```

### Error Logs

```
Failed to queue NS record DNS change
  error: <error details>
  domain: example.tld
  host: ns1.example.com
```

## Performance Considerations

### Batch Size Impact

For a domain with `N` hosts:
- Each host has 1 NS record + M glue records (M = number of IP addresses)
- Setting hold: `N * (1 + M)` DELETE operations
- Removing hold: `N * (1 + M)` ADD operations

**Example**: Domain with 10 hosts, each with 2 IP addresses:
- Hold set: 30 DELETE operations queued
- Hold removed: 30 ADD operations queued

**Mitigation**: 
- Default batch size is 10,000 changes
- Worker processes batches every 1 minute
- Large domains (10 hosts) well within limits

### Database Impact

- Each DNS change is a single row insert into `dns_change_queue`
- Batch processing uses single transaction for publishing
- Index on `(zone_name, published_at)` ensures efficient queries

## Future Enhancements

### 1. Bulk Status Operations

For mass hold/unhold operations (e.g., registrar suspension), consider adding:

```go
func (s *DomainService) BulkSetHoldStatus(ctx context.Context, 
    domainNames []string, holdType string) error
```

### 2. Hold Status Audit Trail

Track when and why domains are placed on hold:

```sql
CREATE TABLE domain_hold_history (
    id SERIAL PRIMARY KEY,
    domain_name VARCHAR(255),
    hold_type VARCHAR(50),
    action VARCHAR(10), -- 'SET' or 'UNSET'
    reason TEXT,
    operator VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW()
);
```

### 3. DNS Record Caching

Cache current DNS state to avoid re-queueing identical operations:

```go
type DNSStateCache struct {
    domainRecords map[string][]DNSRecord
    lastSync      time.Time
}
```

## Related Documentation

- [DNS Batch Publisher Integration](./DNS_INTEGRATION_STEP1_COMPLETE.md)
- [DNS Batch Publisher Phase 1](./DNS_BATCH_PUBLISHER_PHASE1_COMPLETE.md)
- [Domain Status Specification](../internal/domain/entities/domainStatus.go)
- RFC 5731 - Extensible Provisioning Protocol (EPP) Domain Name Mapping

## Summary

This implementation ensures that:
1. ✅ Hold statuses (`clientHold`, `serverHold`) immediately remove DNS delegation
2. ✅ Removing hold status restores all DNS delegation
3. ✅ Adding hosts to held domains doesn't publish DNS records
4. ✅ Dual hold status (both client and server) requires both to be removed before restoration
5. ✅ All operations are logged and auditable
6. ✅ Implementation is RFC 5731 compliant

The system now properly enforces the semantic meaning of hold statuses at the DNS level, preventing domains on hold from being active in DNS while maintaining data integrity.
