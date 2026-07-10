#!/bin/bash
# Creates the Temporal user and databases on the shared postgres instance.
# Runs once on first container start (docker-entrypoint-initdb.d).
# The temporal role uses md5 auth so temporalio/auto-setup can connect
# without needing scram-sha-256 support.
set -euo pipefail

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
    -- Temporal role: CREATEDB required for auto-setup to manage schema
    DO \$\$
    BEGIN
        IF NOT EXISTS (SELECT FROM pg_catalog.pg_roles WHERE rolname = 'temporal') THEN
            CREATE ROLE temporal WITH LOGIN PASSWORD 'temporal' CREATEDB;
        ELSE
            ALTER ROLE temporal CREATEDB;
        END IF;
    END
    \$\$;

    -- Temporal databases (pre-create so auto-setup just validates/migrates)
    SELECT 'CREATE DATABASE temporal OWNER temporal'
        WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'temporal')\gexec

    SELECT 'CREATE DATABASE temporal_visibility OWNER temporal'
        WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = 'temporal_visibility')\gexec

    -- Grant all privileges
    GRANT ALL PRIVILEGES ON DATABASE temporal TO temporal;
    GRANT ALL PRIVILEGES ON DATABASE temporal_visibility TO temporal;
EOSQL

# Allow the temporal role to authenticate with md5 (pg_hba is scram-sha-256 by default)
# Append a rule before the catch-all that allows temporal user via md5
HBA_FILE=$(psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -t -c "SHOW hba_file;" | tr -d ' ')
echo "host    temporal             temporal        all             md5" >> "$HBA_FILE"
echo "host    temporal_visibility  temporal        all             md5" >> "$HBA_FILE"

# Reload pg_hba without restart
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" -c "SELECT pg_reload_conf();"

echo "✅ Temporal databases and role created successfully."
