# DNS Service Testing Strategy

## Overview

This document outlines the comprehensive testing strategy for `DNSService` in the domain-os application. The strategy follows existing patterns in the codebase while adapting to the unique requirements of DNS monitoring functionality.

## Current Testing Patterns in Codebase

### Existing Patterns (from TLDService, DomainService)

**Pattern 1: Mock Repository Pattern**
```go
type MockTLDRepository struct {
    Tlds []*entities.TLD
}

func (repo *MockTLDRepository) Create(ctx context.Context, tld *entities.TLD) error {
    repo.Tlds = append(repo.Tlds, tld)
    return nil
}
```

**Pattern 2: Table-Driven Tests**
```go
tests := []struct {
    name    string
    cmd     *commands.CreateDomainCommand
    wantErr bool
}{
    {name: "Valid command", cmd: validCmd, wantErr: false},
    {name: "Invalid command", cmd: invalidCmd, wantErr: true},
}
```

**Pattern 3: Testify Assertions**
```go
assert.NoError(t, err)
assert.NotNil(t, result)
assert.Equal(t, expected, actual)
```

## Recommended Testing Approach for DNSService

### Option 1: Mock-Based Unit Tests (RECOMMENDED)

**Approach:** Create mock BatchPublisher using sqlmock for database queries

**Pros:**
- ✅ Fast execution (no real database)
- ✅ Isolated unit tests
- ✅ Easy to test error scenarios
- ✅ CI/CD friendly
- ✅ Follows existing patterns

**Cons:**
- ❌ Requires sqlmock dependency
- ❌ Mock setup can be verbose
- ❌ Doesn't test actual SQL queries

**Implementation:**
```go
package services

import (
    "testing"
    "github.com/DATA-DOG/go-sqlmock"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
    sqlDB, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Failed to create sqlmock: %v", err)
    }

    gormDB, err := gorm.Open(postgres.New(postgres.Config{
        Conn: sqlDB,
    }), &gorm.Config{})
    if err != nil {
        t.Fatalf("Failed to create gorm DB: %v", err)
    }

    return gormDB, mock
}
```

### Option 2: Integration Tests with Test Database

**Approach:** Use real PostgreSQL database with test data

**Pros:**
- ✅ Tests real SQL queries
- ✅ Catches database-specific issues
- ✅ Tests actual BatchPublisher behavior

**Cons:**
- ❌ Slower execution
- ❌ Requires database setup
- ❌ Harder to test error scenarios
- ❌ Needs test data management

**Implementation:**
```go
func setupTestDB(t *testing.T) *gorm.DB {
    dsn := "host=localhost user=test password=test dbname=domain_os_test"
    db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
    if err != nil {
        t.Skip("Test database not available")
    }
    
    // Run migrations
    // Seed test data
    
    return db
}
```

### Option 3: Hybrid Approach

**Approach:** Unit tests with mocks + integration tests for critical paths

**Benefits:**
- ✅ Fast unit tests for most scenarios
- ✅ Integration tests verify real behavior
- ✅ Best of both worlds

## Comprehensive Test Coverage Plan

### 1. GetQueueStats Tests

#### Test Cases:
```go
TestDNSService_GetQueueStats_Success
    - Multiple zones with stats
    - Verify totals calculated correctly
    - Verify individual zone stats
    
TestDNSService_GetQueueStats_EmptyQueue
    - No zones/stats exist
    - Returns zeros for all totals
    
TestDNSService_GetQueueStats_SingleZone
    - One zone with stats
    - Totals equal single zone values
    
TestDNSService_GetQueueStats_DatabaseError
    - GetQueueStats fails
    - Returns appropriate error
    
TestDNSService_GetQueueStats_NilPublisher
    - Service created without publisher
    - Returns error gracefully
```

#### Mock Setup Example:
```go
func TestDNSService_GetQueueStats_Success(t *testing.T) {
    bp, mock := setupMockBatchPublisher(t)
    service := NewDNSService(bp)

    // Mock GetQueueStats response
    rows := sqlmock.NewRows([]string{
        "zone_name", "pending_count", "published_count", "error_count",
    }).
        AddRow("tld.", 10, 100, 2).
        AddRow("example.tld.", 5, 50, 0)

    mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "dns_queue_stats"`)).
        WillReturnRows(rows)

    // Execute
    result, err := service.GetQueueStats(context.Background())

    // Assert
    assert.NoError(t, err)
    assert.Len(t, result.Stats, 2)
    assert.Equal(t, int64(15), result.TotalPending)
    assert.Equal(t, int64(150), result.TotalPublished)
    assert.Equal(t, int64(2), result.TotalErrors)
}
```

### 2. GetQueueStatsForZone Tests

#### Test Cases:
```go
TestDNSService_GetQueueStatsForZone_Success
    - Zone exists
    - Returns correct stats
    
