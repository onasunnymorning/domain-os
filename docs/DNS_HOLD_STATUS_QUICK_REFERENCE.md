# DNS Hold Status - Quick Reference

## Visual Flow Diagrams

### Setting Hold Status

```
┌─────────────────────────────────────────────────────────────────┐
│ SetStatus(ctx, "example.tld", "clientHold")                     │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │ Check if setting hold│
              │ status AND domain    │
              │ not already on hold  │
              └──────────┬───────────┘
                         │
                 ┌───────┴────────┐
                 │ YES            │ NO
                 ▼                ▼
    ┌─────────────────────┐  ┌──────────────┐
    │ Queue DELETE for:   │  │ Skip DNS ops │
    │ • All NS records    │  └──────┬───────┘
    │ • All glue A records│         │
    │ • All glue AAAA rec │         │
    └──────────┬──────────┘         │
               │                    │
               └────────┬───────────┘
                        │
                        ▼
              ┌──────────────────────┐
              │ Set domain.Status    │
              │ .ClientHold = true   │
              └──────────┬───────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │ Save to database     │
              └──────────┬───────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │ DNS batch publisher  │
              │ processes DELETEs    │
              │ (within 1 minute)    │
              └──────────────────────┘
```

### Removing Hold Status

```
┌─────────────────────────────────────────────────────────────────┐
│ UnSetStatus(ctx, "example.tld", "clientHold")                   │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │ Capture previous     │
              │ hold status          │
              │ wasOnHold = HasHold()│
              └──────────┬───────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │ Unset domain.Status  │
              │ .ClientHold = false  │
              └──────────┬───────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │ Check current status │
              │ isStillOnHold?       │
              └──────────┬───────────┘
                         │
         ┌───────────────┴────────────────┐
         │ Still on hold (serverHold)     │ No longer on hold
         ▼                                ▼
    ┌──────────────┐          ┌─────────────────────┐
    │ Skip DNS ops │          │ Queue ADD for:      │
    └──────┬───────┘          │ • All NS records    │
           │                  │ • All glue A records│
           │                  │ • All glue AAAA rec │
           │                  └──────────┬──────────┘
           │                             │
           └──────────┬──────────────────┘
                      │
                      ▼
           ┌──────────────────────┐
           │ Save to database     │
           └──────────┬───────────┘
                      │
                      ▼
           ┌──────────────────────┐
           │ DNS batch publisher  │
           │ processes ADDs       │
           │ (within 1 minute)    │
           └──────────────────────┘
```

### Adding Host to Held Domain

```
┌─────────────────────────────────────────────────────────────────┐
│ AddHostToDomain(ctx, "example.tld", "ns3.example.com")          │
│ (domain has clientHold status)                                  │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │ Add host to domain   │
              │ in database          │
              └──────────┬───────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │ queueDNSChangesForHost│
              │ called               │
              └──────────┬───────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │ Check: HasHold()?    │
              └──────────┬───────────┘
                         │
                         │ YES (hold active)
                         ▼
              ┌──────────────────────┐
              │ Log: Skipping DNS    │
              │ changes for domain   │
              │ on hold              │
              └──────────┬───────────┘
                         │
                         ▼
              ┌──────────────────────┐
              │ Return early         │
              │ NO DNS changes queued│
              └──────────────────────┘
                         │
                         │ Result: Host added to DB,
                         │ but NOT in DNS until
                         │ hold is removed
                         ▼
```

## State Transitions

### Domain Hold Status State Machine

```
                    ┌──────────────┐
                    │   ACTIVE     │
                    │ (no holds)   │
                    └───┬──────┬───┘
                        │      │
          SetStatus     │      │    SetStatus
          (clientHold)  │      │    (serverHold)
                        │      │
                        ▼      ▼
        ┌──────────────────────────────────┐
        │     CLIENT HOLD or SERVER HOLD   │
        │   (DNS delegation REMOVED)       │
        └──────────────┬───────────────────┘
                       │
                       │ SetStatus (other hold type)
                       │
                       ▼
        ┌──────────────────────────────────┐
        │   DUAL HOLD (client + server)    │
        │   (DNS delegation REMOVED)       │
        └──────────────┬───────────────────┘
                       │
                       │ UnSetStatus (one hold)
                       │
                       ▼
        ┌──────────────────────────────────┐
        │   SINGLE HOLD (one remains)      │
        │   (DNS still REMOVED)            │
        └──────────────┬───────────────────┘
                       │
                       │ UnSetStatus (last hold)
                       │
                       ▼
                    ┌──────────────┐
                    │   ACTIVE     │
                    │ (no holds)   │
                    │ DNS RESTORED │
                    └──────────────┘
```

## Decision Matrix

### When DNS Changes are Queued

| Action | Domain State | Result |
|--------|--------------|--------|
| AddHost | Active (no hold) | ✅ DNS changes queued |
| AddHost | On hold | ❌ DNS changes skipped |
| RemoveHost | Active (no hold) | ✅ DNS changes queued |
| RemoveHost | On hold | ❌ DNS changes skipped |
| SetStatus(clientHold) | Not on hold | ✅ Queue DELETE all records |
| SetStatus(clientHold) | Already on hold | ❌ No DNS changes |
| SetStatus(serverHold) | Not on hold | ✅ Queue DELETE all records |
| SetStatus(serverHold) | Already on hold | ❌ No DNS changes |
| UnSetStatus(clientHold) | Only clientHold | ✅ Queue ADD all records |
| UnSetStatus(clientHold) | Client + Server hold | ❌ No DNS changes (serverHold remains) |
| UnSetStatus(serverHold) | Only serverHold | ✅ Queue ADD all records |
| UnSetStatus(serverHold) | Client + Server hold | ❌ No DNS changes (clientHold remains) |

