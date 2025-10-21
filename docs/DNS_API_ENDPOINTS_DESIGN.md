# DNS Management API - Endpoint Design

## Overview

This document outlines the proposed REST API endpoints for DNS zone management, monitoring, and operations.

## API Design Philosophy

- **RESTful**: Follow REST conventions
- **Read-heavy**: Most endpoints are GET (monitoring/querying)
- **Admin-only**: All endpoints require authentication token
- **JSON responses**: Consistent JSON format
- **Pagination**: Large result sets support pagination
- **Standards-compliant**: Follow existing controller patterns

## Proposed Endpoints

### 1. DNS Queue Monitoring

#### GET `/api/v1/dns/queue/stats`
**Purpose**: Get queue statistics across all zones  
**Authentication**: Required  
**Response**: Array of queue statistics per zone

```json
{
  "stats": [
    {
      "zone_name": "tld.",
      "pending_count": 150,
      "published_count": 1250,
      "error_count": 3,
      "oldest_pending": "2025-10-13T01:10:00Z",
      "newest_pending": "2025-10-13T01:16:00Z",
      "avg_wait_seconds": 45.5
    }
  ],
  "total_pending": 150,
  "total_published": 1250,
  "total_errors": 3
}
```

**Use Cases**:
- Dashboard overview of DNS queue health
- Identify zones with high pending counts
- Detect processing delays
- Monitor error rates

---

#### GET `/api/v1/dns/queue/stats/:zone`
**Purpose**: Get queue statistics for a specific zone  
**Authentication**: Required  
**Parameters**: 
- `zone` (path): Zone name (e.g., "tld")

**Response**: Single zone statistics

```json
{
  "zone_name": "tld.",
  "pending_count": 150,
  "published_count": 1250,
  "error_count": 3,
  "oldest_pending": "2025-10-13T01:10:00Z",
  "newest_pending": "2025-10-13T01:16:00Z",
  "avg_wait_seconds": 45.5
}
```

---

#### GET `/api/v1/dns/queue/pending`
**Purpose**: List pending DNS changes  
**Authentication**: Required  
**Query Parameters**:
- `zone` (optional): Filter by zone name
- `domain` (optional): Filter by domain name
- `change_type` (optional): Filter by ADD or DELETE
- `record_type` (optional): Filter by NS, A, AAAA
- `limit` (optional, default: 100): Max results
- `offset` (optional, default: 0): Pagination offset

**Response**: List of pending changes

```json
{
  "changes": [
    {
      "id": 12345,
      "zone_name": "tld.",
      "change_type": "ADD",
      "record_type": "NS",
      "record_name": "example.tld.",
      "record_data": "ns1.example.com.",
      "ttl": 3600,
      "source_operation": "AddHostToDomain",
      "domain_name": "example.tld.",
      "queued_at": "2025-10-13T01:16:00Z"
    }
  ],
  "total": 150,
  "limit": 100,
  "offset": 0
}
```

**Use Cases**:
- Inspect what's waiting to be published
- Debug specific domain DNS issues
- Verify changes are queued correctly

---

#### GET `/api/v1/dns/queue/errors`
**Purpose**: List DNS changes with errors  
**Authentication**: Required  
**Query Parameters**:
- `zone` (optional): Filter by zone name
- `limit` (optional, default: 50): Max results
- `offset` (optional, default: 0): Pagination offset

**Response**: List of errored changes

```json
{
  "errors": [
    {
      "id": 12340,
      "zone_name": "tld.",
      "change_type": "ADD",
      "record_type": "NS",
      "record_name": "example.tld.",
      "record_data": "ns1.example.com.",
      "domain_name": "example.tld.",
      "error_count": 3,
      "last_error": "failed to get next serial: connection timeout",
      "last_error_at": "2025-10-13T01:15:00Z",
      "queued_at": "2025-10-13T01:10:00Z"
    }
  ],
  "total": 3,
  "limit": 50,
  "offset": 0
}
```

**Use Cases**:
- Monitor failed DNS operations
- Debug publishing issues
- Identify problematic records

---

### 2. DNS Zone Journal

#### GET `/api/v1/dns/journal/zones`
**Purpose**: List all zones with journal entries  
**Authentication**: Required

**Response**: List of zones

```json
{
  "zones": [
    {
      "zone_name": "tld.",
      "current_serial": 2025101301,
      "total_changes": 5432,
      "last_change_at": "2025-10-13T01:16:51Z"
    }
  ]
}
```

---

#### GET `/api/v1/dns/journal/:zone`
**Purpose**: Get journal entries for a specific zone  
**Authentication**: Required  
**Parameters**:
- `zone` (path): Zone name

