# EPP Rate Limiting - Implementation Checklist

## ✅ Completed Tasks

### Infrastructure
- [x] Added Redis 7 container to docker-compose.yml
- [x] Configured Redis with persistence (AOF)
- [x] Set memory limits and eviction policy
- [x] Added Redis to `essential` profile
- [x] Updated example.env with Redis configuration

### Core Implementation
- [x] Created rate_limiter.go with all core functionality
- [x] Implemented connection limiting (per-IP and per-registrar)
- [x] Implemented request rate limiting (token bucket algorithm)
- [x] Implemented failed login tracking with lockout
- [x] Added statistics API for monitoring
- [x] Defined custom error types

### Testing
- [x] Created unit tests (no Redis required)
- [x] Created integration tests (requires Redis)
- [x] All unit tests passing (3/3)
- [x] Separated integration tests with build tags

### Dependencies
- [x] Added github.com/redis/go-redis/v9 to go.mod
- [x] Updated vendor directory with go mod vendor
- [x] Verified EPP server compiles with new dependencies

### Documentation
- [x] Created comprehensive middleware README
- [x] Updated EPP Production Architecture (RabbitMQ instead of Kafka)
- [x] Created implementation summary document
- [x] Added quick start guides
- [x] Added troubleshooting guides

## 🔄 Next Steps (To Be Implemented)

### Phase 1: EPP Server Integration (Priority: HIGH)
- [ ] Initialize Redis client in main()
- [ ] Create RateLimiter instance with config from env
- [ ] Add connection tracking to logConnection()
- [ ] Enhance respondToLoginCommand() with failed login tracking
- [ ] Add registrar ID extraction from client certificate
- [ ] Implement graceful connection rejection when limits exceeded
- [ ] Add request rate limiting to command handlers

### Phase 2: Connection Lifecycle (Priority: HIGH)
- [ ] Track connection count on accept
- [ ] Increment counter after successful TLS handshake
- [ ] Decrement counter on connection close
- [ ] Handle cleanup on server shutdown
- [ ] Add metrics for connection tracking

### Phase 3: Authentication Enhancement (Priority: MEDIUM)
- [ ] Implement CertificateValidator
- [ ] Add TLSA/DANE validation
- [ ] Extract registrar ID from certificate
- [ ] Integrate with login handler
- [ ] Add certificate pinning support

### Phase 4: Session Management (Priority: MEDIUM)
- [ ] Create SessionManager with Redis backend
- [ ] Implement session creation/retrieval
- [ ] Add session timeout handling
- [ ] Track authenticated state in session
- [ ] Add session statistics

### Phase 5: Observability (Priority: MEDIUM)
- [ ] Add Prometheus metrics for rate limiter
- [ ] Implement frame logging to RabbitMQ
- [ ] Create Grafana dashboards
- [ ] Add distributed tracing
- [ ] Set up alerting rules

### Phase 6: Production Hardening (Priority: LOW)
- [ ] Add circuit breaker for Redis
- [ ] Implement Redis connection pooling
- [ ] Add retry logic for Redis operations
- [ ] Create health check endpoints
- [ ] Add graceful degradation when Redis unavailable

## 📋 Implementation Guide

### Step 1: Initialize Redis Client

Add to `cmd/epp/eppServer.go`:

```go
import (
    "github.com/redis/go-redis/v9"
    "github.com/onasunnymorning/domain-os/internal/infrastructure/epp/middleware"
)

func main() {
    // ... existing logger setup ...
    
    // Initialize Redis client
    redisClient := redis.NewClient(&redis.Options{
        Addr:     os.Getenv("REDIS_HOST") + ":" + os.Getenv("REDIS_PORT"),
        Password: os.Getenv("REDIS_PASSWORD"),
        DB:       0,
    })
    
    // Test connection
    ctx := context.Background()
    if err := redisClient.Ping(ctx).Err(); err != nil {
        logger.Error("Failed to connect to Redis", "error", err)
        // Decide: fail fast or continue without rate limiting?
    }
    
    // Create rate limiter
    rateLimiter := middleware.NewRateLimiter(redisClient, nil, logger)
    
    // ... rest of server setup ...
}
```

### Step 2: Update logConnection()

```go
func logConnection(ctx context.Context, conn *tls.Conn) (context.Context, error) {
    clientIP := conn.RemoteAddr().String()
    
    // Check connection limit
    if err := rateLimiter.CheckConnectionLimit(ctx, clientIP, ""); err != nil {
        logger.Warn("Connection rejected", "ip", clientIP, "error", err)
        return nil, err // Connection will be rejected
    }
    
    // Increment connection counter
    if err := rateLimiter.IncrementConnection(ctx, clientIP, ""); err != nil {
        logger.Error("Failed to track connection", "error", err)
    }
    
    // Add connection ID to context
    connectionID := generateConnectionID()
    ctx = context.WithValue(ctx, connectionIDKey, connectionID)
    
    logger.Info("Connection established", 
        "connection_id", connectionID,
        "client_ip", clientIP)
    
    return ctx, nil
}
```

### Step 3: Add Connection Cleanup

```go
// Need to track active connections to decrement on close
type connectionTracker struct {
    mu          sync.Mutex
    connections map[string]*connectionInfo
}

type connectionInfo struct {
    IP          string
    RegistrarID string
    ConnectedAt time.Time
}

// On connection close (need to add hook in server)
func (ct *connectionTracker) onClose(connectionID string) {
    ct.mu.Lock()
    info := ct.connections[connectionID]
    delete(ct.connections, connectionID)
    ct.mu.Unlock()
    
    if info != nil {
        ctx := context.Background()
        if err := rateLimiter.DecrementConnection(ctx, info.IP, info.RegistrarID); err != nil {
            logger.Error("Failed to decrement connection", "error", err)
        }
    }
}
```

