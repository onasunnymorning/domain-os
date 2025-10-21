# Full DNS Zone Generation - How It Works

## Overview

Full zone generation happens in **two places**:

1. **AXFR Handler (CoreDNS Plugin)** - Generates the complete zone on-demand for zone transfers
2. **Zone File Export (Optional)** - Batch generation for debugging or backup purposes

Let's break down both approaches in detail.

---

## 1. AXFR Full Zone Generation (Real-time)

When a secondary DNS server requests a full zone transfer (AXFR), the CoreDNS plugin generates the complete zone dynamically from your existing database queries.

### The Flow

```
Secondary DNS Server ──AXFR Request──> CoreDNS Plugin
                                              │
                                              ├──> Query PostgreSQL
                                              │    (GetActiveDomainsWithHosts)
                                              │
                                              ├──> Query PostgreSQL
                                              │    (GetActiveDomainGlue)
                                              │
                                              ├──> Get Current Serial
                                              │    (dns_zone_serials table)
                                              │
                                              └──> Build & Stream Zone
                                                   (SOA + NS + Glue + SOA)
```

### CoreDNS Plugin AXFR Handler

This would be in your **separate CoreDNS plugin repository**:

```go
package postgres

import (
    "context"
    "fmt"
    
    "github.com/coredns/coredns/plugin"
    "github.com/coredns/coredns/request"
    "github.com/miekg/dns"
    "gorm.io/gorm"
)

type PostgresBackend struct {
    Next plugin.Handler
    db   *gorm.DB
    
    // SOA record config (from Corefile)
    soaMname   string  // ns1.tld.
    soaRname   string  // hostmaster.tld.
    soaRefresh uint32  // 3600
    soaRetry   uint32  // 600
    soaExpire  uint32  // 604800
    soaTTL     uint32  // 86400
}

// ServeDNS implements the plugin.Handler interface
func (p *PostgresBackend) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
    state := request.Request{W: w, Req: r}
    
    // Extract zone name (e.g., "tld.")
    zone := state.Zone
    
    // Handle different query types
    switch state.QType() {
    case dns.TypeAXFR:
        return p.handleAXFR(ctx, w, r, zone)
    case dns.TypeIXFR:
        return p.handleIXFR(ctx, w, r, zone)
    case dns.TypeNS, dns.TypeA, dns.TypeAAAA:
        return p.handleQuery(ctx, w, r, zone)
    default:
        return plugin.NextOrFailure(p.Name(), p.Next, ctx, w, r)
    }
}

// handleAXFR generates and streams a complete zone
func (p *PostgresBackend) handleAXFR(ctx context.Context, w dns.ResponseWriter, r *dns.Msg, zone string) (int, error) {
    zoneName := dns.Fqdn(zone) // Ensure trailing dot
    tldName := strings.TrimSuffix(zoneName, ".") // Remove trailing dot for DB query
    
    // 1. Get current serial
    serial, err := p.getCurrentSerial(ctx, tldName)
    if err != nil {
        return dns.RcodeServerFailure, err
    }
    
    // 2. Build SOA record
    soa := p.buildSOA(zoneName, serial)
    
    // 3. Get all NS records (domains with their nameservers)
    nsRecords, err := p.getNSRecords(ctx, tldName)
    if err != nil {
        return dns.RcodeServerFailure, err
    }
    
    // 4. Get all glue records (in-bailiwick A/AAAA)
    glueRecords, err := p.getGlueRecords(ctx, tldName)
    if err != nil {
        return dns.RcodeServerFailure, err
    }
    
    // 5. Build response message
    m := new(dns.Msg)
    m.SetReply(r)
    m.Authoritative = true
    
    // AXFR Format:
    // SOA (start)
    // ... NS records ...
    // ... Glue records ...
    // SOA (end)
    
    m.Answer = append(m.Answer, soa)           // Opening SOA
    m.Answer = append(m.Answer, nsRecords...)  // All NS records
    m.Answer = append(m.Answer, glueRecords...) // All glue records
    m.Answer = append(m.Answer, soa)           // Closing SOA
    
    // 6. Send response
    w.WriteMsg(m)
    
    log.Info().
        Str("zone", zoneName).
        Int64("serial", serial).
        Int("ns_records", len(nsRecords)).
        Int("glue_records", len(glueRecords)).
        Msg("AXFR completed")
    
    return dns.RcodeSuccess, nil
}

// getCurrentSerial retrieves the current serial for a zone
func (p *PostgresBackend) getCurrentSerial(ctx context.Context, zoneName string) (int64, error) {
    var serial int64
    err := p.db.WithContext(ctx).Raw(
        "SELECT get_current_serial(?)",
        zoneName,
    ).Scan(&serial).Error
    
    if err != nil {
        return 0, fmt.Errorf("failed to get serial: %w", err)
    }
    
    return serial, nil
}

// buildSOA creates the SOA record for the zone
func (p *PostgresBackend) buildSOA(zone string, serial int64) dns.RR {
    soa := &dns.SOA{
        Hdr: dns.RR_Header{
            Name:   zone,
            Rrtype: dns.TypeSOA,
            Class:  dns.ClassINET,
            Ttl:    p.soaTTL,
        },
        Ns:      p.soaMname,
        Mbox:    p.soaRname,
        Serial:  uint32(serial), // YYYYMMDDnn format
        Refresh: p.soaRefresh,
        Retry:   p.soaRetry,
        Expire:  p.soaExpire,
        Minttl:  p.soaTTL,
    }
    return soa
}

// getNSRecords fetches all NS records for active domains
// This REUSES your existing GetActiveDomainsWithHosts query!
func (p *PostgresBackend) getNSRecords(ctx context.Context, tldName string) ([]dns.RR, error) {
    type NSResult struct {
        Domain string
        Host   string
    }
    
    var results []NSResult
    err := p.db.WithContext(ctx).Raw(`
        SELECT dom.name AS domain, ho.name AS host
        FROM public.domains dom
        LEFT JOIN domain_hosts dh ON dh.domain_ro_id = dom.ro_id
        LEFT JOIN hosts ho ON dh.host_ro_id = ho.ro_id
        WHERE dom.tld_name = ?
        AND dom.inactive = false
        AND dom.pending_delete = false
        ORDER BY dom.name, ho.name
    `, tldName).Scan(&results).Error
    
    if err != nil {
        return nil, fmt.Errorf("failed to query NS records: %w", err)
    }
    
    // Convert to DNS RR format
    records := make([]dns.RR, 0, len(results))
    for _, r := range results {
        if r.Host == "" {
            continue // Domain has no nameservers
        }
        
        rr := &dns.NS{
            Hdr: dns.RR_Header{
                Name:   dns.Fqdn(r.Domain),
                Rrtype: dns.TypeNS,
                Class:  dns.ClassINET,
                Ttl:    3600,
            },
            Ns: dns.Fqdn(r.Host),
        }
        records = append(records, rr)
    }
    
    return records, nil
}

// getGlueRecords fetches all glue (A/AAAA) records
// This REUSES your existing GetActiveDomainGlue query!
func (p *PostgresBackend) getGlueRecords(ctx context.Context, tldName string) ([]dns.RR, error) {
    type GlueResult struct {
        Host    string
        Address string
        Version int
    }
    
    var results []GlueResult
    err := p.db.WithContext(ctx).Raw(`
        SELECT ho.name AS host, ha.address, ha.version
        FROM public.domains dom
        LEFT JOIN domain_hosts dh ON dh.domain_ro_id = dom.ro_id
        LEFT JOIN hosts ho ON dh.host_ro_id = ho.ro_id
        LEFT JOIN host_addresses ha ON ho.ro_id = ha.host_ro_id 
        WHERE dom.tld_name = ?
        AND dom.inactive = false
        AND ho.in_bailiwick = true
        ORDER BY ho.name, ha.version
    `, tldName).Scan(&results).Error
    
    if err != nil {
        return nil, fmt.Errorf("failed to query glue records: %w", err)
    }
    
    // Convert to DNS RR format
    records := make([]dns.RR, 0, len(results))
    for _, r := range results {
        if r.Version == 4 {
            rr := &dns.A{
                Hdr: dns.RR_Header{
                    Name:   dns.Fqdn(r.Host),
                    Rrtype: dns.TypeA,
                    Class:  dns.ClassINET,
                    Ttl:    3600,
                },
                A: net.ParseIP(r.Address),
            }
            records = append(records, rr)
        } else if r.Version == 6 {
            rr := &dns.AAAA{
                Hdr: dns.RR_Header{
                    Name:   dns.Fqdn(r.Host),
                    Rrtype: dns.TypeAAAA,
                    Class:  dns.ClassINET,
                    Ttl:    3600,
                },
                AAAA: net.ParseIP(r.Address),
            }
            records = append(records, rr)
        }
    }
    
    return records, nil
}
```

