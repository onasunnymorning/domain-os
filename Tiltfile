# Get the current branch
branch = str(local('git branch --show-current', quiet=True)).strip()
# Write the env var to a temporary .env.tilt file for docker-compose to read
local('echo "BRANCH=' + branch + '" > .env.tilt')

# Clean up dangling images and containers
docker_prune_settings(disable=False, max_age_mins=360, num_builds=0, interval_hrs=1, keep_recent=2)

# Point Tilt at the existing docker-compose configuration with the 'full' profile
docker_compose("./docker-compose.yml", profiles=['full'], env_file='.env.tilt')

# Explicitly build local Docker images that docker-compose lacks local builds for
docker_build('geapex/domain-os:' + branch, '.', dockerfile='Dockerfile')
docker_build('geapex/whois:' + branch, '.', dockerfile='./cmd/whois/Dockerfile')
docker_build('geapex/epp-server:' + branch, '.', dockerfile='Dockerfile.epp')
docker_build('geapex/domain-lifecycle-worker:' + branch, '.', dockerfile='./cmd/workers/domainLifecycle/Dockerfile')
docker_build('geapex/escrow-import-worker:' + branch, '.', dockerfile='./cmd/workers/escrowImport/Dockerfile')

# Group backend database and cache resources
dc_resource('db', labels=["backend"])
dc_resource('redis', labels=["backend"])

# Core API
dc_resource('admin-api', labels=["api"])
# Init container for admin DB setup
dc_resource('admin-init', labels=["init"], trigger_mode=TRIGGER_MODE_MANUAL)

# Event broker
dc_resource('msg-broker', labels=["events"])

# EPP and Whois services
dc_resource('epp-server', labels=["epp"])
dc_resource('whois', labels=["whois"])

# Temporal Workflow stack
dc_resource('temporal-postgres', labels=["temporal"])
dc_resource('temporal', labels=["temporal"])
dc_resource('temporal-ui', labels=["temporal"])

# Workers
dc_resource('domain-lifecycle-worker', labels=["workers"])
dc_resource('escrow-import-worker', labels=["workers"])

# Object Storage
dc_resource('minio', labels=["storage"])
dc_resource('minio-setup', labels=["init"], trigger_mode=TRIGGER_MODE_MANUAL)

# Observability and Data Platforms
dc_resource('prometheus', labels=["observability"])
dc_resource('grafana', labels=["observability"])
dc_resource('metabase', labels=["observability"])
dc_resource('metabase-init', labels=["init"], trigger_mode=TRIGGER_MODE_MANUAL)

# Start Next.js frontend native development server
local_resource(
    'frontend',
    serve_cmd='cd frontend && ' +
              'NEXT_PUBLIC_API_URL="http://localhost:${API_PORT:-8080}" ' +
              'NEXT_PUBLIC_API_TOKEN="${ADMIN_TOKEN:-devtoken}" ' +
              'PORT=3002 npm run dev',
    labels=["frontend"]
)
