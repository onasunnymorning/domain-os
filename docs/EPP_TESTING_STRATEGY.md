# EPP Testing Infrastructure Strategy

## Overview

A comprehensive testing strategy for the EPP server and client components, covering unit tests, integration tests, and end-to-end testing.

## Testing Layers

### 1. Unit Tests

#### EPP Server Unit Tests (`cmd/epp/eppServer_test.go`)

```go
// Test individual handlers in isolation
- TestSendGreeting() - Verify greeting XML format
- TestRespondToLoginCommand() - Verify login response
- TestRespondToLogoutCommand() - Verify logout and connection closure
- TestRespondToDomainCheckCommand() - Verify domain check logic
- TestGenerateCertificate() - Verify TLS certificate generation
- TestLogConnection() - Verify context management
```

**Key aspects:**
- Mock the `epp.Writer` interface
- Test XML response format compliance with EPP RFC
- Verify proper context handling
- Test error cases (malformed XML, missing fields)

#### EPP Client Unit Tests (`cmd/cli/epp/eppCliClient_test.go`)

```go
- TestFormatXML() - Verify XML formatting
- TestPrettyXMLLogger() - Test logger behavior
- TestGenerateCertificate() - Verify client certificate generation
```

### 2. Integration Tests

#### Server Integration Tests (`cmd/epp/integration_test.go`)

Test the EPP server with real network connections but mocked backend services.

```go
// Test complete EPP session flows
- TestServerStartStop() - Server lifecycle
- TestConnectionHandling() - Multiple concurrent connections
- TestSessionLifecycle() - Connect → Greeting → Login → Command → Logout
- TestCommandRouting() - Verify CommandMux routes to correct handlers
- TestTimeouts() - Idle timeout, read timeout, write timeout
- TestTLSHandshake() - Certificate validation
- TestMaxMessageSize() - Message size limits
```

**Setup:**
- Start server on random available port
- Use real TLS connections
- Mock domain/contact/host repositories
- Verify state management across commands

#### Client-Server Integration Tests (`test/epp/integration_test.go`)

Test actual client-server interaction.

```go
- TestFullEPPSession() - Complete workflow
- TestLoginFailure() - Invalid credentials handling
- TestCommandSequence() - Multiple commands in one session
- TestReconnection() - Client reconnect after disconnect
- TestConcurrentSessions() - Multiple clients simultaneously
```

### 3. End-to-End Tests

#### E2E Test Suite (`test/e2e/epp_test.go`)

Test real-world scenarios with actual database and full stack.

```go
- TestDomainRegistrationFlow() - Complete domain lifecycle
- TestBulkOperations() - Performance under load
- TestErrorRecovery() - Network failures, timeouts
- TestCompliance() - EPP RFC compliance
```

### 4. Property-Based Testing

Use property-based testing for XML handling:

```go
- TestXMLRoundTrip() - Any valid EPP XML should parse and serialize correctly
- TestCommandVariations() - Generate random valid EPP commands
- TestInvalidXMLHandling() - Fuzz testing with malformed XML
```

## Proposed Directory Structure

```
domain-os/
├── cmd/
│   ├── epp/
│   │   ├── eppServer.go
│   │   ├── eppServer_test.go          # Unit tests
│   │   ├── integration_test.go        # Integration tests
│   │   └── testdata/                  # Test XML fixtures
│   │       ├── valid_login.xml
│   │       ├── valid_domain_check.xml
│   │       └── invalid_*.xml
│   └── cli/
│       └── epp/
│           ├── eppCliClient.go
│           └── eppCliClient_test.go   # Unit tests
│
├── test/
│   ├── epp/
│   │   ├── integration_test.go        # Client-Server integration
│   │   ├── helpers.go                 # Test utilities
│   │   └── fixtures/                  # Shared test data
│   │       ├── greeting.xml
│   │       ├── login_success.xml
│   │       └── domain_responses.xml
│   │
│   ├── e2e/
│   │   ├── epp_test.go               # E2E tests
│   │   └── docker-compose.test.yml   # Test environment
│   │
│   └── testutils/
│       ├── epp_client.go             # Test client utilities
│       ├── epp_server.go             # Test server utilities
│       ├── mock_writer.go            # Mock implementations
│       └── xml_helpers.go            # XML test helpers
│
└── internal/
    └── testutils/                     # Shared test utilities
        ├── db.go                      # Test database setup
        └── fixtures.go                # Data fixtures
```

## Test Utilities to Create

