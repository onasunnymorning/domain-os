# API Integration Testing Guide

> **Audience**: AI agents and human developers contributing to the domain-os integration test suite.

## Philosophy

Our API integration tests follow a **story-driven** approach. Each test file tells a narrative that mirrors how a real API consumer would use the system. This is our most important testing principle — preserve it.

### Why Story-Driven?

Domain-os is a **domain registry platform**. Its core value is managing domain lifecycles: register → renew → expire → restore → purge. These operations form sequential chains where each step depends on the previous one. Testing them in isolation would miss the integration bugs that matter most.

```
❌ BAD: Test register in isolation, test renew in isolation
   → Misses: "renew fails after register because status wasn't set correctly"

✅ GOOD: Register → verify state → renew → verify state → expire → verify state
   → Catches: real integration bugs in the lifecycle chain
```

## Architecture

### Test Harness

All tests share a single `TestAPI` instance initialized in `BeforeSuite`:

```
internal/interface/rest/tests/
├── api_test_suite.go          # Shared TestAPI harness + HTTP helpers
├── controller_test.go         # Suite bootstrap (BeforeSuite/AfterSuite)
├── registrar_controller_test.go  # Tests + testRegistrar() helper
├── contact_controller_test.go
├── tld_controller_test.go
├── host_controller_test.go
├── domain_controller_test.go
└── ...
```

**`TestAPI`** provides:
- `api.GET(path)`, `api.POST(path, body)`, `api.PUT(path, body)`, `api.DELETE(path)`, `api.PATCH(path, body)`, `api.POSTNoBody(path)` — HTTP helpers that return `*httptest.ResponseRecorder`
- `DecodeJSON(resp, &target)` — response body decoder
- All 17 service instances for direct data access if needed
- A real Postgres database (auto-migrated)
- `httptest.Server` with `ContextWithFallback = true` (prevents gin.Context pool races)

### Test Database

Tests run against a real Postgres instance. Configuration via env vars:

| Variable | Default | Description |
|---|---|---|
| `TEST_DB_HOST` | `127.0.0.1` | Database host |
| `TEST_DB_PORT` | `5432` | Database port |
| `TEST_DB_USER` | `postgres` | Database user |
| `TEST_DB_PASS` | `unittest` | Database password |
| `TEST_DB_NAME` | `dos_integration_tests` | Database name |
| `TEST_DB_SSLMODE` | `disable` | SSL mode |

Run locally with `make test-api` (starts its own Postgres on port 5433).

## Writing Tests — The Rules

### Rule 1: Use `Ordered` Containers for Story-Based Tests

Every controller test `Describe` block MUST use the `Ordered` decorator. This guarantees specs run sequentially, which is essential for story-driven testing.

```go
var _ = Describe("DomainLifecycle", Ordered, func() {
    BeforeAll(func() { /* create prerequisites */ })

    It("should register a domain")          // Step 1
    It("should renew the domain")           // Step 2 — depends on Step 1
    It("should expire the domain")          // Step 3 — depends on Step 2

    AfterAll(func() { /* best-effort cleanup */ })
})
```

### Rule 2: Test Both Happy and Unhappy Paths

Every endpoint should have tests for:
- ✅ **Happy path** — correct input produces correct output
- ❌ **Duplicate/conflict** — creating something that already exists returns 400/409
- ❌ **Not found** — requesting something that doesn't exist returns 404
- ❌ **Invalid input** — malformed data returns 400 with a meaningful error
- ❌ **Post-deletion** — verifying an entity is truly gone after DELETE

```go
It("should create a registrar", func() { /* 201 */ })
It("should fail to create a duplicate registrar", func() { /* 400/409 */ })
It("should get the registrar", func() { /* 200 + verify fields */ })
It("should return 404 for a non-existent registrar", func() { /* 404 */ })
It("should delete the registrar", func() { /* 204 */ })
It("should return 404 after deletion", func() { /* 404 */ })
```

### Rule 3: Decode into Go Structs, Not Maps

Always decode API responses into the **same Go types** the API returns. This gives compile-time safety — if a field is renamed, the test won't compile.

```go
// ✅ GOOD — type-safe, compiler catches renames
var domain entities.Domain
err := DecodeJSON(resp, &domain)
Expect(domain.Name.String()).To(Equal("example.com"))

// ❌ BAD — stringly typed, silently passes if field is renamed
var result map[string]interface{}
err := DecodeJSON(resp, &result)
Expect(result["Name"]).To(Equal("example.com"))  // won't catch renames
```

**Exception**: Use `map[string]interface{}` only for list endpoints where the `Data` field contains an `interface{}` slice that can't be directly deserialized into a concrete type (e.g., `ListItemResult.Meta.Filter`).

### Rule 4: Use Unique Identifiers Per Test File

Each test file creates its own prerequisite entities (registrars, TLDs, etc.). To prevent cross-file conflicts in the shared database:

- **ClID values** must be ≤ 16 characters (EPP `ClIDType` max length) and unique per file
- **GurID values** must be unique per file (use 10001, 10002, 10003, etc.)
- **Entity names** should be prefixed with the test file context

```go
// host_controller_test.go
const registrarClid = "hostTestRar"  // 11 chars, unique to this file
// GurID: 10003

// contact_controller_test.go
registrarClid := "contTestRar"      // 11 chars, unique to this file
// GurID: 10002
```

### Rule 5: Prerequisites Go in `BeforeAll`, Cleanup in `AfterAll`

**`BeforeAll`**: Create all prerequisite entities the story needs. Assert that creation succeeds — if a prerequisite fails, the entire story should abort.

**`AfterAll`**: Best-effort cleanup. Do NOT assert on cleanup operations — entity relationships or prior test failures may prevent clean deletion.

