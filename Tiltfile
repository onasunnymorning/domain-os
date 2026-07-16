# Read version from the single source of truth
version = str(local('cat VERSION', quiet=True)).strip()

# Get the current branch
branch = str(local('git branch --show-current', quiet=True)).strip()
# Sanitize branch name for use as a Docker image tag (replace '/' with '-')
tag = branch.replace('/', '-')
# Write the env var to a temporary .env.tilt file for docker-compose to read
local('echo "BRANCH=' + tag + '" > .env.tilt')

# Clean up dangling images and containers
docker_prune_settings(disable=False, max_age_mins=360, num_builds=0, interval_hrs=1, keep_recent=2)

# Read TILT_MODE from environment: 'native' (default) or 'docker'
tilt_mode = os.getenv('TILT_MODE', 'native').lower()

# In native mode we use the 'dev_personal' Doppler config, which inherits from
# 'dev' but overrides Docker-network hostnames with localhost equivalents:
#   DB_HOST=localhost  REDIS_HOST=localhost  STORAGE_ENDPOINT=localhost:9000
# Set these via: doppler secrets set DB_HOST=localhost ... --config dev_personal
DOPPLER_NATIVE = 'doppler run --config dev_personal --'
DOPPLER_DOCKER = 'doppler run --'

if tilt_mode == 'docker':
    # ─── Docker Mode: everything runs as containers ───────────────────────────
    docker_compose('./docker-compose.yml', profiles=['full'], env_file='.env.tilt')

    docker_build('gprins/domain-os-api:'      + tag, '.', dockerfile='Dockerfile',                        build_args={'SKIP_SWAG': 'true'})
    docker_build('gprins/domain-os-whois:'          + tag, '.', dockerfile='./cmd/whois/Dockerfile')
    docker_build('gprins/domain-os-epp:'     + tag, '.', dockerfile='Dockerfile.epp')
    docker_build('gprins/domain-os-worker:' + tag, '.', dockerfile='./cmd/workers/unified/Dockerfile')
    docker_build('gprins/domain-os-mcp:'     + tag, '.', dockerfile='Dockerfile.mcp')

    dc_resource('db',               labels=['infrastructure'])
    dc_resource('redis',            labels=['infrastructure'])
    dc_resource('temporal-postgres',labels=['infrastructure'])
    dc_resource('temporal',         labels=['infrastructure'])
    dc_resource('temporal-ui',      labels=['infrastructure'])
    dc_resource('minio',            labels=['infrastructure'])
    dc_resource('minio-setup',      labels=['init'], trigger_mode=TRIGGER_MODE_MANUAL)
    dc_resource('prometheus',       labels=['infrastructure'], auto_init=False)
    dc_resource('grafana',          labels=['infrastructure'], auto_init=False)
    dc_resource('admin-api',        labels=['app'],  resource_deps=['db'])
    dc_resource('unified-worker',   labels=['app'],  resource_deps=['db', 'temporal'])
    dc_resource('epp-server',       labels=['app'],  auto_init=False)
    dc_resource('whois',            labels=['app'],  auto_init=False)
    dc_resource('mcp-server',       labels=['app'],  auto_init=False, resource_deps=['db'])
    dc_resource('admin-init',       labels=['init'], trigger_mode=TRIGGER_MODE_MANUAL, resource_deps=['admin-api'])

else:
    # ─── Native Mode: infra in Docker, Go + Temporal on host ──────────────────
    docker_compose('./docker-compose.yml', profiles=['infra'], env_file='.env.tilt')

    dc_resource('db',               labels=['infrastructure'])
    dc_resource('redis',            labels=['infrastructure'])
    dc_resource('minio',            labels=['infrastructure'])
    dc_resource('minio-setup',      labels=['init'], trigger_mode=TRIGGER_MODE_MANUAL)

    # Temporal dev server runs natively via the `temporal` CLI with SQLite.
    # Persists workflow state across restarts via a local DB file.
    local_resource(
        'temporal',
        serve_cmd=' '.join([
            'temporal', 'server', 'start-dev',
            '--port', '7233',
            '--ui-port', '8233',
            '--namespace', 'default',
            '--db-filename', './local/temporal.db',
            '--log-format', 'pretty',
        ]),
        readiness_probe=probe(
            period_secs=5,
            tcp_socket=tcp_socket_action(port=7233),
        ),
        labels=['infrastructure'],
    )

    local_resource(
        'admin-api',
        serve_cmd=DOPPLER_NATIVE + ' go run ./cmd/api/ry-admin',
        deps=['./internal', './pkg', './cmd/api/ry-admin'],
        labels=['app'],
        resource_deps=['db'],
    )

    local_resource(
        'unified-worker',
        serve_cmd=DOPPLER_NATIVE + ' go run ./cmd/workers/unified',
        deps=['./internal', './pkg', './cmd/workers/unified'],
        labels=['app'],
        resource_deps=['db', 'temporal'],
    )

    local_resource(
        'epp-server',
        serve_cmd=DOPPLER_NATIVE + ' go run ./cmd/epp',
        deps=['./internal', './pkg', './cmd/epp'],
        labels=['app'],
        auto_init=False,
    )

    local_resource(
        'whois',
        serve_cmd=DOPPLER_NATIVE + ' go run ./cmd/whois',
        deps=['./internal', './pkg', './cmd/whois'],
        labels=['app'],
        auto_init=False,
    )

    local_resource(
        'admin-init',
        cmd=DOPPLER_NATIVE + ' go run ./cmd/api/ry-admin init-registrars',
        trigger_mode=TRIGGER_MODE_MANUAL,
        resource_deps=['admin-api'],
        labels=['init'],
    )

    local_resource(
        'mcp-server',
        serve_cmd=DOPPLER_NATIVE + ' MCP_TRANSPORT=http MCP_PORT=3001 go run ./cmd/mcp',
        deps=['./internal', './pkg', './cmd/mcp'],
        labels=['app'],
        resource_deps=['db'],
        auto_init=False,
    )

# ─── Frontend: always native (Next.js dev server) ─────────────────────────────
local_resource(
    'frontend',
    serve_cmd='cd frontend && PORT=3002 npm run dev',
    env={
        'NEXT_PUBLIC_API_URL':         'http://localhost:8080',
        'NEXT_PUBLIC_API_TOKEN':       os.getenv('ADMIN_TOKEN', 'devtoken'),
        'NEXT_PUBLIC_AUTH0_ENABLED':   'false',
        'NEXT_PUBLIC_TEMPORAL_UI_URL':  'http://localhost:8233',  # native temporal CLI UI; docker mode uses :8081
        'NEXT_PUBLIC_STORAGE_UI_URL':  'http://localhost:9001',   # MinIO console from docker-compose
        'NEXT_PUBLIC_APP_VERSION':     version + '-dev',
    },
    deps=[
        'frontend/package.json',
        'frontend/package-lock.json',
        'frontend/next.config.ts',
        'frontend/tsconfig.json',
    ],
    labels=['app'],
)
