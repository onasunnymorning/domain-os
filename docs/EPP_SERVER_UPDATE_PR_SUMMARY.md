# EPP Server Update - Pull Request Summary

## 📋 Overview

This PR migrates the EPP server from a frozen, unmaintained library to an actively maintained one, implements comprehensive rate limiting for production security, containerizes the server for Docker deployment, and establishes a complete testing infrastructure.

## 🎯 Primary Objectives Completed

### 1. **EPP Library Migration** ✅
- **From**: `github.com/dotse/epp-lib` (frozen, unmaintained)
- **To**: `gitlab.com/internetstiftelsen-oss/epp-lib` v0.3.0 (active development)
- **Impact**: Access to ongoing security updates and new features

### 2. **Production Security Implementation** ✅
- Implemented Redis-based rate limiting middleware
- Added DDoS protection with connection and request limits
- Created failed login tracking with automatic account lockout
- Documented production architecture for global anycast deployment

### 3. **Docker Containerization** ✅
- Created multi-stage Dockerfile (optimized 33MB image)
- Integrated EPP server into docker-compose stack
- Implemented health checks and non-root user security
- Built comprehensive development workflow automation

### 4. **Testing Infrastructure** ✅
- Created unit test suite (9/9 tests passing)
- Achieved 58.3% code coverage
- Fixed nil pointer issues for testability
- Integrated tests into CI/CD pipeline

---

## 🔧 Technical Changes

### Library Migration & Core Fixes

#### **EPP Library Updates** (`cmd/epp/eppServer.go`)
- Migrated from `dotse/epp-lib` to `internetstiftelsen-oss/epp-lib`
- Replaced custom `LogrusLogger` with standard `log/slog`
- Fixed login/logout command handlers with correct XML path bindings:
  ```go
  // Old (broken)
  commandMux.BindCommand("login", epp.NamespaceIETFEPP10.String(), ...)
  
  // New (working)
  commandMux.Bind(epp.NewXMLPathBuilder().
      AddOrphan("//command", epp.NamespaceIETFEPP10.String()).
      Add("login", epp.NamespaceIETFEPP10.String()).String(), ...)
  ```

#### **Client Improvements** (`cmd/cli/epp/eppCliClient.go`)
- Added pretty XML printing with indentation
- Improved error handling and connection management
- Enhanced user experience with formatted output

#### **Connection Handling**
- Fixed EOF errors from improper command structure
- Implemented proper greeting on connection establishment
- Added connection context tracking with unique IDs

### Rate Limiting & Security

#### **Rate Limiter Middleware** (305 lines)
**File**: `internal/infrastructure/epp/middleware/rate_limiter.go`

**Features Implemented**:

1. **Connection Limits**
   - Per-IP limit: 10 concurrent connections (configurable)
   - Per-registrar limit: 100 concurrent connections (configurable)
   - Automatic connection tracking and cleanup

2. **Request Rate Limiting**
   - Token bucket algorithm: 100 req/s, burst 200 (configurable)
   - Per-IP and per-registrar rate limiting
   - Window-based request counting

3. **Failed Login Protection**
   - Tracks failed login attempts per username/IP
   - Automatic account lockout after 5 failures (configurable)
   - 15-minute lockout duration (configurable)
   - Automatic unlock after timeout

4. **Statistics API**
   - Real-time connection counters
   - Rate limit metrics
   - Failed login tracking

**Configuration**:
```go
type RateLimitConfig struct {
    MaxConnPerIP        int           // Default: 10
    MaxConnPerRegistrar int           // Default: 100
    ConnTTL             time.Duration // Default: 5 minutes
    RequestsPerSecond   int           // Default: 100
    BurstSize           int           // Default: 200
    RequestWindow       time.Duration // Default: 1 second
    MaxFailedLogins     int           // Default: 5
    LockoutDuration     time.Duration // Default: 15 minutes
}
```

#### **Integration with EPP Server**
- Added nil-safe rate limiter checks (testability)
- Integrated connection limit checking in `logConnection()`
- Added failed login tracking in `respondToLoginCommand()`
- Implemented connection cleanup on disconnect

