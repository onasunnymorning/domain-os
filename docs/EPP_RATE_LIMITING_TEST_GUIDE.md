# EPP Rate Limiting Test Guide

## Setup

### 1. Start Redis Container

```bash
docker compose --profile essential up -d redis
```

Verify Redis is running:
```bash
docker compose ps redis
docker compose logs redis
```

### 2. Test Redis Connection

```bash
docker exec -it domain-os-redis-1 redis-cli ping
```

Expected output: `PONG`

### 3. Set Environment Variables

```bash
export REDIS_HOST=localhost
export REDIS_PORT=6379
export REDIS_PASSWORD=
export REDIS_DB=0
```

Or create a `.env` file:
```
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
```

## Running the EPP Server

```bash
go run cmd/epp/eppServer.go
```

Expected output:
```
{"time":"...","level":"INFO","msg":"Connected to Redis","addr":"localhost:6379"}
Listening on port 700
```

## Testing Rate Limiting

### Test 1: Connection Tracking

Open multiple terminal windows and connect using the EPP client:

```bash
# Terminal 1
go run cmd/cli/epp/eppCliClient.go

# Terminal 2
go run cmd/cli/epp/eppCliClient.go

# Terminal 3 (and so on...)
go run cmd/cli/epp/eppCliClient.go
```

After 10 connections from the same IP, you should see:
```
Connection from 127.0.0.1 rejected: too many connections from this IP
```

### Test 2: Monitor Connection Counts in Redis

In another terminal:

```bash
docker exec -it domain-os-redis-1 redis-cli

# List all connection keys
KEYS conn:*

# Get connection count for an IP
GET conn:ip:127.0.0.1

# Get connection count for a registrar
GET conn:reg:testuser
```

### Test 3: Failed Login Tracking

Using the EPP client, attempt to login multiple times with wrong credentials.

After 5 failed attempts, the account should be locked for 15 minutes.

Check in Redis:
```bash
# Check failed login count
GET failed:login:testuser:127.0.0.1

# Check if account is locked
GET locked:testuser
```

### Test 4: Connection Cleanup

1. Start an EPP client connection
2. Check Redis: `GET conn:ip:127.0.0.1` (should show 1)
3. Close the EPP client
4. Wait a few seconds
5. Check Redis again: `GET conn:ip:127.0.0.1` (should show 0 or key deleted)

## Rate Limiter Statistics

You can query rate limiter statistics programmatically:

```go
stats, err := rateLimiter.GetStats(ctx)
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Stats: %+v\n", stats)
```

## Testing with Multiple IPs

To test per-IP limits with different IPs (requires running in Docker or VM):

```bash
# From different machines or using Docker networks
docker run --rm -it --network domain-os_dos \
  golang:1.23 go run /path/to/eppCliClient.go
```

## Load Testing

### Simple Bash Script

```bash
#!/bin/bash
# test_rate_limit.sh

for i in {1..15}; do
    echo "Connection attempt $i"
    timeout 2 go run cmd/cli/epp/eppCliClient.go &
    sleep 0.1
done

wait
echo "All connection attempts completed"
```

Run:
```bash
chmod +x test_rate_limit.sh
./test_rate_limit.sh
```

Expected: First 10 connections succeed, next 5 are rejected.

## Monitoring

### Watch Connection Counts

```bash
watch -n 1 'docker exec domain-os-redis-1 redis-cli KEYS "conn:*" | wc -l'
```

### Monitor Redis Commands

```bash
docker exec -it domain-os-redis-1 redis-cli MONITOR
```

## Cleanup

### Clear All Rate Limiting Data

```bash
docker exec -it domain-os-redis-1 redis-cli FLUSHDB
```

### Clear Specific Keys

```bash
# Clear all connection counters
docker exec -it domain-os-redis-1 redis-cli --scan --pattern "conn:*" | \
  xargs docker exec -i domain-os-redis-1 redis-cli DEL

# Clear all failed login counters
docker exec -it domain-os-redis-1 redis-cli --scan --pattern "failed:*" | \
  xargs docker exec -i domain-os-redis-1 redis-cli DEL

# Unlock all accounts
docker exec -it domain-os-redis-1 redis-cli --scan --pattern "locked:*" | \
  xargs docker exec -i domain-os-redis-1 redis-cli DEL
```

## Configuration Tuning

Edit the rate limiter config in `cmd/epp/eppServer.go`:

```go
rateLimitConfig := &middleware.RateLimitConfig{
    MaxConnPerIP:        10,   // Increase/decrease connection limit per IP
    MaxConnPerRegistrar: 100,  // Increase/decrease connection limit per registrar
    ConnTTL:             5 * time.Minute,  // How long to track connections
    RequestsPerSecond:   100,  // Requests per second per registrar
    BurstSize:           200,  // Burst capacity
    RequestWindow:       time.Second,  // Rate limit window
    MaxFailedLogins:     5,    // Failed attempts before lockout
    LockoutDuration:     15 * time.Minute,  // Lockout duration
}
```

## Expected Behavior

1. **Connection Limits**: Max 10 connections per IP, 100 per registrar
2. **Failed Login**: 5 failed attempts → 15 minute lockout
3. **Automatic Cleanup**: Connections decremented when client disconnects
4. **TTL**: Connection counts expire after 5 minutes of inactivity
5. **Rate Limits**: 100 requests/sec with burst of 200

## Troubleshooting

### Redis Connection Failed

```bash
# Check if Redis is running
docker compose ps redis

# Check Redis logs
docker compose logs redis

# Restart Redis
docker compose restart redis
```

### Connection Not Being Tracked

- Check if `logConnection` is being called
- Verify Redis connection in server logs
- Check for errors in rate limiter logs

### Connections Not Cleaned Up

- Ensure context is properly cancelled when connection closes
- Check goroutine for cleanup is running
- Verify Redis connectivity during cleanup

## Next Steps

1. Add unit tests for rate limiting integration
2. Add metrics/monitoring for rate limiting events
3. Implement certificate-based authentication
4. Add request-level rate limiting
5. Implement session management