## Code Path Summary

### Path 1: Normal Host Addition (No Hold)

```
AddHostToDomain()
  └─> dom.AddHost()                    [DB: Add host to domain]
  └─> domainRepository.UpdateDomain()  [DB: Save association]
  └─> hostRepository.UpdateHost()      [DB: Set host linked flag]
  └─> queueDNSChangesForHost()         [DNS: Queue NS + glue]
      └─> HasHold() = false            ✅ Proceed
      └─> QueueChange(NS record)       [Queue: NS record]
      └─> QueueChange(A/AAAA records)  [Queue: Glue records]
```

### Path 2: Host Addition to Held Domain

```
AddHostToDomain()
  └─> dom.AddHost()                    [DB: Add host to domain]
  └─> domainRepository.UpdateDomain()  [DB: Save association]
  └─> hostRepository.UpdateHost()      [DB: Set host linked flag]
  └─> queueDNSChangesForHost()         [DNS: Queue NS + glue]
      └─> HasHold() = true             ❌ Skip DNS
      └─> Return early
```

### Path 3: Setting Hold Status

```
SetStatus(domain, "clientHold")
  └─> GetDomainByName()                [DB: Load domain]
  └─> wasOnHold = HasHold()            [Check: Previous state]
  └─> isSettingHold = true
  └─> if !wasOnHold:
      └─> queueRemoveAllDomainDelegation()
          └─> Save hold status         [Backup current status]
          └─> Clear hold flags         [Temporary]
          └─> For each host:
              └─> queueDNSChangesForHost(DELETE)
                  └─> HasHold() = false (cleared)
                  └─> Queue DELETE operations
          └─> Restore hold status      [Restore backup]
  └─> dom.SetStatus("clientHold")      [Entity: Set flag]
  └─> UpdateDomain()                   [DB: Save]
```

### Path 4: Removing Hold Status (Last Hold)

```
UnSetStatus(domain, "clientHold")
  └─> GetDomainByName()                [DB: Load domain]
  └─> wasOnHold = HasHold()            [Check: Was held]
  └─> dom.UnSetStatus("clientHold")    [Entity: Clear flag]
  └─> isStillOnHold = HasHold()        [Check: Still held?]
  └─> if wasOnHold && !isStillOnHold:
      └─> queueAddAllDomainDelegation()
          └─> For each host:
              └─> queueDNSChangesForHost(ADD)
                  └─> HasHold() = false
                  └─> Queue ADD operations
  └─> UpdateDomain()                   [DB: Save]
```

### Path 5: Removing Hold Status (Dual Hold)

```
UnSetStatus(domain, "clientHold")
  └─> GetDomainByName()                [DB: Load domain]
  └─> wasOnHold = HasHold()            [true: has client+server]
  └─> dom.UnSetStatus("clientHold")    [Entity: Clear clientHold]
  └─> isStillOnHold = HasHold()        [true: serverHold remains]
  └─> if wasOnHold && !isStillOnHold:  ❌ Condition false
      └─> SKIP queueAddAllDomainDelegation()
  └─> UpdateDomain()                   [DB: Save]
```

## Quick Debugging

### Check Domain Hold Status

```sql
SELECT 
    name,
    status->>'ClientHold' as client_hold,
    status->>'ServerHold' as server_hold,
    (status->>'ClientHold')::boolean OR (status->>'ServerHold')::boolean as has_hold
FROM domains
WHERE name = 'example.tld';
```

### Check Pending DNS Operations

```sql
SELECT 
    id,
    change_type,
    record_type,
    record_name,
    created_at
FROM dns_change_queue
WHERE domain_name = 'example.tld.'
  AND published_at IS NULL
ORDER BY id;
```

### Check Published DNS Operations

```sql
SELECT 
    serial,
    change_type,
    record_type,
    record_name,
    created_at
FROM dns_zone_journal
WHERE domain_name = 'example.tld.'
ORDER BY serial DESC
LIMIT 20;
```

## Common Issues & Solutions

| Issue | Symptom | Solution |
|-------|---------|----------|
| DNS not removed on hold | Records still in journal after setting hold | Check if hold status actually set in DB. Check batch publisher is running. Wait 1 minute for batch. |
| DNS not restored on unhold | Records missing after removing hold | Check if BOTH holds removed (if dual hold). Check batch publisher running. Query queue for pending ADDs. |
| Host addition queues DNS on held domain | DNS changes queued despite hold | Check `HasHold()` returns true. Check logger for "Skipping DNS changes" message. |
| Dual hold confusion | DNS appears/disappears unexpectedly | Check BOTH hold flags. Remember: ALL holds must be removed for restoration. |
| Batch not publishing | Queue fills up, nothing published | Check worker is running. Check for errors in logs. Check database connectivity. |

## Performance Expectations

| Operation | Domain with 2 Hosts | Domain with 10 Hosts |
|-----------|---------------------|----------------------|
| Set Hold | 6 DELETE ops queued | 30 DELETE ops queued |
| Remove Hold | 6 ADD ops queued | 30 ADD ops queued |
| Add Host (held) | 0 DNS ops | 0 DNS ops |
| Batch Process Time | < 1 second | < 1 second |
| Queue to DNS Time | ~60 seconds | ~60 seconds |

**Note**: Assumes 2 IP addresses per host (1 IPv4 + 1 IPv6). Each host generates 3 records: 1 NS + 2 glue.