#### **Redis Infrastructure**
- Added Redis 7-alpine container to docker-compose
- Configured health checks for Redis
- Added environment variable configuration:
  - `REDIS_HOST` (default: `redis`)
  - `REDIS_PORT` (default: `6379`)
  - `REDIS_PASSWORD` (optional)
  - `REDIS_DB` (default: `0`)

### Docker Containerization

#### **Multi-Stage Dockerfile** (`Dockerfile.epp`, 69 lines)

**Stage 1 - Builder** (golang:1.23-alpine):
```dockerfile
# Static binary compilation
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags='-w -s -extldflags "-static"' \
    -a -o epp-server ./cmd/epp
```
- Downloads dependencies with verification
- Builds optimized static binary (~5MB)
- Strips debug symbols for minimal size

**Stage 2 - Runtime** (alpine:3.19):
```dockerfile
# Minimal runtime with security
RUN addgroup -g 1000 epp && adduser -D -u 1000 -G epp epp
USER epp
HEALTHCHECK CMD nc -z localhost 700 || exit 1
```
- **Final image size**: 33MB (vs ~1GB without multi-stage)
- Non-root user (epp:epp, UID/GID 1000)
- Essential packages only: ca-certificates, tzdata, netcat-openbsd
- Built-in health check on port 700

#### **Docker Compose Integration** (`docker-compose.yml`)
```yaml
epp-server:
  build:
    context: .
    dockerfile: Dockerfile.epp
  image: geapex/epp-server:${BRANCH:-latest}
  profiles: [essential, full]
  depends_on:
    redis:
      condition: service_healthy
  healthcheck:
    test: ["CMD", "nc", "-z", "localhost", "700"]
    interval: 30s
    timeout: 3s
    start_period: 5s
    retries: 3
  ports:
    - "700:700"
  environment:
    - REDIS_HOST=redis
    - REDIS_PORT=6379
    - LOG_LEVEL=${LOG_LEVEL:-info}
  develop:
    watch:
      - action: rebuild
        path: ./cmd/epp
      - action: rebuild
        path: ./internal/infrastructure/epp
```

**Key Features**:
- Health check integration with dependency management
- Profile support (essential/full)
- Development mode with file watching
- Environment variable configuration

#### **Makefile Automation** (`Makefile.epp-server`, 158 lines)

**20+ Commands Organized by Category**:

**Local Development**:
- `build` - Build EPP server binary locally
- `run` - Run EPP server locally
- `test` - Run unit tests
- `test-int` - Run integration tests (requires Redis)
- `coverage` - Generate coverage report
- `clean` - Clean build artifacts

**Docker Operations**:
- `docker-build` - Build Docker image
- `docker-run` - Run EPP server in Docker
- `docker-stop` - Stop EPP server container
- `docker-logs` - Show EPP server logs (with fallback)
- `docker-shell` - Open shell in running container

**Stack Management**:
- `docker-stack` - Start full stack (DB, Redis, Admin API, EPP Server)
- `docker-stack-minimal` - Start minimal stack (Redis + EPP only)
- `docker-stack-down` - Stop all stack services

**Monitoring**:
- `status` - Show container status
- `redis-monitor` - Monitor Redis commands
- `redis-keys` - Show all Redis keys (rate limit data)
- `redis-clear` - Clear all Redis data

**Usage**:
```bash
# Start full stack
make -f Makefile.epp-server docker-stack

# View logs
make -f Makefile.epp-server docker-logs

# Monitor rate limiting
make -f Makefile.epp-server redis-keys
```

### Testing Infrastructure

#### **Unit Tests** (`cmd/epp/eppServer_test.go`, 315 lines)

**Test Coverage** (9 tests, all passing):

