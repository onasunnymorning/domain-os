# Get the current branch
branch = str(local('git branch --show-current', quiet=True)).strip()
# Sanitize branch name for use as a Docker image tag (replace '/' with '-')
tag = branch.replace('/', '-')
# Write the env var to a temporary .env.tilt file for docker-compose to read
local('echo "BRANCH=' + tag + '" > .env.tilt')

# Clean up dangling images and containers
docker_prune_settings(disable=False, max_age_mins=360, num_builds=0, interval_hrs=1, keep_recent=2)

# Point Tilt at the existing docker-compose configuration with the 'full' profile
docker_compose("./docker-compose.yml", profiles=['full'], env_file='.env.tilt')

# Explicitly build local Docker images that docker-compose lacks local builds for
docker_build('geapex/domain-os:' + tag, '.', dockerfile='Dockerfile')
docker_build('geapex/whois:' + tag, '.', dockerfile='./cmd/whois/Dockerfile')
docker_build('geapex/epp-server:' + tag, '.', dockerfile='Dockerfile.epp')
docker_build('geapex/unified-worker:' + tag, '.', dockerfile='./cmd/workers/unified/Dockerfile')

# Group database and cache resources
dc_resource('db', labels=["infrastructure"])
dc_resource('redis', labels=["infrastructure"])

# Endpoints
dc_resource('admin-api', labels=["endpoints"])
dc_resource('epp-server', labels=["endpoints"])
dc_resource('whois', labels=["endpoints"])

# Init containers
dc_resource('admin-init', labels=["init"], trigger_mode=TRIGGER_MODE_MANUAL)

# Infrastructure
dc_resource('temporal-postgres', labels=["infrastructure"])
dc_resource('temporal', labels=["infrastructure"])
dc_resource('temporal-ui', labels=["infrastructure"])

# Workers
dc_resource('unified-worker', labels=["workers"])

# Object Storage
dc_resource('minio', labels=["infrastructure"])
dc_resource('minio-setup', labels=["init"], trigger_mode=TRIGGER_MODE_MANUAL)

dc_resource('prometheus', labels=["infrastructure"])
dc_resource('grafana', labels=["infrastructure"])
dc_resource('metabase', labels=["infrastructure"])
dc_resource('metabase-init', labels=["init"], trigger_mode=TRIGGER_MODE_MANUAL)

# Start Next.js frontend native development server
local_resource(
    'frontend',
    serve_cmd='cd frontend && ' +
              'NEXT_PUBLIC_API_URL="http://localhost:${API_PORT:-8080}" ' +
              'NEXT_PUBLIC_API_TOKEN="${ADMIN_TOKEN:-devtoken}" ' +
              'PORT=3002 npm run dev',
    labels=["endpoints"]
)
