# DNS Events via Messaging System - Architecture Analysis

## Executive Summary

**Proposal:** Use a messaging system (Kafka, NATS, RabbitMQ, Redis Streams) instead of direct database writes for DNS event publishing.

**Verdict:** 🟢 **RECOMMENDED for Scale** - Best for high-volume scenarios, but adds operational complexity. Can be combined with time-based batching for optimal results.

---

## Architecture Comparison

### Current: Direct Database Publishing

```
┌─────────────────┐
│ Domain Created  │
└────────┬────────┘
         │
         ▼
┌─────────────────────────┐
│ db.Transaction(...)     │
│  ├─ Create domain       │
│  └─ PublishChange(tx)   │
│      └─ INSERT INTO     │
│         dns_zone_journal│
└─────────────────────────┘
         │
         ▼
┌─────────────────────────┐
│ PostgreSQL              │
│  ├─ domains table       │
│  ├─ dns_zone_journal    │
│  └─ dns_zone_serials    │
└─────────────────────────┘
```

**Characteristics:**
- ✅ Simple (one database)
- ✅ ACID guarantees
- ⚠️ Tight coupling
- ❌ Limited throughput
- ❌ No replay capability

### Proposed: Messaging System

```
┌─────────────────┐
│ Domain Created  │
└────────┬────────┘
         │
         ▼
┌─────────────────────────┐
│ db.Transaction(...)     │
│  ├─ Create domain       │
│  └─ Publish to Kafka    │
└─────────────────────────┘
         │
         ▼
┌─────────────────────────┐
│ Kafka/NATS/Redis        │
│ Topic: dns.changes      │
│  ├─ Partition by zone   │
│  └─ Retention: 7 days   │
└─────────┬───────────────┘
          │
          ▼
┌─────────────────────────┐
│ DNS Event Consumer      │
│  ├─ Batches messages    │
│  ├─ get_next_serial()   │
│  └─ INSERT journal      │
└─────────────────────────┘
         │
         ▼
┌─────────────────────────┐
│ PostgreSQL              │
│  ├─ domains table       │
│  ├─ dns_zone_journal    │
│  └─ dns_zone_serials    │
└─────────────────────────┘
```

**Characteristics:**
- ✅ Decoupled services
- ✅ High throughput
- ✅ Replay capability
- ✅ Multiple consumers
- ⚠️ Eventual consistency
- ❌ More complex
- ❌ More infrastructure

---

## Detailed Design

### Option 1: Apache Kafka (Best for High Scale)

```go
// internal/infrastructure/dnsevents/kafka_publisher.go

package dnsevents

import (
    "context"
    "encoding/json"
    
    "github.com/segmentio/kafka-go"
    "github.com/rs/zerolog/log"
)

// KafkaPublisher publishes DNS changes to Kafka
type KafkaPublisher struct {
    writer *kafka.Writer
    topic  string
}

// NewKafkaPublisher creates a new Kafka-based DNS publisher
func NewKafkaPublisher(brokers []string, topic string) *KafkaPublisher {
    return &KafkaPublisher{
        writer: &kafka.Writer{
            Addr:         kafka.TCP(brokers...),
            Topic:        topic,
            Balancer:     &kafka.Hash{}, // Partition by zone_name
            RequiredAcks: kafka.RequireOne,
            Async:        false, // Synchronous for reliability
        },
        topic: topic,
    }
}

// PublishChange publishes a DNS change event to Kafka
func (kp *KafkaPublisher) PublishChange(ctx context.Context, change *DNSChange) error {
    // Serialize to JSON
    data, err := json.Marshal(change)
    if err != nil {
        return err
    }
    
    // Publish to Kafka
    err = kp.writer.WriteMessages(ctx, kafka.Message{
        Key:   []byte(change.ZoneName),  // Partition by zone for ordering
        Value: data,
        Headers: []kafka.Header{
            {Key: "change_type", Value: []byte(change.ChangeType)},
            {Key: "record_type", Value: []byte(change.RecordType)},
        },
    })
    
    if err != nil {
        log.Error().
            Err(err).
            Str("zone", change.ZoneName).
            Msg("Failed to publish DNS change to Kafka")
        return err
    }
    
    log.Debug().
        Str("zone", change.ZoneName).
        Str("record", change.RecordName).
        Msg("DNS change published to Kafka")
    
    return nil
}

// Close shuts down the Kafka writer
func (kp *KafkaPublisher) Close() error {
    return kp.writer.Close()
}
```