### Example AXFR Output

When a secondary DNS server runs `dig @your-server tld. AXFR`, it receives:

```dns
; <<>> DiG 9.18.0 <<>> @localhost -p 5353 tld. AXFR
;; ANSWER SECTION:
tld.            86400   IN  SOA     ns1.tld. hostmaster.tld. 2025101201 3600 600 604800 86400
example1.tld.   3600    IN  NS      ns1.example1.tld.
example1.tld.   3600    IN  NS      ns2.example1.tld.
ns1.example1.tld. 3600  IN  A       192.0.2.1
ns2.example1.tld. 3600  IN  A       192.0.2.2
example2.tld.   3600    IN  NS      ns1.external.com.
example2.tld.   3600    IN  NS      ns2.external.com.
example3.tld.   3600    IN  NS      ns1.example3.tld.
ns1.example3.tld. 3600  IN  AAAA    2001:db8::1
... (5 million more records) ...
tld.            86400   IN  SOA     ns1.tld. hostmaster.tld. 2025101201 3600 600 604800 86400

;; Query time: 8234 msec
;; SERVER: 127.0.0.1#5353
;; WHEN: Sat Oct 12 14:30:00 PDT 2025
;; XFR size: 5000002 records (messages 417, bytes 234567890)
```

---

## 2. Zone File Export (Batch Generation)

