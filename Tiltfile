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
#   DB_HOST=localhost  REDIS_HOST=localhost  MINIO_ENDPOINT=localhost:9000
# Set these via: doppler secrets set DB_HOST=localhost ... --config dev_personal
DOPPLER_NATIVE = 'doppler run --config dev_personal --'
DOPPLER_DOCKER = 'doppler run --'

if tilt_mode == 'docker':
    # ─── Docker Mode: everything runs as containers ───────────────────────────
    docker_compose('./docker-compose.yml', profiles=['full'], env_file='.env.tilt')

    docker_build('geapex/domain-os:'      + tag, '.', dockerfile='Dockerfile',                        build_args={'SKIP_SWAG': 'true'})
    docker_build('geapex/whois:'          + tag, '.', dockerfile='./cmd/whois/Dockerfile')
    docker_build('geapex/epp-server:'     + tag, '.', dockerfile='Dockerfile.epp')
    docker_build('geapex/unified-worker:' + tag, '.', dockerfile='./cmd/workers/unified/Dockerfile')

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
    dc_resource('admin-init',       labels=['init'], trigger_mode=TRIGGER_MODE_MANUAL, resource_deps=['admin-api'])

else:
    # ─── Native Mode: infra in Docker, Go apps on host ────────────────────────
    docker_compose('./docker-compose.yml', profiles=['infra'], env_file='.env.tilt')

    dc_resource('db',               labels=['infrastructure'])
    dc_resource('redis',            labels=['infrastructure'])
    dc_resource('temporal-dev',     labels=['infrastructure'])
    dc_resource('minio',            labels=['infrastructure'])
    dc_resource('minio-setup',      labels=['init'], trigger_mode=TRIGGER_MODE_MANUAL)

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
        resource_deps=['db', 'temporal-dev'],
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

# ─── Frontend: always native (Next.js dev server) ─────────────────────────────
local_resource(
    'frontend',
    serve_cmd='cd frontend && PORT=3002 npm run dev',
    env={
        'NEXT_PUBLIC_API_URL':         'http://localhost:8080',
        'NEXT_PUBLIC_API_TOKEN':       'devtoken',
        'NEXT_PUBLIC_AUTH0_ENABLED':   'false',
        'NEXT_PUBLIC_TEMPORAL_UI_URL':  'http://localhost:8233',  # temporal-dev built-in UI (native) or temporal-ui (docker)
        'NEXT_PUBLIC_APP_VERSION':     'dev',
    },
    deps=[
        'frontend/package.json',
        'frontend/package-lock.json',
        'frontend/next.config.ts',
        'frontend/tsconfig.json',
    ],
    labels=['app'],
)