### Kafka Consumer with Time-Based Batching

```go
// cmd/workers/dns_consumer.go

package main

import (
    "context"
    "encoding/json"
    "time"
    
    "github.com/segmentio/kafka-go"
    "github.com/onasunnymorning/domain-os/internal/infrastructure/dnsevents"
    "gorm.io/gorm"
)

type DNSConsumer struct {
    reader    *kafka.Reader
    db        *gorm.DB
    publisher *dnsevents.EventPublisher
    
    // Batching config
    batchInterval time.Duration
    maxBatchSize  int
    
    // Batch state
    batches map[string][]*dnsevents.DNSChange  // zone -> changes
}

func NewDNSConsumer(brokers []string, topic string, db *gorm.DB) *DNSConsumer {
    return &DNSConsumer{
        reader: kafka.NewReader(kafka.ReaderConfig{
            Brokers:  brokers,
            Topic:    topic,
            GroupID:  "dns-event-consumer",
            MinBytes: 10e3, // 10KB
            MaxBytes: 10e6, // 10MB
        }),
        db:            db,
        publisher:     dnsevents.NewEventPublisher(db),
        batchInterval: 1 * time.Minute,
        maxBatchSize:  10000,
        batches:       make(map[string][]*dnsevents.DNSChange),
    }
}

func (dc *DNSConsumer) Start(ctx context.Context) {
    // Start batch flusher
    go dc.batchFlusher(ctx)
    
    // Start consuming messages
    for {
        select {
        case <-ctx.Done():
            return
        default:
            dc.consumeMessage(ctx)
        }
    }
}

func (dc *DNSConsumer) consumeMessage(ctx context.Context) {
    msg, err := dc.reader.ReadMessage(ctx)
    if err != nil {
        log.Error().Err(err).Msg("Failed to read Kafka message")
        return
    }
    
    // Deserialize
    var change dnsevents.DNSChange
    if err := json.Unmarshal(msg.Value, &change); err != nil {
        log.Error().Err(err).Msg("Failed to unmarshal DNS change")
        return
    }
    
    // Add to batch
    dc.batches[change.ZoneName] = append(dc.batches[change.ZoneName], &change)
    
    // Check if batch is full (flush early)
    if len(dc.batches[change.ZoneName]) >= dc.maxBatchSize {
        dc.flushZone(change.ZoneName)
    }
}

func (dc *DNSConsumer) batchFlusher(ctx context.Context) {
    ticker := time.NewTicker(dc.batchInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            dc.flushAll()
        case <-ctx.Done():
            dc.flushAll()  // Final flush
            return
        }
    }
}

func (dc *DNSConsumer) flushAll() {
    for zoneName := range dc.batches {
        dc.flushZone(zoneName)
    }
}

func (dc *DNSConsumer) flushZone(zoneName string) {
    changes := dc.batches[zoneName]
    if len(changes) == 0 {
        return
    }
    
    // Clear batch
    delete(dc.batches, zoneName)
    
    // Publish to database with single serial
    err := dc.db.Transaction(func(tx *gorm.DB) error {
        var serial int64
        err := tx.Raw("SELECT get_next_serial(?)", zoneName).Scan(&serial).Error
        if err != nil {
            return err
        }
        
        for _, change := range changes {
            err = tx.Exec(`
                INSERT INTO dns_zone_journal (
                    zone_name, serial, change_type, record_type,
                    record_name, record_data, ttl,
                    source_operation, domain_name
                ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
                zoneName, serial,
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
        
        return nil
    })
    
    if err != nil {
        log.Error().
            Err(err).
            Str("zone", zoneName).
            Int("changes", len(changes)).
            Msg("Failed to flush DNS batch")
    } else {
        log.Info().
            Str("zone", zoneName).
            Int("changes", len(changes)).
            Msg("DNS batch flushed to database")
    }
}
```

### Integration with Domain Service

```go
// internal/application/services/domain_service.go

