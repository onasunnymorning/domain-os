#!/bin/bash
# Reusable test database setup script
# Used by Makefile and CI workflows to avoid copy-pasting the Postgres startup command
set -euo pipefail

CONTAINER_NAME="${1:-testdb}"

# Clean up any previous instance
docker rm -f "$CONTAINER_NAME" 2>/dev/null || true

# Start Postgres with SSL + SCRAM-SHA-256 auth
docker run --rm -d \
  -e POSTGRES_HOST_AUTH_METHOD=scram-sha-256 \
  -e POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256 \
  -e POSTGRES_PASSWORD=unittest \
  -e POSTGRES_USER=postgres \
  --name "$CONTAINER_NAME" \
  -p 5432:5432 \
  postgres:16.1 \
  -c ssl=on \
  -c ssl_cert_file=/etc/ssl/certs/ssl-cert-snakeoil.pem \
  -c ssl_key_file=/etc/ssl/private/ssl-cert-snakeoil.key

# Wait for database to be ready
echo "Waiting for database ($CONTAINER_NAME) to be ready..."
for i in $(seq 1 30); do
  if docker exec "$CONTAINER_NAME" pg_isready -U postgres > /dev/null 2>&1; then
    # Verify the host port mapping is active using bash /dev/tcp
    if (exec 3<>/dev/tcp/127.0.0.1/5432) >/dev/null 2>&1; then
      exec 3>&-
      echo "Database ready and accessible on host port 5432!"
      exit 0
    else
      echo "Database ready inside container, waiting for host port forward..."
    fi
  fi
  sleep 1
done

echo "ERROR: Database failed to start or host port 5432 was not bound within 30 seconds!" >&2
exit 1

