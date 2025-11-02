#!/usr/bin/env bash
set -euo pipefail

# Simple smoke test for escrow import workflow via Admin API
# Requires: curl, jq, running docker-compose stack
# Usage: scripts/escrow_smoke.sh <tld> <xml_path> [api_host] [api_port] [admin_token]

TLD=${1:-aquarelle}
XML=${2:-scraps/aquarelle_2024-06-20_full_S1_R0.xml}
API_HOST=${3:-localhost}
API_PORT=${4:-8080}
# Prefer explicit arg, then environment, then auto-detect from running container, then known default
TOKEN=${5:-${ADMIN_TOKEN:-}}
if [ -z "${TOKEN}" ]; then
  # Try to read the token from the running admin-api container (if available)
  if command -v docker >/dev/null 2>&1; then
    CONTAINER_TOKEN=$(docker compose exec -T admin-api printenv ADMIN_TOKEN 2>/dev/null || true)
    if [ -n "${CONTAINER_TOKEN:-}" ]; then
      TOKEN="$CONTAINER_TOKEN"
    fi
  fi
fi
# Final fallback to the documented dev token phrase if nothing else found
TOKEN=${TOKEN:-the-brave-may-not-live-forever-but-the-cautious-do-not-live-at-all}
BASE="http://${API_HOST}:${API_PORT}"

if [ ! -f "$XML" ]; then
  echo "XML file not found: $XML" >&2
  exit 1
fi

FNAME=$(basename "$XML")

echo "Presigning upload for $FNAME..."
# Capture HTTP status to provide a helpful error if unauthorized
HTTP_CODE=0
PRESIGN=$(curl -sS -o >(cat) -w "\n%{http_code}" -X POST "${BASE}/escrow/uploads/presign?filename=${FNAME}" -H "Authorization: Bearer ${TOKEN}")
HTTP_CODE=$(echo "$PRESIGN" | tail -n1)
BODY=$(echo "$PRESIGN" | sed '$d')
if [ "$HTTP_CODE" = "401" ]; then
  echo "Authorization failed (401). The admin API requires a Bearer token that matches its ADMIN_TOKEN." >&2
  echo "Tip: export ADMIN_TOKEN=\"$TOKEN\" or pass it as the 5th arg, or run: docker compose exec admin-api printenv ADMIN_TOKEN" >&2
  exit 1
fi
PRESIGN="$BODY"
URL=$(echo "$PRESIGN" | jq -r .url)
OBJ=$(echo "$PRESIGN" | jq -r .objectKey)

# If the presigned URL uses the internal Docker hostname, connect via localhost but preserve Host header using curl --resolve.
# This avoids breaking the signature (Host header is part of SigV4).
HOSTPORT=$(echo "$URL" | awk -F/ '{print $3}')
CURL_RESOLVE_ARGS=()
if echo "$HOSTPORT" | grep -qi '^minio:'; then
  CURL_RESOLVE_ARGS=(--resolve "${HOSTPORT}:127.0.0.1")
fi

if [ -z "$URL" ] || [ "$URL" == "null" ]; then
  echo "Failed to obtain presigned URL" >&2
  echo "$PRESIGN" >&2
  exit 1
fi

echo "Uploading to S3/MinIO..."
set +e
UPLOAD_OUT=$(curl -sS -o /dev/null -w "%{http_code}" -X PUT --upload-file "$XML" "${URL}" "${CURL_RESOLVE_ARGS[@]}" 2>&1)
CURL_RC=$?
set -e
if [ $CURL_RC -ne 0 ] || [ "$UPLOAD_OUT" != "200" ] && [ "$UPLOAD_OUT" != "204" ]; then
  echo "Upload failed (curl_rc=$CURL_RC, http_code=$UPLOAD_OUT)." >&2
  echo "If http_code=403 and URL host is an internal Docker hostname, ensure we used curl --resolve (should be automatic)." >&2
  echo "Alternatively, rebuild/restart admin-api so presigned URLs use localhost by setting MINIO_PUBLIC_ENDPOINT=\"http://localhost:9000\"." >&2
  exit 1
fi

echo "Starting workflow..."
START=$(curl -fsSL -X POST "${BASE}/escrow/imports" -H 'Content-Type: application/json' -H "Authorization: Bearer ${TOKEN}" \
  -d "{\"tld\":\"${TLD}\",\"objectKey\":\"${OBJ}\"}")
WFID=$(echo "$START" | jq -r .workflowId)
RUNID=$(echo "$START" | jq -r .runId)
WFLINK=$(echo "$START" | jq -r .url)

if [ -z "$WFID" ] || [ "$WFID" == "null" ]; then
  echo "Failed to start workflow" >&2
  echo "$START" >&2
  exit 1
fi

echo "Workflow started: $WFID ($RUNID)"
echo "Temporal UI: $WFLINK"

echo "Polling imports list for summary.json (TLD=${TLD})..."
ATTEMPTS=60
SLEEP=5
FOUND="false"
for i in $(seq 1 $ATTEMPTS); do
  LIST=$(curl -fsSL -H "Authorization: Bearer ${TOKEN}" "${BASE}/escrow/imports?tld=${TLD}&limit=50")
  HAS=$(echo "$LIST" | jq -r ".items[] | select(.workflowId==\"${WFID}\") | .hasSummary" | head -n1)
  if [ "$HAS" == "true" ]; then
    SUM=$(echo "$LIST" | jq -r ".items[] | select(.workflowId==\"${WFID}\") | .summaryKey" | head -n1)
    echo "Summary available at S3 key: $SUM"
    FOUND="true"
    break
  fi
  echo "[$i/$ATTEMPTS] Waiting for summary..."
  sleep $SLEEP
done

if [ "$FOUND" != "true" ]; then
  echo "Timed out waiting for summary.json. Check Temporal UI: $WFLINK" >&2
  exit 2
fi

echo "Done."