type DomainService struct {
    domainRepository repositories.DomainRepository
    dnsPublisher     DNSPublisher  // Interface for flexibility
    // ...
}

// DNSPublisher interface (supports multiple implementations)
type DNSPublisher interface {
    PublishChange(ctx context.Context, change *dnsevents.DNSChange) error
}

func (s *DomainService) Create(ctx context.Context, cmd *commands.CreateDomainCommand) error {
    var domain *entities.Domain
    
    // Create domain in transaction
    err := s.db.Transaction(func(tx *gorm.DB) error {
        var err error
        domain, err = s.domainRepository.Create(ctx, domainEntity)
        return err
    })
    
    if err != nil {
        return err
    }
    
    // Publish DNS event AFTER transaction commits
    // (Kafka publish is separate from DB transaction)
    if s.isDNSEnabled(domain.TLDName) {
        changes := s.buildDNSChanges(domain)
        for _, change := range changes {
            if err := s.dnsPublisher.PublishChange(ctx, change); err != nil {
                log.Error().
                    Err(err).
                    Str("domain", domain.Name.String()).
                    Msg("Failed to publish DNS change")
                // Continue - don't fail domain creation if DNS publish fails
            }
        }
    }
    
    return nil
}
```

---

## Advantages of Messaging System

### 1. Decoupling (Major Benefit)

**Current (Tight Coupling):**
```
Domain Service ──[Same Transaction]──> DNS Journal
                                       │
                                       ▼
                          If DNS insert fails,
                          domain creation fails
```

**With Messaging:**
```
Domain Service ──[Transaction]──> Domain DB
      │                                ✅ Committed
      │
      └──[Async]──> Kafka ──> Consumer ──> DNS Journal
                      │                        │
                      ✅                       ❌ Failure doesn't affect domain
```

**Benefits:**
- Domain creation succeeds even if DNS publishing fails
- Can retry DNS publishing without re-creating domain
- Services can evolve independently

### 2. Scalability (Huge Win)

**Current:**
```
Domain Service
    ↓ (writes)
PostgreSQL (single bottleneck)
    ├─ domains table
    └─ dns_zone_journal
```

**With Messaging:**
```
Domain Service 1 ──┐
Domain Service 2 ──┼──> Kafka (partitioned, distributed)
Domain Service 3 ──┘      ├─ Partition 0 (com, net)
                          ├─ Partition 1 (org, io)
                          └─ Partition 2 (dev, test)
                                    │
                    ┌───────────────┼───────────────┐
                    ▼               ▼               ▼
              Consumer 1      Consumer 2      Consumer 3
                    │               │               │
                    └───────────────┼───────────────┘
                                    ▼
                              PostgreSQL
```

**Throughput comparison:**
- Direct DB: ~2,000-5,000 TPS (limited by serial lock)
- Kafka: ~100,000+ messages/sec (then batched to DB)

### 3. Replay & Recovery

**Scenario: DNS journal corrupted or lost**

**Current:**
```
❌ No way to recover
❌ Must rescan entire domain table
❌ May miss historical changes
```

**With Kafka (7-day retention):**
```
✅ Replay messages from Kafka
✅ Rebuild DNS journal from events
✅ Point-in-time recovery
```

**Example:**
```bash
# Rebuild DNS journal from scratch
kafka-consumer-groups --bootstrap-server localhost:9092 \
  --group dns-event-consumer \
  --topic dns.changes \
  --reset-offsets --to-earliest --execute

# Consumer will replay all messages from beginning
# Rebuilds dns_zone_journal from events
```

### 4. Multiple Consumers (Fan-out)

```
                    ┌──> DNS Journal Writer
                    │
Kafka Topic ────────┼──> Metrics Collector
                    │
                    ├──> Audit Log Service
                    │
                    └──> Real-time Monitoring
```

Each consumer can process the same events for different purposes:
- **Journal Writer**: Updates `dns_zone_journal`
- **Metrics**: Tracks changes per zone, velocity, etc.
- **Audit**: Compliance logging
- **Monitoring**: Real-time alerts

### 5. Built-in Buffering

```
High traffic spike:
1M domain creations in 1 hour

Current:
❌ PostgreSQL overwhelmed
❌ Serial lock contention
❌ Timeouts, errors