```go
BeforeAll(func() {
    // Create prerequisites — assert success
    resp := api.POST("/registry-operators", ryCmd)
    Expect(resp.Code).To(Equal(http.StatusCreated))  // ← assert
})

AfterAll(func() {
    // Best-effort cleanup — no assertions
    api.DELETE("/registry-operators/" + ryID)  // ← fire and forget
})
```

### Rule 6: Pass `ctx.Request.Context()` in Controllers, Not `ctx`

When writing or modifying controllers (not tests), always pass `ctx.Request.Context()` to service methods, never the raw `*gin.Context`. This prevents a data race between Gin's context pool and GORM's background goroutines.

```go
// ✅ GOOD — detaches from Gin's pooled context
result, err := ctrl.service.Create(ctx.Request.Context(), &cmd)

// ❌ BAD — causes data race under concurrent load
result, err := ctrl.service.Create(ctx, &cmd)
```

### Rule 7: Timestamps Must Be UTC

The domain-os entity layer validates that timestamps are UTC. Always use `.UTC()` when creating time values:

```go
// ✅ GOOD
Starts: time.Now().UTC().Add(-24 * time.Hour)

// ❌ BAD — will fail validation with "timestamp not UTC"
Starts: time.Now().Add(-24 * time.Hour)
```

## Test File Template

Use this as a starting point for new controller test files:

```go
package tests

import (
    "fmt"
    "net/http"

    "github.com/onasunnymorning/domain-os/internal/application/commands"
    "github.com/onasunnymorning/domain-os/pkg/domain/entities"
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
)

var _ = Describe("XxxController", Ordered, func() {
    // --- Constants unique to this test file ---
    const (
        someID = "xxxTestID"
    )

    // --- Shared state across specs ---
    var createdEntity entities.Xxx

    BeforeAll(func() {
        // Create prerequisites
    })

    // === Happy Path Story ===

    It("should create a new xxx", func() {
        resp := api.POST("/xxx", createCmd)
        Expect(resp.Code).To(Equal(http.StatusCreated))

        err := DecodeJSON(resp, &createdEntity)
        Expect(err).NotTo(HaveOccurred())
        Expect(createdEntity.Name).To(Equal("expected"))
    })

    It("should get the xxx by ID", func() {
        resp := api.GET(fmt.Sprintf("/xxx/%s", someID))
        Expect(resp.Code).To(Equal(http.StatusOK))

        var result entities.Xxx
        err := DecodeJSON(resp, &result)
        Expect(err).NotTo(HaveOccurred())
        Expect(result.ID).To(Equal(someID))
    })

    // === Unhappy Path ===

    It("should fail to create a duplicate xxx", func() {
        resp := api.POST("/xxx", createCmd)
        Expect(resp.Code).To(Equal(http.StatusBadRequest))
    })

    It("should return 404 for non-existent xxx", func() {
        resp := api.GET("/xxx/doesnotexist")
        Expect(resp.Code).To(Equal(http.StatusNotFound))
    })

    // === Cleanup Verification ===

    It("should delete the xxx", func() {
        resp := api.DELETE(fmt.Sprintf("/xxx/%s", someID))
        Expect(resp.Code).To(Equal(http.StatusNoContent))
    })

    It("should return 404 after deletion", func() {
        resp := api.GET(fmt.Sprintf("/xxx/%s", someID))
        Expect(resp.Code).To(Equal(http.StatusNotFound))
    })

    AfterAll(func() {
        // Best-effort cleanup of prerequisites (no assertions)
        api.DELETE("/prerequisites/" + prereqID)
    })
})
```

## Domain Lifecycle Tests — The Gold Standard

The domain lifecycle is the most complex test story in the suite. It models the full EPP lifecycle:

```
Setup: RyOp → TLD → Launch Phase → GA Phase → Registrar → Accreditation → Contact → Hosts
  │
  ├─ Register domain (with hosts, registrant contact)
  ├─ Verify domain is NOT available
  ├─ Renew domain (happy path)
  ├─ Renew domain (already renewed — error)
  ├─ Renew non-existent domain (404)
  ├─ Check canAutoRenew
  ├─ AutoRenew domain
  ├─ Update domain (change registrant, authInfo)
  ├─ Mark domain for deletion (pendingDelete)
  ├─ Attempt double mark-for-deletion (error)
  ├─ Restore domain (undo pendingDelete)
  ├─ Restore already-active domain (error)
  ├─ Expire domain
  ├─ Renew after expiry (error — expired domains can't renew)
  ├─ Delete domain (admin)
  └─ Verify domain is gone (404)
  │
Teardown: reverse order cleanup
```

Each step verifies not just the status code but also the **state of the domain entity** after the operation. This is what catches real bugs — operations that return 200 but leave the entity in an incorrect state.

## Checklist for Adding a New Controller Test

- [ ] File named `{controller}_controller_test.go` in `internal/interface/rest/tests/`
- [ ] Uses `Ordered` decorator on `Describe`
- [ ] Unique ClIDs (≤16 chars) and GurIDs for prerequisite entities
- [ ] `BeforeAll` creates prerequisites with assertions
- [ ] `AfterAll` does best-effort cleanup without assertions
- [ ] Happy path: create → get → list → count → update → delete → verify 404
- [ ] Unhappy path: duplicate create, not found, invalid input
- [ ] Responses decoded into Go structs (not maps)
- [ ] Timestamps use `.UTC()`
- [ ] Builds cleanly: `go build ./internal/interface/rest/tests/...`
- [ ] Passes with race detector: `go test -race ./internal/interface/rest/tests/...`

## Running Tests

```bash
# Run API integration tests only
make test-api

# Run with race detector (same as CI)
make ci-test-backend

# Run full local CI pipeline (lint + unit + integration + API)
make ci-local
```
