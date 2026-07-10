#!/usr/bin/env bash
# ---------------------------------------------------------
# pg-dump-local.sh — Dump the local Docker PostgreSQL DB
#
# Usage:
#   doppler run -- ./scripts/pg-dump-local.sh           # custom format (default)
#   doppler run -- ./scripts/pg-dump-local.sh --sql      # plain SQL format
#
# Environment (injected by Doppler):
#   DB_USER, DB_PASS, DB_NAME
#   DB_HOST (optional, defaults to localhost)
#   DB_PORT (optional, defaults to 5432)
# ---------------------------------------------------------
set -euo pipefail

# ── Config ────────────────────────────────────────────────
DB_HOST="${DB_HOST:-localhost}"
DB_PORT="${DB_PORT:-5432}"
DUMP_DIR="$(cd "$(dirname "$0")/.." && pwd)/backups"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"

# ── Parse args ────────────────────────────────────────────
FORMAT="custom"  # default: pg_dump custom format (.dump)
if [[ "${1:-}" == "--sql" ]]; then
  FORMAT="sql"
fi

# ── Validate ──────────────────────────────────────────────
for var in DB_USER DB_PASS DB_NAME; do
  if [[ -z "${!var:-}" ]]; then
    echo "❌ $var is not set. Run this script with: doppler run -- $0" >&2
    exit 1
  fi
done

if ! command -v pg_dump &>/dev/null; then
  echo "❌ pg_dump not found. Install PostgreSQL client tools:" >&2
  echo "   brew install libpq && brew link --force libpq" >&2
  exit 1
fi

# ── Prepare ───────────────────────────────────────────────
mkdir -p "$DUMP_DIR"

export PGPASSWORD="$DB_PASS"

if [[ "$FORMAT" == "sql" ]]; then
  OUTFILE="$DUMP_DIR/${DB_NAME}_${TIMESTAMP}.sql"
  echo "📦 Dumping $DB_NAME → $OUTFILE (plain SQL)…"
  pg_dump \
    --host="$DB_HOST" \
    --port="$DB_PORT" \
    --username="$DB_USER" \
    --dbname="$DB_NAME" \
    --no-owner \
    --no-privileges \
    --verbose \
    > "$OUTFILE"
else
  OUTFILE="$DUMP_DIR/${DB_NAME}_${TIMESTAMP}.dump"
  echo "📦 Dumping $DB_NAME → $OUTFILE (custom format)…"
  pg_dump \
    --host="$DB_HOST" \
    --port="$DB_PORT" \
    --username="$DB_USER" \
    --dbname="$DB_NAME" \
    --format=custom \
    --no-owner \
    --no-privileges \
    --verbose \
    --file="$OUTFILE"
fi

unset PGPASSWORD

# ── Summary ───────────────────────────────────────────────
SIZE="$(du -h "$OUTFILE" | cut -f1)"
echo ""
echo "✅ Dump complete"
echo "   File: $OUTFILE"
echo "   Size: $SIZE"
echo ""
echo "To restore into Neon:"
if [[ "$FORMAT" == "sql" ]]; then
  echo "   psql \"\$NEON_DATABASE_URL\" < $OUTFILE"
else
  echo "   pg_restore --no-owner --no-privileges --clean --if-exists -d \"\$NEON_DATABASE_URL\" $OUTFILE"
fi