With Kafka:
✅ Messages buffered in Kafka
✅ Consumer processes at sustainable rate
✅ Automatic backpressure
```

### 6. Observability

**Kafka provides rich metrics:**
```
kafka.topic.dns.changes.messages_per_sec: 1523
kafka.consumer.dns-event-consumer.lag: 12450  ← Consumer falling behind!
kafka.consumer.dns-event-consumer.offset: 1234567890
kafka.partition.0.size_bytes: 2.3GB
```

**Easy monitoring:**
```go
// Check consumer lag
lag := kafkaOffset - consumerOffset
if lag > 100000 {
    alert("DNS consumer falling behind!")
}
```

---

## Disadvantages & Risks

### 1. Eventual Consistency (Major Concern)

**Current (Immediate):**
```
00:00:00.001  Domain created in DB
00:00:00.002  DNS journal updated (same transaction)
00:00:00.003  Serial incremented
              ✅ Domain and DNS are consistent immediately
```

**With Kafka:**
```
00:00:00.001  Domain created in DB ✅
00:00:00.002  Event published to Kafka ✅
00:00:00.003  ... [event sits in Kafka] ...
00:00:01.000  Consumer reads event
00:01:00.000  Batch flushed to DNS journal ✅
              
              ⚠️ Up to 1 minute delay between domain creation and DNS update
```

**Impact:**
```go
// User creates domain
domain := CreateDomain(ctx, "example.com")

// Immediately query DNS
dig example.com NS

// ❌ SERVFAIL - DNS not updated yet!
// Must wait up to 1 minute
```

**Mitigation:**
```go
// Option 1: Synchronous mode for critical operations
if cmd.RequireImmediateDNS {
    dnsPublisher.PublishChangeSync(ctx, change)  // Write to DB directly
} else {
    dnsPublisher.PublishChange(ctx, change)  // Kafka async
}

// Option 2: Read-your-writes pattern
func (svc *DomainService) GetDomain(ctx context.Context, name string) (*Domain, error) {
    domain := svc.domainRepo.Get(ctx, name)
    
    // Check if DNS event is pending (not yet in journal)
    if svc.hasPendingDNSChanges(domain) {
        // Trigger immediate flush or return "DNS pending" status
    }
    
    return domain
}
```

### 2. Message Loss (Low but Possible)

**Scenarios:**

#### A. Producer failure
```
Domain created ✅
Publish to Kafka ❌ (network error, Kafka down)
Domain exists but NO DNS event published!
```

**Mitigation:**
```go
// Retry with exponential backoff
func (kp *KafkaPublisher) PublishChange(ctx context.Context, change *DNSChange) error {
    var err error
    for i := 0; i < 3; i++ {
        err = kp.writer.WriteMessages(ctx, msg)
        if err == nil {
            return nil
        }
        time.Sleep(time.Duration(i*100) * time.Millisecond)
    }
    
    // Still failed - write to dead letter queue
    kp.deadLetterQueue.Add(change)
    return err
}

// Background reconciliation job
func ReconcileDNS() {
    // Find domains without DNS journal entries
    // Re-publish events
}
```

#### B. Consumer crash before commit
```
Consumer reads message ✅
Flushes batch to DB ✅
Consumer crashes before committing Kafka offset ❌
On restart: Re-processes same message ❌
```

**Result:** Duplicate DNS journal entries!

**Mitigation:**
```go
// Idempotent inserts
INSERT INTO dns_zone_journal (zone_name, serial, record_name, ...)
VALUES (?, ?, ?, ...)
ON CONFLICT (zone_name, serial, record_name, record_data) DO NOTHING;

// Or use Kafka transactions (exactly-once semantics)
```

### 3. Operational Complexity

**Additional infrastructure needed:**
```
┌─────────────────┐
│ Kafka Cluster   │
│  ├─ Broker 1    │
│  ├─ Broker 2    │
│  └─ Broker 3    │
└─────────────────┘
        ↓
┌─────────────────┐
│ Zookeeper       │
│  (or KRaft)     │
└─────────────────┘
        ↓