**Query Parameters**:
- `serial` (optional): Filter by specific serial number
- `since_serial` (optional): Get changes since this serial
- `change_type` (optional): Filter by ADD or DELETE
- `limit` (optional, default: 100): Max results
- `offset` (optional, default: 0): Pagination offset

**Response**: Journal entries

```json
{
  "zone_name": "tld.",
  "current_serial": 2025101301,
  "entries": [
    {
      "id": 98765,
      "serial": 2025101301,
      "change_type": "ADD",
      "record_type": "NS",
      "record_name": "example.tld.",
      "record_data": "ns1.example.com.",
      "ttl": 3600,
      "source_operation": "AddHostToDomain",
      "domain_name": "example.tld.",
      "created_at": "2025-10-13T01:16:51Z"
    }
  ],
  "total": 45,
  "limit": 100,
  "offset": 0
}
```

**Use Cases**:
- Audit DNS changes
- Track zone modifications
- Debug DNS propagation issues
- IXFR support (incremental zone transfers)

---

#### GET `/api/v1/dns/journal/:zone/serial/:serial`
**Purpose**: Get all changes for a specific serial number  
**Authentication**: Required  
**Parameters**:
- `zone` (path): Zone name
- `serial` (path): Serial number

**Response**: All changes in that batch

```json
{
  "zone_name": "tld.",
  "serial": 2025101301,
  "batch_id": 1760318211679029014,
  "created_at": "2025-10-13T01:16:51Z",
  "changes": [
    {
      "id": 98765,
      "change_type": "ADD",
      "record_type": "NS",
      "record_name": "example.tld.",
      "record_data": "ns1.example.com.",
      "ttl": 3600
    }
  ],
  "total_changes": 3
}
```

**Use Cases**:
- View complete batch of changes
- Verify atomic operations
- Debug batch publishing

---

#### GET `/api/v1/dns/journal/:zone/domain/:domain`
**Purpose**: Get all journal entries for a specific domain  
**Authentication**: Required  
**Parameters**:
- `zone` (path): Zone name
- `domain` (path): Domain name (e.g., "example.tld")

**Query Parameters**:
- `limit` (optional, default: 100)
- `offset` (optional, default: 0)

**Response**: Domain-specific journal entries

```json
{
  "zone_name": "tld.",
  "domain_name": "example.tld.",
  "entries": [
    {
      "serial": 2025101301,
      "change_type": "ADD",
      "record_type": "NS",
      "record_name": "example.tld.",
      "record_data": "ns1.example.com.",
      "created_at": "2025-10-13T01:16:51Z"
    }
  ],
  "total": 12
}
```

**Use Cases**:
- Audit domain DNS history
- Debug domain-specific issues
- Track delegation changes

---

### 3. DNS Operations

#### POST `/api/v1/dns/queue/cleanup`
**Purpose**: Cleanup old published queue items  
**Authentication**: Required  
**Request Body**:

```json
{
  "retention_days": 7
}
```

**Response**:

```json
{
  "deleted_count": 1234,
  "retention_days": 7,
  "completed_at": "2025-10-13T01:20:00Z"
}
```

**Use Cases**:
- Maintenance operations
- Reduce database size
- Keep queue table performant

---

#### POST `/api/v1/dns/queue/retry`
**Purpose**: Retry failed DNS changes  
**Authentication**: Required  
**Request Body**:

```json
{
  "zone": "tld.",  // optional
  "max_attempts": 3  // only retry items with fewer attempts
}
```

**Response**:

```json
{
  "reset_count": 5,
  "zone": "tld."
}
```

**Use Cases**:
- Recover from transient failures
- Retry after fixing underlying issues
- Manual intervention for stuck items

---

#### POST `/api/v1/dns/batch/flush`
**Purpose**: Manually trigger batch flush (for testing/emergency)  
**Authentication**: Required  
**Request Body**: (optional)

```json
{
  "zone": "tld."  // optional: flush specific zone only
}
```

**Response**:

```json
{
  "flushed_zones": ["tld.", "com."],
  "total_published": 45,
  "errors": 0,
  "completed_at": "2025-10-13T01:20:00Z"
}
```

**Use Cases**:
- Emergency DNS updates
- Testing
- Manual operations

---

### 4. DNS Configuration & Health

#### GET `/api/v1/dns/config`
**Purpose**: Get DNS batch publisher configuration  
**Authentication**: Required

**Response**:

```json
{
  "enabled": true,
  "batch_interval_seconds": 60,
  "max_batch_size": 10000,
  "worker_running": true,
  "last_flush": "2025-10-13T01:16:51Z",
  "next_flush": "2025-10-13T01:17:51Z"
}
```

---