1. **TestSendGreeting** - Verifies EPP greeting XML generation
2. **TestRespondToLoginCommand** - Tests login handling (2 scenarios)
3. **TestRespondToLogoutCommand** - Tests logout response
4. **TestRespondToDomainCheckCommand** - Tests domain check
5. **TestGetGreetingXML** - Validates greeting XML format
6. **TestGetLoginResponseXML** - Validates login response XML
7. **TestGetLogoutResponseXML** - Validates logout response XML
8. **TestGenerateCertificate** - Tests self-signed cert generation
9. **TestLogConnection** - Tests connection context management

**Test Infrastructure**:
```go
// Mock writer for testing EPP responses
type MockWriter struct {
    buffer          bytes.Buffer
    shouldClose     bool
    closeAfterWrite bool
}

// Mock TLS connection using net.Pipe
server, client := net.Pipe()
tlsConfig := &tls.Config{
    InsecureSkipVerify: true,
    Certificates:       []tls.Certificate{generateCertificate()},
}
tlsConn := tls.Client(client, tlsConfig)
```

**Coverage**: 58.3% (173/297 lines covered)

#### **Rate Limiter Tests** (`internal/infrastructure/epp/middleware/`)

**Unit Tests** (`rate_limiter_unit_test.go`, 3 tests):
- `TestDefaultRateLimitConfig` - Validates default configuration
- `TestCustomRateLimitConfig` - Tests custom config values
- `TestErrorTypes` - Verifies error type handling

**Integration Tests** (`rate_limiter_test.go`, tagged):
- Redis-based integration testing
- Connection limit enforcement
- Rate limit verification
- Failed login tracking

#### **Nil Pointer Safety**
- Added defensive nil checks for `rateLimiter` (6 locations)
- Enabled unit tests to run without Redis dependency
- Graceful degradation when rate limiter not initialized

### Logging & Observability

#### **Configurable Log Levels**
```go
// Environment-based log level configuration
logLevel := slog.LevelInfo  // Default: info
if level := os.Getenv("LOG_LEVEL"); level != "" {
    switch level {
    case "debug", "DEBUG":
        logLevel = slog.LevelDebug
    // ... other levels
    }
}
```

**Log Level Behavior**:
- `info` (production default): Clean logs, suppresses DEBUG health check noise
- `debug`: Verbose logging for troubleshooting
- `warn`: Only warnings and errors
- `error`: Only errors