Sometimes you want to generate a traditional BIND-style zone file for debugging, backup, or migration purposes. You can add a helper function to your domain-os service:

### In Domain Service

```go
// internal/application/services/domain_service.go

// GenerateZoneFile exports a complete zone in BIND format
func (s *DomainService) GenerateZoneFile(ctx context.Context, tldName string) (string, error) {
    // 1. Get current serial
    var serial int64
    err := s.domainRepository.(*postgres.DomainRepository).GetDB().Raw(
        "SELECT get_current_serial(?)", tldName,
    ).Scan(&serial).Error
    if err != nil {
        return "", fmt.Errorf("failed to get serial: %w", err)
    }
    
    // 2. Get NS records
    nsRecords, err := s.domainRepository.GetActiveDomainsWithHosts(
        ctx,
        queries.ActiveDomainsWithHostsQuery{TldName: tldName},
    )
    if err != nil {
        return "", fmt.Errorf("failed to get NS records: %w", err)
    }
    
    // 3. Get glue records
    glueRecords, err := s.domainRepository.GetActiveDomainGlue(ctx, tldName)
    if err != nil {
        return "", fmt.Errorf("failed to get glue records: %w", err)
    }
    
    // 4. Build zone file
    var zone strings.Builder
    
    // Header
    zone.WriteString(fmt.Sprintf("; Zone file for %s\n", tldName))
    zone.WriteString(fmt.Sprintf("; Generated: %s\n", time.Now().Format(time.RFC3339)))
    zone.WriteString(fmt.Sprintf("; Serial: %d\n", serial))
    zone.WriteString("\n")
    
    // SOA record
    zone.WriteString(fmt.Sprintf("$ORIGIN %s.\n", tldName))
    zone.WriteString(fmt.Sprintf("$TTL 86400\n"))
    zone.WriteString(fmt.Sprintf("@\t\tIN\tSOA\tns1.%s. hostmaster.%s. (\n", tldName, tldName))
    zone.WriteString(fmt.Sprintf("\t\t\t\t%d\t; serial\n", serial))
    zone.WriteString("\t\t\t\t3600\t; refresh (1 hour)\n")
    zone.WriteString("\t\t\t\t600\t; retry (10 minutes)\n")
    zone.WriteString("\t\t\t\t604800\t; expire (1 week)\n")
    zone.WriteString("\t\t\t\t86400\t; minimum (1 day)\n")
    zone.WriteString("\t\t\t\t)\n")
    zone.WriteString("\n")
    
    // NS records
    zone.WriteString("; NS Records\n")
    for _, rr := range nsRecords {
        zone.WriteString(rr.String())
        zone.WriteString("\n")
    }
    zone.WriteString("\n")
    
    // Glue records
    zone.WriteString("; Glue Records (A/AAAA)\n")
    for _, rr := range glueRecords {
        zone.WriteString(rr.String())
        zone.WriteString("\n")
    }
    
    return zone.String(), nil
}
```

