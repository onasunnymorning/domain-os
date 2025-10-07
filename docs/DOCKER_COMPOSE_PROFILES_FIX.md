# Docker Compose Profile Issue - Resolution

## Problem

**Question**: "Why isn't admin-api and db containers running in the compose project?"

**Observed Behavior**:
```bash
$ docker ps
NAMES                    STATUS                   PORTS
domain-os-epp-server-1   Up 2 minutes (healthy)   0.0.0.0:700->700/tcp
domain-os-redis-1        Up 3 minutes (healthy)   0.0.0.0:6379->6379/tcp
```

Missing: `domain-os-db-1` and `domain-os-admin-api-1`

## Root Cause

The Makefile `docker-stack` target was running:
```bash
docker compose --profile essential up -d redis epp-server
```

When you specify **specific service names** after the `docker compose up` command, Docker Compose **only starts those services** (and their dependencies), even if other services are in the same profile.

## Docker Compose Profile Behavior

### ✅ Correct - Starts ALL services in the profile:
```bash
docker compose --profile essential up -d
```

### ❌ Incorrect - Only starts specified services:
```bash
docker compose --profile essential up -d redis epp-server
```
This ignores `db` and `admin-api` even though they have `profiles: [essential]`.

## Solution

### Updated Makefile Targets

#### 1. **Full Stack** (default) - Starts all essential services:
```makefile
docker-stack:
	@echo "Building EPP server image..."
	docker build -f Dockerfile.epp -t geapex/epp-server:latest .
	@echo "Starting full EPP stack (all essential services)..."
	BRANCH=latest doppler run -- docker compose --profile essential up -d
	@echo "✓ EPP stack is running"
	@echo "  Database: localhost:5432"
	@echo "  Redis: localhost:6379"
	@echo "  EPP Server: localhost:700"
	@echo "  Admin API: localhost:8080"
```

**Usage**: `make -f Makefile.epp-server docker-stack`

**Starts**:
- PostgreSQL database (port 5432)
- Redis (port 6379)
- EPP Server (port 700)
- Admin API (port 8080)

#### 2. **Minimal Stack** (new) - Just Redis + EPP Server:
```makefile
docker-stack-minimal:
	@echo "Building EPP server image..."
	docker build -f Dockerfile.epp -t geapex/epp-server:latest .
	@echo "Starting minimal EPP stack (Redis + EPP Server only)..."
	BRANCH=latest doppler run -- docker compose --profile essential up -d redis epp-server
	@echo "✓ Minimal EPP stack is running"
	@echo "  Redis: localhost:6379"
	@echo "  EPP Server: localhost:700"
```

**Usage**: `make -f Makefile.epp-server docker-stack-minimal`

**Starts**:
- Redis (port 6379)
- EPP Server (port 700)

## Verification

### Before Fix:
```bash
$ docker ps --format "table {{.Names}}\t{{.Status}}"
NAMES                    STATUS
domain-os-epp-server-1   Up 2 minutes (healthy)
domain-os-redis-1        Up 3 minutes (healthy)
```

### After Fix:
```bash
$ make -f Makefile.epp-server docker-stack
Building EPP server image...
Starting full EPP stack (all essential services)...
[+] Running 5/5
 ✔ Volume domain-os_db               Created
 ✔ Container domain-os-redis-1       Healthy
 ✔ Container domain-os-db-1          Healthy
 ✔ Container domain-os-epp-server-1  Started
 ✔ Container domain-os-admin-api-1   Started
✓ EPP stack is running

$ docker ps --format "table {{.Names}}\t{{.Status}}"
NAMES                    STATUS
domain-os-admin-api-1    Up 43 seconds
domain-os-db-1           Up 48 seconds (healthy)
domain-os-epp-server-1   Up 48 seconds (healthy)
domain-os-redis-1        Up 5 minutes (healthy)
```

✅ **All services running!**

## Updated Help Menu

```bash
$ make -f Makefile.epp-server help

EPP Server - Available targets:

Local Development:
  build              - Build EPP server binary locally
  run                - Run EPP server locally
  test               - Run unit tests
  test-int           - Run integration tests (requires Redis)
  clean              - Clean build artifacts

Docker Operations:
  docker-build       - Build Docker image for EPP server
  docker-run         - Run EPP server in Docker with compose
  docker-stop        - Stop EPP server Docker container
  docker-logs        - Show EPP server logs
  docker-shell       - Open shell in running EPP container

Stack Management:
  docker-stack       - Start full stack (DB, Redis, Admin API, EPP Server)
  docker-stack-minimal - Start minimal stack (Redis + EPP Server only)
  docker-stack-down  - Stop all stack services

Monitoring:
  status             - Show container status
  redis-monitor      - Monitor Redis commands
  redis-keys         - Show all Redis keys
  redis-clear        - Clear all Redis data
```

## Service Dependencies in docker-compose.yml

All services properly configured with the `essential` profile:

```yaml
services:
  db:
    image: postgres:16.1
    profiles: [essential, full]
    # ... config

  redis:
    image: redis:7-alpine
    profiles: [essential, full]
    # ... config

  epp-server:
    build:
      dockerfile: Dockerfile.epp
    image: geapex/epp-server:${BRANCH:-latest}
    profiles: [essential, full]
    depends_on:
      redis:
        condition: service_healthy
    # ... config

  admin-api:
    build: .
    image: geapex/domain-os:${BRANCH}
    profiles: [essential, full]
    depends_on:
      db:
        condition: service_healthy
    # ... config
```

## Key Takeaways

1. **Profiles define which services CAN run**, not which services WILL run
2. **Specifying service names** limits what actually starts
3. **To start ALL services in a profile**, don't specify service names:
   - ✅ `docker compose --profile essential up -d`
   - ❌ `docker compose --profile essential up -d service1 service2`

4. **Dependencies are respected**: 
   - `epp-server` depends on `redis` (healthy)
   - `admin-api` depends on `db` (healthy)
   - Docker Compose waits for health checks before starting dependent services

## Commands Summary

| Command | What It Does |
|---------|-------------|
| `make -f Makefile.epp-server docker-stack` | Start full stack (DB + Redis + EPP + API) |
| `make -f Makefile.epp-server docker-stack-minimal` | Start minimal stack (Redis + EPP only) |
| `make -f Makefile.epp-server docker-stack-down` | Stop all services |
| `make -f Makefile.epp-server status` | Show container status |
| `make -f Makefile.epp-server docker-logs` | View EPP server logs |

---
*Issue resolved: October 7, 2025*
