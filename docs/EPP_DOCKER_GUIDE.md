# Running EPP Server in Docker

This guide explains how to build and run the EPP server as a containerized service.

## Quick Start

### 1. Build the EPP Server Image

```bash
# Using the Makefile
make -f Makefile.epp-server docker-build

# Or manually
docker build -f Dockerfile.epp -t gprins/domain-os-epp:latest .
```

### 2. Start the EPP Stack (Redis + EPP Server)

```bash
# Using the Makefile
make -f Makefile.epp-server docker-stack

# Or using docker-compose directly
doppler run -- docker compose --profile essential up -d redis epp-server
```

### 3. Verify It's Running

```bash
# Check status
make -f Makefile.epp-server status

# Test connection
make -f Makefile.epp-server test-connect

# View logs
make -f Makefile.epp-server docker-logs
```

## Architecture

```
┌─────────────────────────────────────────────┐
│          Docker Compose Stack               │
│                                             │
│  ┌──────────────┐      ┌─────────────────┐ │
│  │              │      │                 │ │
│  │  Redis       │◄─────┤  EPP Server     │ │
│  │  :6379       │      │  :700           │ │
│  │              │      │                 │ │
│  └──────────────┘      └─────────────────┘ │
│                                             │
└─────────────────────────────────────────────┘
                     │
                     ▼
            EPP Clients (port 700)
```

## Docker Image Details

### Dockerfile.epp Structure

**Stage 1: Builder**
- Based on `golang:1.23-alpine`
- Compiles static binary with no CGO dependencies
- Optimized with `-ldflags='-w -s'` for smaller size

**Stage 2: Runtime**
- Based on `alpine:3.19` (minimal footprint)
- Includes only necessary runtime tools (ca-certificates, netcat)
- Runs as non-root user (`epp:epp`)
- Final image size: ~15-20MB

### Security Features

- ✅ Non-root user (`epp:epp` with UID/GID 1000)
- ✅ Minimal attack surface (Alpine base, no unnecessary packages)
- ✅ Static binary (no dynamic library dependencies)
- ✅ Health checks enabled
- ✅ Read-only filesystem compatible (optional)

## Environment Variables

The EPP server accepts the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `REDIS_HOST` | `localhost` | Redis server hostname |
| `REDIS_PORT` | `6379` | Redis server port |
| `REDIS_PASSWORD` | `` | Redis password (if required) |
| `REDIS_DB` | `0` | Redis database number |
| `LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`). **Default `info` suppresses health check DEBUG messages**. |

**Note on Health Checks**: The Docker health check opens port 700 every 30 seconds without sending EPP data, causing harmless EOF errors. These are logged at DEBUG level only. In production, keep `LOG_LEVEL=info` to suppress this noise. Set to `debug` only when troubleshooting.

## Docker Compose Configuration

The EPP server is configured in `docker-compose.yml`:

```yaml
epp-server:
  build:
    context: .
    dockerfile: Dockerfile.epp
  image: gprins/domain-os-epp:${BRANCH:-latest}
  restart: unless-stopped
  profiles: [essential, full]
  depends_on:
    redis:
      condition: service_healthy
  ports:
    - 700:700
  networks:
    - dos
  environment:
    - REDIS_HOST=redis
    - REDIS_PORT=6379
    - REDIS_PASSWORD=${REDIS_PASSWORD:-}
    - REDIS_DB=${REDIS_DB:-0}
```

## Makefile Commands

### Building & Running

```bash
# Build Docker image
make -f Makefile.epp-server docker-build

# Run EPP server
make -f Makefile.epp-server docker-run

# Run full stack (Redis + EPP)
make -f Makefile.epp-server docker-stack

# Stop EPP server
make -f Makefile.epp-server docker-stop

# Stop full stack
make -f Makefile.epp-server docker-stack-down
```

### Development

```bash
# Rebuild and restart (for development)
make -f Makefile.epp-server docker-dev

# View logs
make -f Makefile.epp-server docker-logs

# Open shell in container
make -f Makefile.epp-server docker-shell
```

### Testing & Debugging

```bash
# Check status
make -f Makefile.epp-server status

# Test connection
make -f Makefile.epp-server test-connect

# Monitor Redis
make -f Makefile.epp-server redis-monitor

# Show rate limiting keys
make -f Makefile.epp-server redis-keys

# Clear rate limiting data
make -f Makefile.epp-server redis-clear
```

## Health Checks

The EPP server includes health checks at multiple levels:

### Docker Container Health Check
```yaml
healthcheck:
  test: ["CMD", "nc", "-z", "localhost", "700"]
  interval: 30s
  timeout: 5s
  retries: 3
  start_period: 10s
```

### Check Health Status
```bash
# Using docker-compose
docker compose ps epp-server

# Using docker inspect
docker inspect --format='{{.State.Health.Status}}' domain-os-epp-server-1
```

## Networking

### Ports

| Port | Protocol | Description |
|------|----------|-------------|
| 700  | TCP      | EPP protocol (TLS) |

### Docker Network

The EPP server runs on the `dos` Docker network and can communicate with:
- **Redis** (`redis:6379`) - For rate limiting and session management
- **PostgreSQL** (`db:5432`) - For domain registry (future)
- **RabbitMQ** (`msg-broker:5672`) - For audit logging (future)