┌─────────────────┐
│ Monitoring      │
│  ├─ Kafka Exporter
│  └─ Grafana     │
└─────────────────┘
```

**Operational burden:**
- Deploy and manage Kafka cluster
- Monitor Kafka health
- Manage consumer groups
- Handle rebalancing
- Disk space management
- Upgrade management

### 4. Debugging Complexity

**Current (Simple):**
```sql
-- Where did this domain's DNS event come from?
SELECT * FROM dns_zone_journal WHERE domain_name = 'example.com';

-- What happened at this time?
SELECT * FROM dns_zone_journal 
WHERE timestamp BETWEEN '2025-10-12 10:00' AND '2025-10-12 10:05';
```

**With Kafka (Complex):**
```bash
# Is the event in Kafka?
kafka-console-consumer --topic dns.changes --from-beginning | grep "example.com"

# Was it consumed?
kafka-consumer-groups --describe --group dns-event-consumer

# Is it in the database?
SELECT * FROM dns_zone_journal WHERE domain_name = 'example.com';

# Where is it stuck?
- In Kafka? (not consumed yet)
- In consumer batch? (waiting for flush)
- Lost? (publish failed, consume failed)
```

### 5. Cost

**Direct DB:** $0 extra (using existing PostgreSQL)

**Kafka:**
- AWS MSK: ~$300-500/month (small cluster)
- Confluent Cloud: ~$500-1000/month
- Self-hosted: 3 VMs + ops time

---

## Messaging System Options

### Apache Kafka

**Best for:** High-volume, long retention, multiple consumers

```yaml
Pros:
  ✅ Highest throughput (100K+ msg/sec)
  ✅ Durable (replication, persistence)
  ✅ Replay capability (days to weeks)
  ✅ Partitioning (parallel processing)
  ✅ Battle-tested at scale

Cons:
  ❌ Most complex to operate
  ❌ Highest resource usage
  ❌ Requires ZooKeeper (or KRaft in newer versions)
  ❌ Overkill for small deployments

Best fit:
  - 1M+ events per day
  - Multiple consumers
  - Need replay/audit trail
```

### NATS JetStream

**Best for:** Simplicity, low latency, cloud-native

```yaml
Pros:
  ✅ Simple to deploy (single binary)
  ✅ Low latency (<1ms)
  ✅ Cloud-native (Kubernetes-friendly)
  ✅ Built-in persistence
  ✅ Lower resource usage than Kafka

Cons:
  ❌ Less mature than Kafka
  ❌ Smaller ecosystem
  ❌ Retention typically shorter

Best fit:
  - 100K-1M events per day
  - Need simplicity
  - Kubernetes environment
```

### Redis Streams

**Best for:** Existing Redis users, moderate volume

```yaml
Pros:
  ✅ Very simple (if already using Redis)
  ✅ Fast (in-memory)
  ✅ Good Lua support for atomicity
  ✅ Low overhead

Cons:
  ❌ Limited persistence (memory-based)
  ❌ No true partitioning
  ❌ Retention limited by memory
  ❌ Single-node bottleneck

Best fit:
  - <100K events per day
  - Already using Redis
  - Don't need long retention
```

### RabbitMQ

**Best for:** Traditional messaging patterns

```yaml
Pros:
  ✅ Mature, stable
  ✅ Rich routing features
  ✅ Good tooling
  ✅ Lower learning curve

Cons:
  ❌ Lower throughput than Kafka
  ❌ Not designed for high fan-out
  ❌ Limited replay capability

Best fit:
  - Traditional queue-based workflows
  - Need complex routing
  - Moderate volume
```

---

## Hybrid Approach: Best of Both Worlds

### Design: Dual Publishing

```go
type HybridDNSPublisher struct {
    immediate *EventPublisher  // Direct to DB
    kafka     *KafkaPublisher  // To Kafka
    config    HybridConfig
}

type HybridConfig struct {
    // Thresholds for choosing publishing method
    LowTrafficThreshold   int           // Changes/minute
    HighTrafficThreshold  int           // Changes/minute
    
    // Modes
    DefaultMode          PublishMode   // "immediate" or "kafka"
    FallbackMode         PublishMode   // If primary fails
}

