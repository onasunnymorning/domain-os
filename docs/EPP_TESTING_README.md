# EPP Testing Infrastructure

This document describes how to test the EPP server and client components.

## Quick Start

### Run Unit Tests
```bash
# All unit tests
make -f Makefile.epp test-unit

# Specific package
go test ./cmd/epp -v

# With coverage
make -f Makefile.epp test-coverage
```

### Run the Server and Client

```bash
# Terminal 1: Start the server
make -f Makefile.epp run-server

# Terminal 2: Connect with the client
make -f Makefile.epp run-client
```

## Testing Layers

### 1. Unit Tests

Unit tests verify individual functions and handlers in isolation.

**Location**: `cmd/epp/eppServer_test.go`

**Run**:
```bash
go test ./cmd/epp -v
```

**What's tested**:
- XML response format validation
- Handler functions (login, logout, domain check)
- Certificate generation
- Context management

### 2. Integration Tests

Integration tests verify components working together.

**Location**: `test/epp/integration_test.go`

**Run**:
```bash
go test ./test/epp -v -tags=integration
```

**What's tested**:
- Full EPP session lifecycle
- Client-server communication
- Concurrent connections
- Error handling

### 3. End-to-End Tests

E2E tests verify complete workflows with real dependencies.

**Location**: `test/e2e/epp_test.go`

**Run**:
```bash
go test ./test/e2e -v -tags=e2e
```

**What's tested**:
- Real database interactions
- Complete domain registration flows
- Multi-step operations

## Test Utilities

### Mock Writer

Used to capture EPP responses in tests:

```go
import "github.com/onasunnymorning/domain-os/test/testutils"

mockWriter := testutils.NewMockWriter()
sendGreeting(ctx, mockWriter, nil)
xml := mockWriter.GetWrittenXML()
```

### Test Server

Runs an EPP server on a random port for testing:

```go
server, _ := testutils.NewTestEPPServer(commandMux, tlsConfig)
server.Start()
defer server.Stop()

// Connect client to server.GetAddress()
```

## Writing Tests

### Example Unit Test

```go
func TestLoginCommand(t *testing.T) {
    // Setup
    ctx := context.Background()
    mockWriter := testutils.NewMockWriter()
    
    loginXML := `<epp>...</epp>`
    doc := etree.NewDocument()
    doc.ReadFromString(loginXML)
    
    // Execute
    respondToLoginCommand(ctx, mockWriter, doc)
    
    // Assert
    response := mockWriter.GetWrittenXML()
    assert.Contains(t, response, "1000") // Success code
}
```

### Example Integration Test

```go
func TestFullSession(t *testing.T) {
    // Start server
    server, _ := testutils.NewTestEPPServer(...)
    server.Start()
    defer server.Stop()
    
    // Connect client
    conn, _ := net.Dial("tcp", server.GetAddress())
    
    // Read greeting
    // Send login
    // Send commands
    // Send logout
}
```

## Coverage Goals

- **Unit Tests**: 80%+ line coverage
- **Integration Tests**: All critical paths
- **E2E Tests**: Main user journeys

## Running Specific Tests

```bash
# Run single test
go test ./cmd/epp -run TestSendGreeting -v

# Run tests matching pattern
go test ./cmd/epp -run Login -v

# Run with race detector
go test -race ./...

# Run with verbose output
go test -v ./...
```

## Continuous Integration

Tests run automatically on:
- Push to any branch
- Pull requests
- Scheduled daily runs

See `.github/workflows/epp-tests.yml` for configuration.

## Debugging Tests

### View test output
```bash
go test -v ./cmd/epp
```

### Run single test with debugging
```bash
go test ./cmd/epp -run TestLoginCommand -v
```

### Check test coverage
```bash
go test ./cmd/epp -cover
go test ./cmd/epp -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Performance Testing

### Load Testing
```bash
make -f Makefile.epp load-test
```

### Benchmarks
```bash
go test ./cmd/epp -bench=. -benchmem
```

## Test Data

Test XML fixtures are located in:
- `cmd/epp/testdata/` - Server test data
- `test/fixtures/` - Shared test data

## Common Issues

### Port Already in Use
If tests fail with "address already in use":
```bash
lsof -ti:700 | xargs kill -9
```

### Certificate Issues
Test servers use self-signed certificates. Client tests should skip verification:
```go
tlsConfig := &tls.Config{
    InsecureSkipVerify: true,
}
```

### Race Conditions
Always run tests with race detector in CI:
```bash
go test -race ./...
```

## Contributing

When adding new features:
1. Write unit tests first (TDD)
2. Add integration tests for workflows
3. Update this documentation
4. Ensure all tests pass: `make -f Makefile.epp test`

## Resources

- [EPP RFC 5730](https://datatracker.ietf.org/doc/html/rfc5730)
- [Go Testing Documentation](https://golang.org/pkg/testing/)
- [Testify Documentation](https://github.com/stretchr/testify)
- [EPP Testing Strategy](./EPP_TESTING_STRATEGY.md)