## Connecting to EPP Server

### From Host Machine

```bash
# Using the EPP client
go run cmd/cli/epp/eppCliClient.go

# Using openssl
openssl s_client -connect localhost:700
```

### From Another Container

```bash
# Start a test container in the same network
docker run --rm -it --network domain-os_dos alpine sh

# Install openssl
apk add openssl

# Connect to EPP server
openssl s_client -connect epp-server:700
```

## Logs

### View Live Logs

```bash
# Using Makefile
make -f Makefile.epp-server docker-logs

# Using docker-compose
docker compose logs -f epp-server

# With timestamps
docker compose logs -f --timestamps epp-server
```

### Log Format

The EPP server uses structured JSON logging:

```json
{
  "time": "2025-10-07T10:30:45Z",
  "level": "INFO",
  "msg": "Connection established",
  "connection_id": "conn-1728295845123456789",
  "client_ip": "172.18.0.5"
}
```

## Persistence

### Redis Data

Redis data is persisted in a Docker volume:

```bash
# List volumes
docker volume ls | grep redis

# Inspect Redis volume
docker volume inspect domain-os_redis_data

# Backup Redis data
docker compose exec redis redis-cli SAVE
docker cp domain-os-redis-1:/data/dump.rdb ./backup-redis.rdb
```

### Certificates (Future)

Future certificate management will use:
```bash
# Certificate directory (not yet implemented)
/etc/epp/certs/
  ├── server.crt
  ├── server.key
  └── ca.crt
```

## Troubleshooting

### EPP Server Won't Start

```bash
# Check logs
docker compose logs epp-server

# Check Redis is running
docker compose ps redis

# Check if port 700 is already in use
lsof -i :700

# Restart services
docker compose restart redis epp-server
```

### Cannot Connect to EPP Server

```bash
# Check if container is running
docker compose ps epp-server

# Check health status
docker inspect domain-os-epp-server-1 --format='{{.State.Health.Status}}'

# Test from inside container
docker compose exec epp-server nc -zv localhost 700

# Check firewall/network
docker compose exec epp-server netstat -tuln | grep 700
```

### Rate Limiting Issues

```bash
# Check Redis connection from EPP server
docker compose exec epp-server nc -zv redis 6379

# Monitor Redis commands
make -f Makefile.epp-server redis-monitor

# Check rate limiting keys
make -f Makefile.epp-server redis-keys

# Clear rate limiting data
make -f Makefile.epp-server redis-clear
```

### High Memory Usage

```bash
# Check container stats
docker stats domain-os-epp-server-1

# Check Redis memory
docker compose exec redis redis-cli INFO memory

# Restart with memory limit
docker compose up -d epp-server --memory=256m
```

## Production Deployment

### Build for Production

```bash
# Set branch name
export BRANCH=production

# Build optimized image
docker build -f Dockerfile.epp \
  --build-arg GIT_SHA=$(git rev-parse HEAD) \
  -t gprins/domain-os-epp:${BRANCH} \
  .

# Tag for registry
docker tag gprins/domain-os-epp:${BRANCH} registry.example.com/epp-server:${BRANCH}

# Push to registry
docker push registry.example.com/epp-server:${BRANCH}
```

### Production Configuration

```yaml
# docker-compose.prod.yml
epp-server:
  image: gprins/domain-os-epp:production
  restart: always
  deploy:
    replicas: 3
    resources:
      limits:
        cpus: '1'
        memory: 512M
      reservations:
        cpus: '0.5'
        memory: 256M
  environment:
    - REDIS_HOST=redis-cluster
    - LOG_LEVEL=info
```

### Security Hardening

1. **Read-only filesystem**:
   ```yaml
   read_only: true
   tmpfs:
     - /tmp
   ```

2. **Drop capabilities**:
   ```yaml
   cap_drop:
     - ALL
   cap_add:
     - NET_BIND_SERVICE
   ```

3. **Run with security options**:
   ```yaml
   security_opt:
     - no-new-privileges:true
   ```

## Monitoring

### Prometheus Metrics (Future)

```yaml
# Add metrics port
ports:
  - 700:700
  - 9090:9090  # Prometheus metrics

# Add metrics endpoint
EXPOSE 9090
```

### Health Monitoring

```bash
# Continuous health check
watch -n 5 'docker compose ps epp-server'

# Export health status to Prometheus
# (requires custom exporter)
```

## Next Steps

1. ✅ **Integrate with existing services** - EPP server now runs in Docker
2. 🔄 **Add certificate management** - Use Vault for TLS certificates
3. 🔄 **Enable Prometheus metrics** - Add /metrics endpoint
4. 🔄 **Set up log aggregation** - Forward logs to ELK/Loki
5. 🔄 **Configure anycast** - Deploy to multiple regions

## References

- [EPP Server Documentation](../docs/EPP_TESTING_README.md)
- [Rate Limiting Guide](../docs/EPP_RATE_LIMITING_IMPLEMENTATION.md)
- [Production Architecture](../docs/EPP_PRODUCTION_ARCHITECTURE.md)
- [Docker Compose Documentation](https://docs.docker.com/compose/)
