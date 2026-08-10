#!/bin/bash
# Reusable test database setup script
# Used by Makefile and CI workflows to avoid copy-pasting the Postgres startup command
#
# Usage: setup-test-db.sh [CONTAINER_NAME] [HOST_PORT]
#
# HOST_PORT defaults to 5432 so CI is unaffected. `make test` passes 5433 so the
# test database can coexist with a `make dev` stack already holding 5432 —
# otherwise running the suite straight after starting the dev stack fails on a
# port collision rather than on anything to do with the code.
set -euo pipefail

CONTAINER_NAME="${1:-testdb}"
HOST_PORT="${2:-5432}"

# Clean up any previous instance
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true

# Start Postgres with SSL + SCRAM-SHA-256 auth
docker run --rm -d \
  -e POSTGRES_HOST_AUTH_METHOD=scram-sha-256 \
  -e POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256 \
  -e POSTGRES_PASSWORD=unittest \
  -e POSTGRES_USER=postgres \
  --name "$CONTAINER_NAME" \
  -p "${HOST_PORT}:5432" \
  postgres:16.1 \
  -c ssl=on \
  -c ssl_cert_file=/etc/ssl/certs/ssl-cert-snakeoil.pem \
  -c ssl_key_file=/etc/ssl/private/ssl-cert-snakeoil.key

# Wait for database to be ready
echo "Waiting for database ($CONTAINER_NAME) to be ready..."
for i in $(seq 1 30); do
  if docker exec "$CONTAINER_NAME" pg_isready -U postgres > /dev/null 2>&1; then
    # Verify the host port mapping is active using bash /dev/tcp
    if (exec 3<>"/dev/tcp/127.0.0.1/${HOST_PORT}") >/dev/null 2>&1; then
      exec 3>&-
      echo "Database ready and accessible on host port ${HOST_PORT}!"
      exit 0
    else
      echo "Database ready inside container, waiting for host port forward..."
    fi
  fi
  sleep 1
done

echo "ERROR: Database failed to start or host port ${HOST_PORT} was not bound within 30 seconds!" >&2
exit 1