TestDNSService_GetQueueStatsForZone_ZoneNotFound
    - Zone doesn't exist
    - Returns zeros (not error)
    
TestDNSService_GetQueueStatsForZone_EmptyZoneParam
    - Zone parameter empty
    - Returns error or all zones
    
TestDNSService_GetQueueStatsForZone_DatabaseError
    - Query fails
    - Returns error
```

### 3. GetPendingChanges Tests

#### Test Cases:
```go
TestDNSService_GetPendingChanges_NoFilters
    - Returns all pending changes
    - Respects default pagination
    
TestDNSService_GetPendingChanges_ZoneFilter
    - Filters by zone correctly
    - SQL WHERE clause includes zone
    
TestDNSService_GetPendingChanges_DomainFilter
    - Filters by domain correctly
    
TestDNSService_GetPendingChanges_ChangeTypeFilter
    - Filters by ADD/UPDATE/DELETE
    
TestDNSService_GetPendingChanges_RecordTypeFilter
    - Filters by NS/DS/etc
    
TestDNSService_GetPendingChanges_AllFilters
    - Combines all filters
    - AND conditions applied
    
TestDNSService_GetPendingChanges_Pagination
    - Limit and offset work
    - Returns correct page
    
TestDNSService_GetPendingChanges_EmptyResults
    - No matching changes
    - Returns empty array (not null)
    
TestDNSService_GetPendingChanges_DatabaseError
    - Query fails
    - Returns error
    
TestDNSService_GetPendingChanges_LimitBoundaries
    - Test limit = 0, 1, 1000, 9999
    - Ensure no SQL injection