func (hp *HybridDNSPublisher) PublishChange(ctx context.Context, change *DNSChange) error {
    mode := hp.selectMode(change.ZoneName)
    
    switch mode {
    case PublishImmediate:
        // Low traffic zone - write directly to DB (simple, fast)
        return hp.immediate.PublishChange(ctx, hp.db, change)
        
    case PublishKafka:
        // High traffic zone - publish to Kafka (scalable)
        err := hp.kafka.PublishChange(ctx, change)
        if err != nil {
            // Fallback to immediate on Kafka failure
            log.Warn().Err(err).Msg("Kafka publish failed, falling back to immediate")
            return hp.immediate.PublishChange(ctx, hp.db, change)
        }
        return nil
    }
}

func (hp *HybridDNSPublisher) selectMode(zoneName string) PublishMode {
    // Check current traffic for this zone
    rate := hp.getTrafficRate(zoneName)
    
    if rate < hp.config.LowTrafficThreshold {
        return PublishImmediate
    } else {
        return PublishKafka
    }
}
```

**Benefits:**
- ✅ Simple for low-traffic zones (no Kafka overhead)
- ✅ Scalable for high-traffic zones (Kafka buffering)
- ✅ Automatic fallback on failure
- ✅ Gradual migration path

---

## Recommended Architecture

### Phase 1: Start Simple (Direct DB)

```
Domain Service ──> EventPublisher ──> PostgreSQL
                   (time-based batching, 1-minute)
```

**Why:**
- Simplest to implement and operate
- Good for up to 1M changes/day
- No additional infrastructure
- Easy to debug

### Phase 2: Add Kafka for High-Traffic Zones (Hybrid)

```
Domain Service ──> HybridPublisher ──┬──> PostgreSQL (low traffic zones)
                                     │
                                     └──> Kafka ──> Consumer ──> PostgreSQL (high traffic)
```

**When:**
- Any zone exceeds 10K changes/hour
- Need better decoupling
- Multiple consumers needed
- Replay/audit requirements

### Phase 3: Full Kafka (If Needed)

```
All Domain Services ──> Kafka ──> Multiple Consumers ──> PostgreSQL
                                                      ├──> Metrics
                                                      ├──> Audit
                                                      └──> Monitoring
```

**When:**
- Most zones are high-traffic
- Kafka infrastructure already in place
- Need advanced features (replay, fan-out)

---

## Time-Based Batching + Messaging: Perfect Combination

### Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                    Domain Service                             │
│  Create Domain ──> Publish to Kafka (instant, non-blocking)  │
└────────────────────────────┬─────────────────────────────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │     Kafka       │
                    │ Topic: dns.ch.. │
                    │ Retention: 7d   │
                    └────────┬────────┘
                             │
                             ▼
┌──────────────────────────────────────────────────────────────┐
│                   DNS Event Consumer                          │
│                                                               │
│  ┌─────────────────────────────────────────────────────┐    │
│  │ Consume Loop:                                        │    │
│  │   msg := kafka.ReadMessage()                         │    │
│  │   batch[msg.ZoneName].append(msg)                    │    │
│  │                                                       │    │
│  │   if batchFull OR intervalExpired:                   │    │
│  │       flushBatch() ──> get_next_serial()             │    │
│  │                    ──> INSERT dns_zone_journal       │    │
│  │                    ──> commit kafka offset           │    │
│  └─────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────┘
                             │
                             ▼
                    ┌─────────────────┐
                    │   PostgreSQL    │
                    │ dns_zone_journal│
                    │ dns_zone_serials│
                    └─────────────────┘
```

### Combined Benefits