### 1. Mock EPP Writer (`test/testutils/mock_writer.go`)

```go
type MockWriter struct {
    WrittenData []byte
    Closed      bool
    WriteFn     func([]byte) (int, error)
}

func (m *MockWriter) Write(p []byte) (int, error) {
    m.WrittenData = append(m.WrittenData, p...)
    if m.WriteFn != nil {
        return m.WriteFn(p)
    }
    return len(p), nil
}

func (m *MockWriter) GetWrittenXML() string {
    return string(m.WrittenData)
}
```

### 2. Test EPP Server (`test/testutils/epp_server.go`)

```go
type TestEPPServer struct {
    Server   *epp.Server
    Listener net.Listener
    Port     int
}

func NewTestEPPServer() (*TestEPPServer, error) {
    // Create server on random port
    // Return server that can be started/stopped
}

func (s *TestEPPServer) Start() error
func (s *TestEPPServer) Stop() error
func (s *TestEPPServer) GetURL() string
```

### 3. Test EPP Client (`test/testutils/epp_client.go`)

```go
type TestEPPClient struct {
    client *pkg.Client
}

func NewTestEPPClient(serverURL string) (*TestEPPClient, error)
func (c *TestEPPClient) Login(username, password string) error
func (c *TestEPPClient) CheckDomain(domain string) (*DomainCheckResponse, error)
func (c *TestEPPClient) Logout() error
func (c *TestEPPClient) SendRawXML(xml string) (string, error)
```

### 4. XML Helpers (`test/testutils/xml_helpers.go`)

```go
func LoadXMLFixture(filename string) (string, error)
func ValidateEPPXML(xml string) error
func ParseEPPResponse(xml string) (*EPPResponse, error)
func AssertXMLEqual(t *testing.T, expected, actual string)
func CompareXMLStructure(xml1, xml2 string) bool
```

## Example Test Implementation

### Unit Test Example

```go
// cmd/epp/eppServer_test.go
package main

import (
    "context"
    "testing"
    
    "github.com/beevik/etree"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestSendGreeting(t *testing.T) {
    ctx := context.Background()
    mockWriter := &MockWriter{}
    
    sendGreeting(ctx, mockWriter, nil)
    
    // Parse the written XML
    doc := etree.NewDocument()
    err := doc.ReadFromString(mockWriter.GetWrittenXML())
    require.NoError(t, err)
    
    // Verify structure
    greeting := doc.FindElement("//greeting")
    require.NotNil(t, greeting)
    
    svID := greeting.FindElement("svID")
    assert.NotNil(t, svID)
    assert.NotEmpty(t, svID.Text())
    
    svcMenu := greeting.FindElement("svcMenu")
    assert.NotNil(t, svcMenu)
}

func TestRespondToLoginCommand(t *testing.T) {
    tests := []struct {
        name        string
        inputXML    string
        wantCode    string
        wantMessage string
    }{
        {
            name: "valid login",
            inputXML: `<?xml version="1.0"?>
                <epp xmlns="urn:ietf:params:xml:ns:epp-1.0">
                    <command>
                        <login>
                            <clID>testuser</clID>
                            <pw>testpass</pw>
                        </login>
                    </command>
                </epp>`,
            wantCode:    "1000",
            wantMessage: "Command completed successfully",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            ctx := context.Background()
            mockWriter := &MockWriter{}
            
            doc := etree.NewDocument()
            err := doc.ReadFromString(tt.inputXML)
            require.NoError(t, err)
            
            respondToLoginCommand(ctx, mockWriter, doc)
            
            // Verify response
            respDoc := etree.NewDocument()
            err = respDoc.ReadFromString(mockWriter.GetWrittenXML())
            require.NoError(t, err)
            
            result := respDoc.FindElement("//result")
            require.NotNil(t, result)
            assert.Equal(t, tt.wantCode, result.SelectAttrValue("code", ""))
        })
    }
}
```

### Integration Test Example