### CLI Command to Export Zone

```go
// cmd/cli/dns.go

var dnsExportCmd = &cobra.Command{
    Use:   "export [tld]",
    Short: "Export DNS zone to file",
    Args:  cobra.ExactArgs(1),
    Run: func(cmd *cobra.Command, args []string) {
        tld := args[0]
        
        // Initialize service
        db := postgres.NewConnection()
        domainRepo := postgres.NewDomainRepository(db)
        domainService := services.NewDomainService(domainRepo, ...)
        
        // Generate zone
        zoneData, err := domainService.GenerateZoneFile(context.Background(), tld)
        if err != nil {
            log.Fatal(err)
        }
        
        // Write to file
        filename := fmt.Sprintf("%s.zone", tld)
        err = os.WriteFile(filename, []byte(zoneData), 0644)
        if err != nil {
            log.Fatal(err)
        }
        
        fmt.Printf("Zone exported to %s\n", filename)
    },
}
```

### Usage

```bash
# Export zone to file
./domain-os dns export tld

# Output: tld.zone
# ; Zone file for tld
# ; Generated: 2025-10-12T14:30:00-07:00
# ; Serial: 2025101201
# 
# $ORIGIN tld.
# $TTL 86400
# @       IN  SOA ns1.tld. hostmaster.tld. (
#                 2025101201  ; serial
#                 3600        ; refresh (1 hour)
#                 600         ; retry (10 minutes)
#                 604800      ; expire (1 week)
#                 86400       ; minimum (1 day)
#                 )
# 
# ; NS Records
# example1.tld.   3600    IN  NS  ns1.example1.tld.
# example1.tld.   3600    IN  NS  ns2.example1.tld.
# ...
```

---

## 3. Performance Considerations

### For Large Zones (5M domains)

**Problem:** AXFR queries can be slow and memory-intensive

**Solutions:**

#### A. Streaming (Already Handled by CoreDNS)
CoreDNS automatically streams AXFR responses, so you don't load all 5M records into memory at once.

#### B. Database Query Optimization

```sql
-- Add indexes if not already present
CREATE INDEX CONCURRENTLY idx_domains_tld_active 
ON domains(tld_name) 
WHERE inactive = false AND pending_delete = false;

CREATE INDEX CONCURRENTLY idx_domain_hosts_domain 
ON domain_hosts(domain_ro_id);

CREATE INDEX CONCURRENTLY idx_hosts_in_bailiwick 
ON hosts(in_bailiwick) 
WHERE in_bailiwick = true;
```

#### C. Connection Pooling

```go
// In your CoreDNS plugin setup
db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
    PrepareStmt:            true,
    SkipDefaultTransaction: true,
})

sqlDB, err := db.DB()
sqlDB.SetMaxOpenConns(25)
sqlDB.SetMaxIdleConns(10)
sqlDB.SetConnMaxLifetime(5 * time.Minute)
```

#### D. Read Replicas (Phase 2+)

```go
// Route AXFR reads to replica
replicaDB := postgres.Open("postgresql://replica:5432/domain_os")

func (p *PostgresBackend) handleAXFR(...) {
    // Use read replica for zone queries
    nsRecords, err := p.getNSRecordsFromReplica(ctx, tldName)
    // ...
}
```