#### GET `/api/v1/dns/health`
**Purpose**: DNS system health check  
**Authentication**: Required

**Response**:

```json
{
  "status": "healthy",  // healthy, degraded, unhealthy
  "checks": {
    "worker_running": true,
    "database_connected": true,
    "queue_not_stuck": true,
    "error_rate_ok": true
  },
  "metrics": {
    "pending_count": 150,
    "error_count": 3,
    "oldest_pending_seconds": 45,
    "error_rate_percent": 0.5
  },
  "issues": []
}
```

**Use Cases**:
- Monitoring/alerting
- Health checks
- Status dashboards

---

## API Organization

### Recommended Route Groups

```go
/api/v1/dns/
├── queue/
│   ├── GET    /stats                  # All zone stats
│   ├── GET    /stats/:zone            # Zone-specific stats
│   ├── GET    /pending                # List pending changes
│   ├── GET    /errors                 # List errored changes
│   ├── POST   /cleanup                # Cleanup old items
│   └── POST   /retry                  # Retry failed items
│
├── journal/
│   ├── GET    /zones                  # List all zones
│   ├── GET    /:zone                  # Zone journal entries
│   ├── GET    /:zone/serial/:serial   # Specific serial
│   └── GET    /:zone/domain/:domain   # Domain history
│
├── batch/
│   └── POST   /flush                  # Manual flush
│
└── /
    ├── GET    /config                 # Configuration
    └── GET    /health                 # Health check
```

## Implementation Priority

### Phase 1: Essential Monitoring (Implement First)
1. ✅ **GET `/api/v1/dns/queue/stats`** - Overview monitoring
2. ✅ **GET `/api/v1/dns/queue/pending`** - See what's waiting
3. ✅ **GET `/api/v1/dns/queue/errors`** - Error monitoring
4. ✅ **GET `/api/v1/dns/health`** - Health check

### Phase 2: Journal Queries
5. **GET `/api/v1/dns/journal/zones`** - Zone list
6. **GET `/api/v1/dns/journal/:zone`** - Journal entries
7. **GET `/api/v1/dns/journal/:zone/domain/:domain`** - Domain history

### Phase 3: Operations
8. **POST `/api/v1/dns/queue/cleanup`** - Maintenance
9. **POST `/api/v1/dns/batch/flush`** - Manual operations
10. **GET `/api/v1/dns/config`** - Configuration info

### Phase 4: Advanced
11. **GET `/api/v1/dns/journal/:zone/serial/:serial`** - Serial details
12. **POST `/api/v1/dns/queue/retry`** - Retry logic

## Security Considerations

1. **Authentication**: All endpoints require `TokenAuthMiddleware()`
2. **Rate Limiting**: Consider rate limiting for POST operations
3. **Input Validation**: Validate zone names, serial numbers, etc.
4. **Pagination**: Enforce max limits to prevent large queries
5. **Audit Logging**: Log all POST operations

## Performance Considerations

1. **Indexing**: Ensure proper database indexes exist
2. **Caching**: Consider caching stats for 5-10 seconds
3. **Pagination**: Default limits prevent memory issues
4. **Query Optimization**: Use views (`dns_queue_stats`) where available

## Example Client Usage

### Dashboard Monitoring
```javascript
// Get overall health
const health = await fetch('/api/v1/dns/health');

// Get queue stats for dashboard
const stats = await fetch('/api/v1/dns/queue/stats');

// Get recent errors
const errors = await fetch('/api/v1/dns/queue/errors?limit=10');
```

### Debugging Domain Issues
```javascript
// Check if domain changes are queued
const pending = await fetch('/api/v1/dns/queue/pending?domain=example.tld');

// View domain DNS history
const history = await fetch('/api/v1/dns/journal/tld/domain/example.tld');
```

### Operations
```javascript
// Manual flush for urgent update
await fetch('/api/v1/dns/batch/flush', {
  method: 'POST',
  body: JSON.stringify({ zone: 'tld.' })
});

// Cleanup old queue items
await fetch('/api/v1/dns/queue/cleanup', {
  method: 'POST',
  body: JSON.stringify({ retention_days: 7 })
});
```

## Next Steps

1. Review and approve endpoint design
2. Implement Phase 1 (Essential Monitoring)
3. Create DNSController in `internal/interface/rest/`
4. Add service methods in `batch_publisher.go` (some already exist)
5. Add Swagger documentation
6. Write integration tests
7. Update API documentation

## Questions for Discussion

1. Should we add filtering by date range?
2. Do we need a DELETE endpoint to manually remove queue items?
3. Should `/batch/flush` be restricted to specific environments?
4. Do we want real-time stats via WebSocket?
5. Should we expose metrics in Prometheus format?
