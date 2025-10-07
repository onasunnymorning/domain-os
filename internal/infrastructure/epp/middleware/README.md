# EPP Rate Limiter

Redis-based rate limiting and connection tracking for the EPP server to prevent DDoS attacks and brute force attempts.

## Features

- ✅ **Connection Limiting**: Per-IP and per-registrar connection limits
- ✅ **Request Rate Limiting**: Token bucket algorithm for request throttling  
- ✅ **Failed Login Tracking**: Automatic account lockout after failed attempts
- ✅ **Distributed**: Redis-backed for multi-server deployments
- ✅ **Configurable**: All limits and timeouts are configurable

## Quick Start

### 1. Start Redis

Using Docker Compose (with Doppler):
```bash
doppler run -- docker compose --profile essential up -d redis
```

Or start Redis manually:
```bash
docker run -d -p 6379:6379 redis:7-alpine
```

### 2. Initialize Rate Limiter

```go
import (
    "github.com/redis/go-redis/v9"
    "github.com/onasunnymorning/domain-os/internal/infrastructure/epp/middleware"
)

// Create Redis client
redisClient := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "", // no password by default
    DB:       0,  // use default DB
})

// Create rate limiter with default config
rateLimiter := middleware.NewRateLimiter(redisClient, nil, logger)

// Or with custom config
config := &middleware.RateLimitConfig{
    MaxConnPerIP:        10,
    MaxConnPerRegistrar: 100,
    ConnTTL:             5 * time.Minute,
    RequestsPerSecond:   100,
    BurstSize:           200,
    RequestWindow:       time.Second,
    MaxFailedLogins:     5,
    LockoutDuration:     15 * time.Minute,
}
rateLimiter := middleware.NewRateLimiter(redisClient, config, logger)
```

### 3. Use in EPP Server

```go
// In connection handler (before accepting connection)
func handleConnection(conn *tls.Conn, rateLimiter *middleware.RateLimiter) {
    ctx := context.Background()
    clientIP := conn.RemoteAddr().String()
    
    // Check connection limit
    if err := rateLimiter.CheckConnectionLimit(ctx, clientIP, ""); err != nil {
        log.Error("Connection limit exceeded", "ip", clientIP, "error", err)
        conn.Close()
        return
    }
    
    // Increment connection counter
    if err := rateLimiter.IncrementConnection(ctx, clientIP, ""); err != nil {
        log.Error("Failed to track connection", "error", err)
    }
    
    // Ensure decrement on disconnect
    defer func() {
        if err := rateLimiter.DecrementConnection(ctx, clientIP, ""); err != nil {
            log.Error("Failed to decrement connection", "error", err)
        }
    }()
    
    // Handle EPP commands...
}
```

### 4. Track Failed Logins

```go
// In login command handler
func handleLogin(ctx context.Context, username, password, clientIP string) error {
    // Check if account is locked
    locked, err := rateLimiter.IsAccountLocked(ctx, username)
    if err != nil {
        return err
    }
    if locked {
        return middleware.ErrAccountLocked
    }
    
    // Validate credentials
    if !validateCredentials(username, password) {
        // Record failed login
        if err := rateLimiter.RecordFailedLogin(ctx, username, clientIP); err != nil {
            return err
        }
        return errors.New("invalid credentials")
    }
    
    // Clear failed login attempts on successful login
    rateLimiter.ClearFailedLogins(ctx, username, clientIP)
    
    return nil
}
```

### 5. Check Request Rate

```go
// In command handler
func handleCommand(ctx context.Context, registrarID string) error {
    // Check request rate limit
    if err := rateLimiter.CheckRequestRate(ctx, registrarID); err != nil {
        return err
    }
    
    // Process command...
    return nil
}
```

## Configuration

### Default Configuration

```go
config := middleware.DefaultRateLimitConfig()
// MaxConnPerIP:        10 connections per IP
// MaxConnPerRegistrar: 100 connections per registrar
// ConnTTL:             5 minutes
// RequestsPerSecond:   100 requests/second per registrar
// BurstSize:           200 burst requests
// RequestWindow:       1 second
// MaxFailedLogins:     5 failed attempts
// LockoutDuration:     15 minutes
```

### Custom Configuration

Adjust limits based on your needs:

```go
config := &middleware.RateLimitConfig{
    // Connection limits
    MaxConnPerIP:        5,    // Stricter IP limit
    MaxConnPerRegistrar: 200,  // More registrar connections
    ConnTTL:             10 * time.Minute,
    
    // Request rate limits
    RequestsPerSecond:   50,   // Lower rate
    BurstSize:           100,  // Lower burst
    RequestWindow:       time.Second,
    
    // Failed login limits
    MaxFailedLogins:     3,    // Stricter lockout
    LockoutDuration:     30 * time.Minute,
}
```