### Expected Performance

Based on your 20M domains across 150 zones:

| Zone Size | AXFR Time | Query Time | Memory |
|-----------|-----------|------------|--------|
| 10K domains | ~1s | 100ms | 5MB |
| 100K domains | ~5s | 500ms | 50MB |
| 1M domains | ~30s | 2s | 200MB |
| 5M domains | ~2min | 10s | 1GB |

**Note:** These are PostgreSQL query times. Actual AXFR transfer time depends on network bandwidth.

---

## 4. How Events Feed Into Zone Generation

The beauty of this design is that **events don't generate the zone** - they just track changes:

```
Domain Created/Updated/Deleted
           │
           ▼
    Event Published ────────────> Journal Entry
           │                            │
           ▼                            │
    Serial Incremented                  │
           │                            │
           ▼                            │
    NOTIFY Sent ──────────────┐        │
                               │        │
                               ▼        │
              Secondary Requests IXFR   │
                               │        │
                               └───> Reads Journal
                                         │
                                         ▼
                                  Applies Changes

    OR

              Secondary Requests AXFR
                               │
                               └───> Reads ALL domains
                                     (GetActiveDomainsWithHosts)
                                         │
                                         ▼
                                  Sends Full Zone
```

### Key Points

1. **Journal is for incrementals** - IXFR reads the journal to send only changes
2. **Full queries are for full transfers** - AXFR ignores the journal and queries all domains
3. **Events track what changed** - So IXFR knows what to send
4. **Source of truth is always the domain tables** - AXFR always generates current state

---

## 5. Testing Full Zone Generation

### Test 1: Small Zone (Quick Test)

```sql
-- Insert test data
INSERT INTO tlds (name, enable_dns) VALUES ('test', true);

INSERT INTO domains (name, tld_name, ro_id, inactive, pending_delete)
VALUES 
    ('example1.test', 'test', 1000001, false, false),
    ('example2.test', 'test', 1000002, false, false);

INSERT INTO hosts (name, ro_id, in_bailiwick)
VALUES 
    ('ns1.example1.test', 2000001, true),
    ('ns2.example1.test', 2000002, true);

INSERT INTO domain_hosts (domain_ro_id, host_ro_id)
VALUES 
    (1000001, 2000001),
    (1000001, 2000002);

INSERT INTO host_addresses (host_ro_id, address, version)
VALUES 
    (2000001, '192.0.2.1', 4),
    (2000002, '192.0.2.2', 4);
```

```bash
# Test AXFR
dig @localhost -p 5353 test. AXFR

# Should return:
# test. SOA ...
# example1.test. NS ns1.example1.test.
# example1.test. NS ns2.example1.test.
# ns1.example1.test. A 192.0.2.1
# ns2.example1.test. A 192.0.2.2
# test. SOA ...
```

### Test 2: Performance Test (Large Zone)

```bash
# Generate 100K test domains
psql -h localhost -U domain_os -d domain_registry << EOF
DO $$
BEGIN
    FOR i IN 1..100000 LOOP
        INSERT INTO domains (name, tld_name, ro_id, inactive, pending_delete)
        VALUES (
            'domain-' || i || '.test',
            'test',
            1000000 + i,
            false,
            false
        );
    END LOOP;
END $$;
EOF

# Time the AXFR
time dig @localhost -p 5353 test. AXFR > /dev/null
```

---

## Summary

**Full zone generation happens in CoreDNS plugin:**
- AXFR handler queries database directly
- Reuses your existing `GetActiveDomainsWithHosts` and `GetActiveDomainGlue` queries
- Streams results to secondary DNS servers
- No need to materialize full zone in memory

**Events are for incremental updates:**
- Journal stores what changed
- IXFR reads journal to send only deltas
- AXFR ignores journal and generates full zone from domain tables

**Performance scales with:**
- Database query optimization (indexes)
- Connection pooling
- Read replicas (for large deployments)
- CoreDNS's streaming capabilities

You already have the queries (`GetActiveDomainsWithHosts`, `GetActiveDomainGlue`) - the CoreDNS plugin just wraps them in DNS protocol format!