```go
// test/epp/integration_test.go
package epp_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestFullEPPSession(t *testing.T) {
    // Start test server
    server, err := testutils.NewTestEPPServer()
    require.NoError(t, err)
    defer server.Stop()
    
    err = server.Start()
    require.NoError(t, err)
    
    // Connect client
    client, err := testutils.NewTestEPPClient(server.GetURL())
    require.NoError(t, err)
    
    // Test greeting is received automatically
    assert.NotEmpty(t, client.Greeting)
    
    // Test login
    err = client.Login("testuser", "testpass")
    assert.NoError(t, err)
    
    // Test domain check
    result, err := client.CheckDomain("example.com")
    require.NoError(t, err)
    assert.NotNil(t, result)
    
    // Test logout
    err = client.Logout()
    assert.NoError(t, err)
}

func TestConcurrentSessions(t *testing.T) {
    server, err := testutils.NewTestEPPServer()
    require.NoError(t, err)
    defer server.Stop()
    
    err = server.Start()
    require.NoError(t, err)
    
    // Spawn 10 concurrent clients
    const numClients = 10
    done := make(chan error, numClients)
    
    for i := 0; i < numClients; i++ {
        go func(id int) {
            client, err := testutils.NewTestEPPClient(server.GetURL())
            if err != nil {
                done <- err
                return
            }
            
            err = client.Login("user"+string(rune(id)), "pass")
            if err != nil {
                done <- err
                return
            }
            
            _, err = client.CheckDomain("test.com")
            done <- err
        }(i)
    }
    
    // Wait for all clients
    for i := 0; i < numClients; i++ {
        err := <-done
        assert.NoError(t, err)
    }
}
```

## CI/CD Integration

### GitHub Actions Workflow (`.github/workflows/epp-tests.yml`)

```yaml
name: EPP Tests

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      
      - name: Unit Tests
        run: |
          go test ./cmd/epp/... -v -race -coverprofile=coverage.out
          go test ./cmd/cli/epp/... -v -race
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out

  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: postgres
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      
      - name: Integration Tests
        run: go test ./test/epp/... -v -tags=integration
        env:
          DB_HOST: localhost
          DB_USER: postgres
          DB_PASS: postgres

  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.23'
      
      - name: Start test environment
        run: docker-compose -f test/e2e/docker-compose.test.yml up -d
      
      - name: Wait for services
        run: sleep 10
      
      - name: E2E Tests
        run: go test ./test/e2e/... -v -tags=e2e
      
      - name: Cleanup
        run: docker-compose -f test/e2e/docker-compose.test.yml down
```

## Test Coverage Goals

- **Unit Tests**: 80%+ coverage
- **Integration Tests**: All critical paths
- **E2E Tests**: Main user journeys
- **Performance Tests**: Baseline metrics established

## Running Tests

```bash
# Run all unit tests
make test-unit

# Run integration tests
make test-integration

# Run E2E tests
make test-e2e

# Run all tests
make test

# Run with coverage
make test-coverage

# Run specific test
go test ./cmd/epp -run TestSendGreeting -v

# Run with race detector
go test -race ./...
```

## Performance/Load Testing

### Using `vegeta` for load testing

```go
// test/performance/epp_load_test.go
package performance

import (
    "testing"
    vegeta "github.com/tsenart/vegeta/v12/lib"
)

func TestEPPServerLoad(t *testing.T) {
    rate := vegeta.Rate{Freq: 100, Per: time.Second}
    duration := 30 * time.Second
    
    targeter := vegeta.NewStaticTargeter(vegeta.Target{
        Method: "EPP",
        URL:    "epp://localhost:700",
    })
    
    attacker := vegeta.NewAttacker()
    
    var metrics vegeta.Metrics
    for res := range attacker.Attack(targeter, rate, duration, "EPP Load Test") {
        metrics.Add(res)
    }
    metrics.Close()
    
    // Assert performance requirements
    assert.Less(t, metrics.Latencies.P99, 100*time.Millisecond)
    assert.Greater(t, metrics.Success, 0.99)
}
```

## Makefile

```makefile
# Makefile
.PHONY: test test-unit test-integration test-e2e test-coverage

test: test-unit test-integration

test-unit:
	go test ./cmd/... -v -race -short

test-integration:
	go test ./test/epp/... -v -race -tags=integration

test-e2e:
	docker-compose -f test/e2e/docker-compose.test.yml up -d
	go test ./test/e2e/... -v -tags=e2e
	docker-compose -f test/e2e/docker-compose.test.yml down

test-coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html

test-bench:
	go test ./... -bench=. -benchmem

lint:
	golangci-lint run ./...
```

## Next Steps

1. **Start with unit tests** - Test individual handlers
2. **Add integration tests** - Test server lifecycle
3. **Implement test utilities** - Reusable test helpers
4. **Add E2E tests** - Full stack testing
5. **Set up CI/CD** - Automated testing on every commit
6. **Add performance tests** - Establish baselines
7. **Document test scenarios** - Maintain test cases documentation

This infrastructure will ensure robust, reliable EPP server and client implementations!