**Kafka Benefits:**
- ✅ Decoupling (domain service doesn't wait for DNS)
- ✅ Buffering (handles traffic spikes)
- ✅ Replay (recover from errors)

**Time-Based Batching Benefits:**
- ✅ Reduced serial churn (1 serial per minute vs 1000s)
- ✅ Predictable NOTIFY cadence
- ✅ Stays in YYYYMMDDnn format

**Combined Result:**
```
5M domains created in 1 hour:

Without either:
- 5M serial increments
- 5M database transactions
- Constant lock contention
- Format switches to Unix timestamp

With Kafka only:
- 5M messages to Kafka ✅
- 5M serial increments (if consuming immediately)
- Still high lock contention

With Batching only:
- 60 serial increments (1/minute) ✅
- But domain service blocks on DB writes
- No replay capability

With BOTH:
- 5M messages to Kafka ✅ (instant, non-blocking)
- 60 serial increments ✅ (1/minute batching)
- No lock contention ✅
- Replay capability ✅
- Stays in YYYYMMDDnn format ✅
```

---

## Example Configuration

```yaml
# config.yaml

dns:
  publishing:
    mode: hybrid  # "immediate", "kafka", or "hybrid"
    
    # Immediate publishing (direct to DB)
    immediate:
      enabled: true
      batch_interval: 1m
      max_batch_size: 10000
    
    # Kafka publishing
    kafka:
      enabled: true
      brokers:
        - kafka-1:9092
        - kafka-2:9092
        - kafka-3:9092
      topic: dns.changes
      partitions: 12  # One per high-traffic TLD
      replication_factor: 3
      retention_hours: 168  # 7 days
      
    # Consumer
    consumer:
      group_id: dns-event-consumer
      batch_interval: 1m
      max_batch_size: 10000
      workers: 3  # Parallel consumers
      
    # Hybrid mode thresholds
    hybrid:
      low_traffic_threshold: 100   # changes/hour
      high_traffic_threshold: 1000 # changes/hour
```

---

## Monitoring & Observability

### Metrics to Track

```go
// Kafka-specific metrics
kafka_messages_produced_total{topic="dns.changes",zone="com"}
kafka_messages_consumed_total{consumer_group="dns-event-consumer"}
kafka_consumer_lag{consumer_group="dns-event-consumer",partition="0"}
kafka_produce_latency_ms{topic="dns.changes"}

// DNS-specific metrics
dns_events_queued_total{zone="com",source="domain_service"}
dns_events_published_total{zone="com",destination="database"}
dns_batch_size{zone="com",quantile="0.95"}
dns_batch_interval_actual_seconds{zone="com"}
dns_serial_increments_total{zone="com"}

// Alerts
ALERT DNSConsumerLag
  IF kafka_consumer_lag > 100000
  FOR 5m
  LABELS { severity="warning" }
  ANNOTATIONS {
    summary="DNS consumer falling behind",
    description="Consumer lag is {{ $value }} messages"
  }
```

---

## Migration Path

### Week 1: Implement Kafka Publishing (Optional)
```go
// Add Kafka publisher alongside existing
dnsPublisher := NewEventPublisher(db)
kafkaPublisher := NewKafkaPublisher(brokers, "dns.changes")

// Use feature flag
if config.UseKafka {
    service.dnsPublisher = kafkaPublisher
} else {
    service.dnsPublisher = dnsPublisher
}
```

### Week 2: Deploy Consumer
```bash
# Deploy consumer as separate service
./dns-consumer --kafka-brokers=kafka:9092 \
               --topic=dns.changes \
               --batch-interval=1m
```

### Week 3: Enable for Test TLDs
```yaml
kafka:
  enabled_zones:
    - test
    - dev
```

### Week 4: Monitor & Tune
- Check consumer lag
- Adjust batch interval
- Tune partition count

### Week 5: Gradual Rollout
- Enable for .org, .net
- Finally .com
- Keep direct DB as fallback

---

## Final Recommendation

### For Your Use Case (20M Domains, 150 TLDs)

**Start with:** Time-based batching + Direct DB
- Simple to implement (no new infrastructure)
- Handles up to 1M changes/day easily
- Easy to debug and operate

**Add Kafka when:**
- Any zone exceeds 10K changes/hour
- Need decoupling for reliability
- Want replay/audit capability
- Multiple consumers needed

**Architecture:**
```
Immediate (MVP):
  Domain Service ──> BatchPublisher ──> PostgreSQL
  (1-minute batching)

Phase 2 (If needed):
  Domain Service ──> Kafka ──> Consumer (1-min batching) ──> PostgreSQL
                     (buffering, replay)
```

**Verdict:** 
🟢 **Start WITHOUT Kafka** - Add it later if you need the scale/features
🟡 **Consider Kafka** - If you already have it running for other services
🔴 **Don't use Kafka** - If you're under 100K changes/day total

The time-based batching alone solves most of your scaling problems! Kafka adds complexity that may not be worth it until you hit真正 high scale. 🎯