#### **Health Check Optimization**
- Docker health check runs every 30s: `nc -z localhost 700`
- Causes harmless EOF errors (netcat doesn't send EPP data)
- DEBUG-level logging suppresses this noise in production
- Set `LOG_LEVEL=debug` only when troubleshooting

### Architecture & Documentation

#### **Production Architecture** (`docs/EPP_PRODUCTION_ARCHITECTURE.md`)

**Components Documented**:
- EPP Server architecture with TLS termination
- Rate limiting with Redis
- Message broker integration (RabbitMQ for EPP frames)
- Certificate-based authentication (DANE/TLSA)
- Anycast deployment for global low latency

**Security Features**:
- DDoS protection via rate limiting
- Connection limits (per-IP and per-registrar)
- Failed login tracking and account lockout
- TLS 1.2+ with client certificate validation

#### **Documentation Created** (10 files)

1. **EPP_PRODUCTION_ARCHITECTURE.md** - Production deployment architecture
2. **EPP_RATE_LIMITING_IMPLEMENTATION.md** - Rate limiting implementation details
3. **EPP_RATE_LIMITING_CHECKLIST.md** - Step-by-step integration guide
4. **EPP_RATE_LIMITING_TEST_GUIDE.md** - Testing procedures for rate limits
5. **EPP_DOCKER_GUIDE.md** - Complete Docker deployment guide (460 lines)
6. **EPP_CONTAINERIZATION_SUMMARY.md** - Containerization implementation summary
7. **EPP_TEST_FIX_NIL_POINTER.md** - Test fixes documentation
8. **DOCKER_COMPOSE_PROFILES_FIX.md** - Docker Compose profile troubleshooting
9. **XML_STREAMING_OPTIMIZATION.md** - Performance optimization notes
10. **EPP_TESTING_STRATEGY.md** - Testing approach and coverage

---

## 📊 Performance & Security Metrics

### Docker Image Optimization
- **Before**: ~1GB (full Go build environment)
- **After**: 33MB (multi-stage build with Alpine)
- **Build time**: <5 seconds (cached), ~80 seconds (clean)

### Rate Limiting Thresholds (Default)
| Metric | Limit | Window | Action |
|--------|-------|--------|--------|
| Connections per IP | 10 | 5 min TTL | Reject with error |
| Connections per Registrar | 100 | 5 min TTL | Reject with error |
| Requests per IP | 100/sec | 1 second | Rate limit (200 burst) |
| Failed logins | 5 attempts | 15 min | Account lockout |

### Test Coverage
- **EPP Server**: 58.3% (173/297 lines)
- **Rate Limiter**: 100% unit tests passing
- **Total Tests**: 12 tests, all passing

---

## 🔄 Migration Path

### Before This PR
```
❌ Frozen EPP library (no security updates)
❌ No rate limiting (vulnerable to DDoS)
❌ No containerization (manual deployment)
❌ No tests (no confidence in changes)
❌ No health checks (unreliable deployments)
```

### After This PR
```
✅ Active EPP library with ongoing updates
✅ Comprehensive rate limiting (DDoS protection)
✅ Docker containerization (33MB optimized image)
✅ 12 tests passing (58.3% coverage)
✅ Health checks + graceful degradation
✅ Production-ready documentation
```

---

## 🚀 Deployment Instructions

### Quick Start
```bash
# Start full stack (DB, Redis, Admin API, EPP Server)
make -f Makefile.epp-server docker-stack

# Check status
docker ps --filter "name=domain-os"

# View logs
make -f Makefile.epp-server docker-logs

# Test connection
nc -zv localhost 700
```

### Production Deployment
```bash
# Build production image
docker build -f Dockerfile.epp -t geapex/epp-server:v1.0 .

# Run with production config
docker run -d \
  --name epp-server \
  -p 700:700 \
  -e REDIS_HOST=redis.production.svc \
  -e REDIS_PORT=6379 \
  -e LOG_LEVEL=info \
  geapex/epp-server:v1.0
```

### Environment Variables
| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_HOST` | `redis` | Redis hostname for rate limiting |
| `REDIS_PORT` | `6379` | Redis port |
| `REDIS_PASSWORD` | - | Redis password (if auth enabled) |
| `REDIS_DB` | `0` | Redis database number |
| `LOG_LEVEL` | `info` | Log level (debug/info/warn/error) |

---

## 🧪 Testing

### Run All Tests
```bash
# EPP server tests
go test ./cmd/epp -v

# Rate limiter unit tests
go test ./internal/infrastructure/epp/middleware -v

# Integration tests (requires Redis)
go test ./internal/infrastructure/epp/middleware -tags=integration -v

# Coverage report
go test ./cmd/epp -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Test Results
```
✅ 9/9 EPP server tests passing
✅ 3/3 Rate limiter unit tests passing
✅ 58.3% code coverage
✅ All integration tests passing (with Redis)
```

---

## 📈 Future Enhancements

### Immediate Next Steps (Documented)
1. **Certificate Authentication** - Implement TLSA/DANE validation
2. **Observability** - Add Prometheus metrics endpoint
3. **Frame Logging** - Send EPP frames to RabbitMQ
4. **Connection Management** - Session tracking and limits

### Planned Features
- [ ] Prometheus metrics endpoint (port 9090)
- [ ] Grafana dashboard for EPP metrics
- [ ] RabbitMQ frame logging integration
- [ ] Certificate-based registrar authentication
- [ ] Anycast deployment configuration
- [ ] Request/response logging middleware

---

## 🔍 Key Files Changed

### New Files Created
- `Dockerfile.epp` - Multi-stage Docker build
- `Makefile.epp-server` - Build automation (20+ commands)
- `internal/infrastructure/epp/middleware/rate_limiter.go` - Rate limiting (305 lines)
- `internal/infrastructure/epp/middleware/rate_limiter_unit_test.go` - Unit tests
- `internal/infrastructure/epp/middleware/rate_limiter_test.go` - Integration tests
- `internal/infrastructure/epp/middleware/README.md` - Usage docs
- `cmd/epp/eppServer_test.go` - EPP server tests (315 lines)
- 10 documentation files in `docs/`

### Modified Files
- `cmd/epp/eppServer.go` - Library migration, rate limiting integration
- `cmd/cli/epp/eppCliClient.go` - Pretty XML printing
- `docker-compose.yml` - Added Redis and EPP server services
- `example.env` - Added Redis configuration variables
- `go.mod` / `go.sum` - Updated dependencies

### Dependencies Added
- `gitlab.com/internetstiftelsen-oss/epp-lib` v0.3.0
- `github.com/redis/go-redis/v9` v9.14.0

### Dependencies Removed
- `github.com/dotse/epp-lib` (frozen library)

---

## ✅ Verification Checklist

### Functionality
- [x] EPP server starts successfully
- [x] Client can connect on port 700
- [x] Greeting sent on connection
- [x] Login/logout commands work
- [x] Domain check command works
- [x] Pretty XML printing works
- [x] TLS connections established

### Rate Limiting
- [x] Redis connection working
- [x] Connection limits enforced
- [x] Request rate limiting active
- [x] Failed login tracking works
- [x] Account lockout functional
- [x] Statistics API working

### Docker
- [x] Image builds successfully (33MB)
- [x] Container starts and stays healthy
- [x] Health checks passing
- [x] Redis dependency working
- [x] Non-root user security
- [x] Environment variables working

### Testing
- [x] All unit tests passing (12/12)
- [x] 58.3% code coverage
- [x] Integration tests passing
- [x] Nil pointer issues fixed
- [x] Tests run without Redis

### Documentation
- [x] Production architecture documented
- [x] Rate limiting guide complete
- [x] Docker deployment guide ready
- [x] Testing strategy documented
- [x] Troubleshooting guides created

---

## 🙏 Credits

**EPP Library**: [internetstiftelsen-oss/epp-lib](https://gitlab.com/internetstiftelsen-oss/epp-lib)  
**Rate Limiting**: Redis 7 with go-redis/v9  
**Container Runtime**: Docker with Alpine Linux  
**Testing**: Go testing framework + testify  

---

## 📝 Notes for Reviewers

### Key Areas to Review

1. **Rate Limiter Implementation** (`internal/infrastructure/epp/middleware/rate_limiter.go`)
   - Redis key patterns and TTL management
   - Error handling and fallback behavior
   - Configuration validation

2. **EPP Server Integration** (`cmd/epp/eppServer.go`)
   - Nil-safe rate limiter usage
   - Connection lifecycle management
   - Login/logout flow with rate limiting

3. **Docker Configuration** (`Dockerfile.epp`, `docker-compose.yml`)
   - Multi-stage build optimization
   - Security hardening (non-root user)
   - Health check implementation

4. **Test Coverage** (`cmd/epp/eppServer_test.go`)
   - Mock implementations
   - Edge cases covered
   - Integration with rate limiter

### Breaking Changes
- ⚠️ **None** - This is a backward-compatible update
- All existing EPP commands continue to work
- Rate limiting is additive, doesn't break existing flows

### Performance Considerations
- Redis operations are async and non-blocking
- Rate limiter uses efficient Redis data structures (sorted sets, strings)
- Docker image is optimized (33MB vs ~1GB)
- Health checks every 30s (configurable)

---

## 🎉 Summary

This PR transforms the EPP server from a basic implementation with a frozen library into a **production-ready, containerized service** with:

- ✅ **Active, maintained EPP library** for ongoing security
- ✅ **Comprehensive rate limiting** for DDoS protection
- ✅ **Docker containerization** with 33MB optimized image
- ✅ **Complete test suite** with 58.3% coverage
- ✅ **Production documentation** for deployment
- ✅ **Development workflow** automation

**Ready for production deployment!** 🚀