## Testing

### Unit Tests

Run unit tests (no Redis required):
```bash
go test ./internal/infrastructure/epp/middleware -v
```

### Integration Tests

Run integration tests (requires Redis):
```bash
# Start Redis first
doppler run -- docker compose --profile essential up -d redis

# Run integration tests
go test -tags=integration ./internal/infrastructure/epp/middleware -v
```

## Redis Keys

The rate limiter uses the following Redis key patterns:

- `conn:ip:<ip_address>` - Connection count for IP
- `conn:reg:<registrar_id>` - Connection count for registrar
- `rate:req:<registrar_id>` - Request rate counter for registrar
- `failed:login:<username>:<ip>` - Failed login attempt counter
- `locked:<username>` - Account lock flag

All keys have TTL set automatically.

## Monitoring

Get rate limiter statistics:

```go
stats, err := rateLimiter.GetStats(ctx)
if err != nil {
    log.Error("Failed to get stats", "error", err)
}

fmt.Printf("Total IP connections: %d\n", stats["total_ip_connections"])
fmt.Printf("Total registrar connections: %d\n", stats["total_registrar_connections"])
```

## Error Handling

The rate limiter returns specific errors:

- `middleware.ErrTooManyConnections` - IP connection limit exceeded
- `middleware.ErrTooManyRegistrarConnections` - Registrar connection limit exceeded
- `middleware.ErrRateLimitExceeded` - Request rate limit exceeded
- `middleware.ErrAccountLocked` - Account locked due to failed logins

```go
if err := rateLimiter.CheckConnectionLimit(ctx, clientIP, registrarID); err != nil {
    switch err {
    case middleware.ErrTooManyConnections:
        log.Warn("IP blocked", "ip", clientIP)
        // Send error response to client
    case middleware.ErrTooManyRegistrarConnections:
        log.Warn("Registrar limit reached", "registrar", registrarID)
        // Send error response to client
    default:
        log.Error("Unexpected error", "error", err)
    }
}
```

## Production Deployment

### Redis Persistence

Configure Redis with persistence for production:

```yaml
# docker-compose.yml
redis:
  image: redis:7-alpine
  command: >
    redis-server
    --appendonly yes
    --appendfsync everysec
    --maxmemory 256mb
    --maxmemory-policy allkeys-lru
  volumes:
    - redis_data:/data
```

### High Availability

For HA deployments, use Redis Cluster or Redis Sentinel:

```go
// Redis Cluster
redisClient := redis.NewClusterClient(&redis.ClusterOptions{
    Addrs: []string{
        "redis-1:6379",
        "redis-2:6379",
        "redis-3:6379",
    },
})
```

### Monitoring

Export Redis metrics to Prometheus:

```bash
# Add Redis exporter to docker-compose.yml
redis-exporter:
  image: oliver006/redis_exporter
  environment:
    - REDIS_ADDR=redis:6379
  ports:
    - 9121:9121
```

## Best Practices

1. **Set appropriate limits**: Start conservative and adjust based on metrics
2. **Monitor Redis**: Watch memory usage and connection counts
3. **Use separate Redis DB**: Use DB 0 for rate limiting, separate DBs for other uses
4. **Handle errors gracefully**: Always handle rate limit errors and provide feedback
5. **Log rate limit hits**: Track who's being rate limited for security analysis
6. **Regular cleanup**: Redis automatically expires keys, but monitor memory usage

## Troubleshooting

### Redis Connection Issues

```go
// Test Redis connection
ctx := context.Background()
if err := redisClient.Ping(ctx).Err(); err != nil {
    log.Fatal("Cannot connect to Redis", "error", err)
}
```

### Rate Limiter Not Working

1. Check Redis is running: `docker ps | grep redis`
2. Check Redis keys: `redis-cli keys "conn:*"`
3. Check TTL: `redis-cli ttl "conn:ip:192.168.1.1"`
4. Enable debug logging in rate limiter

### High Memory Usage

1. Check key count: `redis-cli dbsize`
2. Check memory: `redis-cli info memory`
3. Reduce TTL values
4. Enable maxmemory policy: `--maxmemory-policy allkeys-lru`

## Next Steps

See the [EPP Production Architecture](../../../docs/EPP_PRODUCTION_ARCHITECTURE.md) for:
- Certificate-based authentication
- Session management
- Audit logging
- Global anycast deployment