```

#### Table-Driven Test Example:
```go
func TestDNSService_GetPendingChanges_Filters(t *testing.T) {
    tests := []struct {
        name       string
        zone       string
        domain     string
        changeType string
        recordType string
        wantSQL    string
        wantArgs   []interface{}
    }{
        {
            name:     "No filters",
            wantSQL:  "WHERE published_at IS NULL",
            wantArgs: []interface{}{},
        },
        {
            name:     "Zone only",
            zone:     "tld.",
            wantSQL:  "WHERE published_at IS NULL AND zone_name = ?",
            wantArgs: []interface{}{"tld."},
        },
        {
            name:       "All filters",
            zone:       "tld.",
            domain:     "example.tld.",
            changeType: "ADD",
            recordType: "NS",
            wantSQL:    "WHERE published_at IS NULL AND zone_name = ? AND domain_name = ? AND change_type = ? AND record_type = ?",
            wantArgs:   []interface{}{"tld.", "example.tld.", "ADD", "NS"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### 4. GetErroredChanges Tests

#### Test Cases:
```go
TestDNSService_GetErroredChanges_NoFilter
    - Returns all errors
    - Error message not null/empty
    
TestDNSService_GetErroredChanges_ZoneFilter
    - Filters errors by zone
    
TestDNSService_GetErroredChanges_Pagination
    - Limit and offset work
    
TestDNSService_GetErroredChanges_NoErrors
    - Empty result set
    - Returns empty array
    
TestDNSService_GetErroredChanges_DatabaseError
    - Query fails
    - Returns error
    
TestDNSService_GetErroredChanges_VerifyErrorMessages
    - Error messages populated
    - Error timestamps present
```

### 5. GetHealth Tests

#### Test Cases:
```go
TestDNSService_GetHealth_AllHealthy
    - All checks pass
    - Status = "healthy"
    - All checks true
    - Metrics populated
    
TestDNSService_GetHealth_NilPublisher
    - Publisher not configured
    - Status = "unavailable"
    - Publisher check false
    
TestDNSService_GetHealth_DatabaseUnavailable
    - GetDB returns nil
    - Status = "unhealthy"
    - Database check false
    
TestDNSService_GetHealth_DatabasePingFails
    - Ping returns error
    - Status = "unhealthy"
    - Connection check false
    
TestDNSService_GetHealth_MissingQueueTable
    - dns_change_queue doesn't exist
    - Status = "unhealthy"
    - Queue table check false
    
TestDNSService_GetHealth_MissingJournalTable
    - dns_zone_journal doesn't exist
    - Status = "unhealthy"
    - Journal table check false
    
TestDNSService_GetHealth_MissingStatsView
    - dns_queue_stats doesn't exist
    - Status = "unhealthy"
    - Stats view check false
    
TestDNSService_GetHealth_HighQueueCount
    - Pending > 10000
    - Status = "healthy" (or "degraded")
    - Metrics show high count
    
TestDNSService_GetHealth_MultipleFailures
    - Multiple checks fail
    - Status = "unhealthy"
    - Issues array populated
    
TestDNSService_GetHealth_MetricsVerification
    - Verify all metrics present
    - Batch interval correct
    - Batch size correct
```

#### Mock Example for Health Check:
```go
func TestDNSService_GetHealth_AllHealthy(t *testing.T) {
    bp, mock := setupMockBatchPublisher(t)
    service := NewDNSService(bp)

    // Mock DB ping
    mock.ExpectPing()

    // Mock table existence checks
    mock.ExpectQuery(`SELECT count(*) FROM information_schema.tables WHERE table_name = ?`).
        WithArgs("dns_change_queue").
        WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

    mock.ExpectQuery(`SELECT count(*) FROM information_schema.tables WHERE table_name = ?`).
        WithArgs("dns_zone_journal").
        WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

    mock.ExpectQuery(`SELECT count(*) FROM information_schema.views WHERE table_name = ?`).
        WithArgs("dns_queue_stats").
        WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

    // Mock GetQueueStats for metrics
    statsRows := sqlmock.NewRows([]string{
        "zone_name", "pending_count", "published_count", "error_count",
    }).AddRow("tld.", 10, 100, 2)
    
    mock.ExpectQuery(`SELECT * FROM "dns_queue_stats"`).
        WillReturnRows(statsRows)

    // Execute
    result, err := service.GetHealth(context.Background())

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, "healthy", result.Status)
    assert.True(t, result.Checks["publisher_initialized"])
    assert.True(t, result.Checks["database_available"])
    assert.True(t, result.Checks["database_connected"])
    assert.True(t, result.Checks["queue_table_exists"])
    assert.True(t, result.Checks["journal_table_exists"])
    assert.True(t, result.Checks["stats_view_exists"])
    
    assert.Equal(t, int64(10), result.Metrics["total_pending"])
    assert.Equal(t, int64(100), result.Metrics["total_published"])
    assert.Equal(t, int64(2), result.Metrics["total_errors"])
    
    assert.NoError(t, mock.ExpectationsWereMet())
}
```

## Implementation Steps

### Step 1: Add Required Dependencies

Add to `go.mod`:
```bash
go get github.com/DATA-DOG/go-sqlmock
go get github.com/stretchr/testify
```

### Step 2: Create Test Helper File

Create `internal/application/services/dns_service_test_helpers.go`:
```go
package services

import (
    "testing"
    "time"
    
    "github.com/DATA-DOG/go-sqlmock"
    "github.com/onasunnymorning/domain-os/internal/infrastructure/dnsevents"
    "gorm.io/driver/postgres"
    "gorm.io/gorm"
)

// setupMockDB creates a mock database connection
func setupMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
    sqlDB, mock, err := sqlmock.New()
    if err != nil {
        t.Fatalf("Failed to create sqlmock: %v", err)
    }

    gormDB, err := gorm.Open(postgres.New(postgres.Config{
        Conn: sqlDB,
    }), &gorm.Config{})
    if err != nil {
        t.Fatalf("Failed to create gorm DB: %v", err)
    }

    return gormDB, mock
}

// setupMockBatchPublisher creates a BatchPublisher with mock DB
func setupMockBatchPublisher(t *testing.T) (*dnsevents.BatchPublisher, sqlmock.Sqlmock) {
    gormDB, mock := setupMockDB(t)

    config := &dnsevents.BatchPublisherConfig{
        BatchInterval: 60 * time.Second,
        MaxBatchSize:  100,
    }

    bp := dnsevents.NewBatchPublisher(gormDB, config)
    return bp, mock
}
```

### Step 3: Create Test Files

Create tests in order of complexity:
1. `TestDNSService_GetQueueStats_*` (simpler - uses GetQueueStats method)
2. `TestDNSService_GetQueueStatsForZone_*` (similar to above)
3. `TestDNSService_GetHealth_*` (medium - multiple checks)
4. `TestDNSService_GetPendingChanges_*` (complex - many filters)
5. `TestDNSService_GetErroredChanges_*` (similar to pending)

### Step 4: Run Tests

```bash
# Run all DNS service tests
go test ./internal/application/services -run TestDNSService -v

# Run specific test
go test ./internal/application/services -run TestDNSService_GetHealth -v

# Run with coverage
go test ./internal/application/services -run TestDNSService -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Test Coverage Goals

### Minimum Coverage (MVP)
- ✅ Happy path for each method (5 tests)
- ✅ Nil publisher error handling (5 tests)
- ✅ Database error handling (5 tests)
- **Total: 15 tests**

### Good Coverage (Recommended)
- ✅ All happy paths
- ✅ All error scenarios
- ✅ Filter combinations
- ✅ Pagination edge cases
- ✅ Health check variations
- **Total: 30-40 tests**

### Excellent Coverage (Ideal)
- ✅ All good coverage tests
- ✅ Edge cases (empty strings, large limits)
- ✅ Performance tests
- ✅ Integration tests with real DB
- **Total: 50+ tests**

## Alternative: Simpler Approach Without Mocks

If you want to avoid sqlmock complexity, consider:

### Option: Test through Controller Layer

Since the controller tests already exist, you could:
1. Keep controller tests comprehensive
2. Add lightweight service tests for business logic only
3. Rely on integration tests for database queries

**Minimal Service Test Example:**
```go
func TestDNSService_CalculateTotals(t *testing.T) {
    // Test the total calculation logic without DB
    stats := []dnsevents.QueueStats{
        {PendingCount: 10, PublishedCount: 100, ErrorCount: 2},
        {PendingCount: 5, PublishedCount: 50, ErrorCount: 0},
    }
    
    totalPending := int64(0)
    totalPublished := int64(0)
    totalErrors := int64(0)
    
    for _, stat := range stats {
        totalPending += stat.PendingCount
        totalPublished += stat.PublishedCount
        totalErrors += stat.ErrorCount
    }
    
    assert.Equal(t, int64(15), totalPending)
    assert.Equal(t, int64(150), totalPublished)
    assert.Equal(t, int64(2), totalErrors)
}
```

## Integration Test Strategy

For end-to-end confidence, create integration tests:

### Location
`test/integration/dns_service_test.go`

### Setup
```go
func TestMain(m *testing.M) {
    // Setup test database
    db := setupTestDatabase()
    defer cleanupTestDatabase(db)
    
    // Run tests
    os.Exit(m.Run())
}

func TestDNSService_Integration_PendingChanges(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Use real database
    db := getTestDB(t)
    bp := dnsevents.NewBatchPublisher(db, config)
    service := NewDNSService(bp)
    
    // Seed test data
    seedDNSChanges(t, db)
    
    // Test
    result, err := service.GetPendingChanges(context.Background(), "", "", "", "", 100, 0)
    
    // Assert against real data
    assert.NoError(t, err)
    assert.NotEmpty(t, result.Changes)
}
```

### Run Integration Tests
```bash
# Skip integration tests
go test ./... -short

# Run only integration tests
go test ./test/integration -v

# Run all tests including integration
go test ./...
```

## Recommended Approach Summary

**For MVP (Minimum Viable Product):**
1. ✅ Keep existing controller tests
2. ✅ Add 15 basic service tests with sqlmock
3. ✅ Add 3-5 integration tests
4. ✅ Achieve ~70% coverage

**For Production:**
1. ✅ Implement all 30-40 service tests with sqlmock
2. ✅ Add comprehensive integration test suite
3. ✅ Add performance/load tests
4. ✅ Achieve 85%+ coverage

**Start with:** GetHealth tests (simpler) → GetQueueStats → GetPendingChanges (most complex)

## Next Steps

1. **Add dependencies:**
   ```bash
   go get github.com/DATA-DOG/go-sqlmock
   go get github.com/stretchr/testify
   ```

2. **Create test helper file** with setupMockBatchPublisher

3. **Implement tests in phases:**
   - Phase 1: Health check tests (5 tests)
   - Phase 2: Queue stats tests (6 tests)
   - Phase 3: Pending/errored changes tests (15+ tests)

4. **Run and verify:**
   ```bash
   go test ./internal/application/services -v -cover
   ```

5. **Add integration tests** when time permits

Would you like me to implement any specific test category first?