### Step 4: Enhance Login Handler

```go
func respondToLoginCommand(ctx context.Context, rw epp.Writer, doc *etree.Document) {
    // Extract login credentials
    clID := doc.FindElement("//clID")
    username := ""
    if clID != nil {
        username = clID.Text()
    }
    
    clientIP := getClientIP(ctx) // Extract from context
    
    // Check if account is locked
    locked, err := rateLimiter.IsAccountLocked(ctx, username)
    if err != nil {
        logger.Error("Failed to check account lock", "error", err)
    }
    if locked {
        sendLoginError(rw, "Account locked due to failed login attempts")
        return
    }
    
    // Validate credentials (replace with real validation)
    valid := validateCredentials(username, password)
    
    if !valid {
        // Record failed login
        if err := rateLimiter.RecordFailedLogin(ctx, username, clientIP); err != nil {
            logger.Error("Failed to record failed login", "error", err)
        }
        sendLoginError(rw, "Authentication failed")
        return
    }
    
    // Clear failed logins on success
    if err := rateLimiter.ClearFailedLogins(ctx, username, clientIP); err != nil {
        logger.Error("Failed to clear failed logins", "error", err)
    }
    
    // Send successful login response
    sendLoginSuccess(rw)
}
```

## 🧪 Testing Checklist

### Unit Tests
- [x] Run: `go test ./internal/infrastructure/epp/middleware -v`
- [x] Verify all tests pass
- [x] Check code coverage

### Integration Tests
- [ ] Start Redis: `doppler run -- docker compose --profile essential up -d redis`
- [ ] Run: `go test -tags=integration ./internal/infrastructure/epp/middleware -v`
- [ ] Verify connection limiting works
- [ ] Verify failed login tracking works
- [ ] Check Redis keys are created correctly

### Manual Testing
- [ ] Start EPP server with rate limiting enabled
- [ ] Test connection limit by opening multiple connections from same IP
- [ ] Test failed login lockout (5 failed attempts)
- [ ] Test successful login clears failed attempts
- [ ] Monitor Redis keys during testing
- [ ] Check logs for rate limit events

### Load Testing
- [ ] Use tool like `wrk` or `hey` to generate load
- [ ] Verify rate limits are enforced under load
- [ ] Check Redis performance under load
- [ ] Monitor memory usage
- [ ] Test recovery after rate limit period expires

## 📊 Monitoring Checklist

### Redis Monitoring
- [ ] Monitor Redis memory usage
- [ ] Track key count growth
- [ ] Monitor connection count to Redis
- [ ] Set up alerts for Redis failures
- [ ] Track Redis command latency

### Rate Limiter Metrics
- [ ] Track connections per IP (histogram)
- [ ] Track connections per registrar (histogram)
- [ ] Track rate limit hits (counter)
- [ ] Track failed login attempts (counter)
- [ ] Track account lockouts (counter)

### Dashboards
- [ ] Create Grafana dashboard for connection metrics
- [ ] Create dashboard for rate limit violations
- [ ] Create dashboard for failed login attempts
- [ ] Set up alerts for unusual patterns

## 🚀 Deployment Checklist

### Development
- [x] Code implemented and tested locally
- [x] Documentation complete
- [ ] Code review completed
- [ ] Integration tests passing

### Staging
- [ ] Deploy to staging environment
- [ ] Run integration tests against staging
- [ ] Performance testing
- [ ] Security review
- [ ] Load testing

### Production
- [ ] Deploy Redis in HA configuration (Sentinel/Cluster)
- [ ] Configure monitoring and alerting
- [ ] Set up backup/restore for Redis
- [ ] Document runbooks for incidents
- [ ] Gradual rollout (canary deployment)
- [ ] Monitor for 24 hours post-deployment

## 📝 Configuration Values

### Recommended Starting Values

**Development**:
```go
MaxConnPerIP:        10
MaxConnPerRegistrar: 100
RequestsPerSecond:   100
BurstSize:           200
MaxFailedLogins:     5
LockoutDuration:     15 * time.Minute
```

**Production** (adjust based on monitoring):
```go
MaxConnPerIP:        5    // Stricter
MaxConnPerRegistrar: 200  // Higher for legitimate registrars
RequestsPerSecond:   50   // Conservative
BurstSize:           100  // Conservative
MaxFailedLogins:     3    // Stricter
LockoutDuration:     30 * time.Minute  // Longer
```

## 🔍 Key Decisions Made

1. **Redis over in-memory**: Chose Redis for distributed rate limiting across multiple servers
2. **RabbitMQ over Kafka**: Used existing RabbitMQ infrastructure instead of adding Kafka
3. **Separate integration tests**: Used build tags to separate Redis-dependent tests
4. **Default config**: Provided sensible defaults that can be overridden
5. **Error types**: Created specific error types for different rate limit scenarios

## 📚 References

- [EPP Production Architecture](./EPP_PRODUCTION_ARCHITECTURE.md)
- [Rate Limiter README](../internal/infrastructure/epp/middleware/README.md)
- [Implementation Summary](./EPP_RATE_LIMITING_IMPLEMENTATION.md)
- [Redis Go Client Docs](https://redis.uptrace.dev/)
- [Token Bucket Algorithm](https://en.wikipedia.org/wiki/Token_bucket)
