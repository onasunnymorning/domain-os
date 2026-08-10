#!/usr/bin/env bash
# Preflight checks for the local development environment.
#
# Run via: make doctor        (verbose — reports every check)
#          make doctor -q     (quiet — prints only failures; used by `make dev`)
#
# Every failure must name the thing that is wrong AND the command that fixes it.
# A check that says "Docker not found" and stops has just moved the question
# rather than answered it.
#
# Exit codes: 0 = ready, 1 = at least one blocking problem.

set -uo pipefail

QUIET=0
[[ "${1:-}" == "--quiet" ]] && QUIET=1

FAILURES=0
WARNINGS=0

# Colours, but only when attached to a terminal.
if [[ -t 1 ]]; then
  RED=$'\033[31m'; YEL=$'\033[33m'; GRN=$'\033[32m'; DIM=$'\033[2m'; RST=$'\033[0m'
else
  RED=""; YEL=""; GRN=""; DIM=""; RST=""
fi

ok()   { [[ $QUIET -eq 1 ]] || echo "  ${GRN}ok${RST}    $1"; }
warn() { WARNINGS=$((WARNINGS+1)); echo "  ${YEL}warn${RST}  $1"; [[ -n "${2:-}" ]] && echo "        ${DIM}fix: $2${RST}"; }
fail() { FAILURES=$((FAILURES+1));  echo "  ${RED}FAIL${RST}  $1"; [[ -n "${2:-}" ]] && echo "        ${DIM}fix: $2${RST}"; }

[[ $QUIET -eq 1 ]] || echo "Checking your local environment..."
[[ $QUIET -eq 1 ]] || echo ""

# ── Docker ───────────────────────────────────────────────────────────────────
if ! command -v docker >/dev/null 2>&1; then
  fail "Docker is not installed" "install Docker Desktop: https://docs.docker.com/get-docker/"
elif ! docker info >/dev/null 2>&1; then
  fail "Docker is installed but the daemon is not running" "start Docker Desktop and wait for the whale icon to settle"
else
  ok "Docker daemon is running"

  if docker compose version >/dev/null 2>&1; then
    ok "docker compose v2 is available"
  else
    fail "docker compose v2 is not available" "update Docker Desktop; the legacy 'docker-compose' binary will not work"
  fi
fi

# ── Go toolchain ─────────────────────────────────────────────────────────────
# go.mod pins the exact patch version; .tool-versions repeats it for asdf/mise.
WANT_GO="$(grep -E '^go [0-9]' go.mod 2>/dev/null | awk '{print $2}')"
if ! command -v go >/dev/null 2>&1; then
  fail "Go is not installed (want ${WANT_GO:-see go.mod})" "install via 'asdf install' / 'mise install', or https://go.dev/dl/"
else
  HAVE_GO="$(go version | awk '{print $3}' | sed 's/^go//')"
  if [[ -n "$WANT_GO" && "$HAVE_GO" != "$WANT_GO" ]]; then
    # Go builds fine on a newer patch; only flag it so a version-specific bug is
    # not a mystery later.
    warn "Go $HAVE_GO installed, go.mod asks for $WANT_GO" "asdf install   (or: mise install)"
  else
    ok "Go $HAVE_GO matches go.mod"
  fi
fi

# ── Node (frontend only) ─────────────────────────────────────────────────────
WANT_NODE="$(cat frontend/.nvmrc 2>/dev/null | tr -d '[:space:]')"
if ! command -v node >/dev/null 2>&1; then
  warn "Node is not installed (want v${WANT_NODE:-22}) — backend works without it" "only needed for 'make dev-frontend'"
else
  HAVE_NODE_MAJOR="$(node -v | sed 's/^v//' | cut -d. -f1)"
  if [[ -n "$WANT_NODE" && "$HAVE_NODE_MAJOR" != "$WANT_NODE" ]]; then
    warn "Node v$(node -v | sed 's/^v//') installed, frontend/.nvmrc asks for v${WANT_NODE}" "nvm use   (or: asdf install)"
  else
    ok "Node $(node -v) matches frontend/.nvmrc"
  fi
fi

# ── Configuration ────────────────────────────────────────────────────────────
if [[ -f .env ]]; then
  ok ".env is present"
  # A .env predating a newly added variable is the classic silent failure.
  if [[ .env.example -nt .env ]]; then
    warn ".env is older than .env.example — a new variable may be missing" "diff <(sort .env) <(sort .env.example)"
  fi
else
  fail ".env is missing" "cp .env.example .env    ('make dev' does this for you)"
fi

if [[ -f .env.example ]]; then
  ok ".env.example is present"
else
  fail ".env.example is missing" "make generate-env-example"
fi

# ── Ports ────────────────────────────────────────────────────────────────────
# Only the ports the essential profile publishes. A port held by our own stack
# is fine — that just means `make dev` is already running.
check_port() {
  local port="$1" what="$2"
  local pids
  pids="$(lsof -nP -iTCP:"$port" -sTCP:LISTEN -t 2>/dev/null)"
  if [[ -z "$pids" ]]; then
    ok "port $port is free ($what)"
    return
  fi
  # Is it ours? Ask Docker rather than string-matching `docker ps` output:
  # contiguous ports are collapsed into ranges there ("0.0.0.0:9000-9001->..."),
  # so grepping for ":9000->" reports our own MinIO as a foreign process.
  if [[ -n "$(docker ps -q --filter "publish=${port}" \
                --filter "label=com.docker.compose.project=domain-os" 2>/dev/null)" ]]; then
    ok "port $port is held by this project's own stack ($what)"
  else
    local who
    who="$(ps -o comm= -p "$(echo "$pids" | head -1)" 2>/dev/null | xargs basename 2>/dev/null)"
    fail "port $port is in use by '${who:-unknown}' ($what)" "stop it, or: lsof -nP -iTCP:$port -sTCP:LISTEN"
  fi
}

if command -v lsof >/dev/null 2>&1; then
  check_port 5432 "postgres"
  check_port 6379 "redis"
  check_port 8080 "admin api"
  check_port 8081 "temporal ui"
  check_port 7233 "temporal"
  check_port 9000 "minio"
  check_port 9001 "minio console"
  check_port 7700 "epp"
else
  warn "lsof not available — skipping port checks" "ports will still fail loudly at 'make dev' if taken"
fi

# ── Disk ─────────────────────────────────────────────────────────────────────
# Images plus build cache for five Go services runs to a few GB.
AVAIL_GB="$(df -Pg . 2>/dev/null | awk 'NR==2 {print $4}')"
if [[ -n "$AVAIL_GB" ]]; then
  if (( AVAIL_GB < 10 )); then
    warn "only ${AVAIL_GB}GB free on this volume; the stack needs roughly 8GB" "docker system prune -a"
  else
    ok "${AVAIL_GB}GB disk available"
  fi
fi

# ── Summary ──────────────────────────────────────────────────────────────────
if [[ $FAILURES -gt 0 ]]; then
  echo ""
  echo "${RED}$FAILURES blocking problem(s) found.${RST} Fix the FAIL lines above, then re-run 'make doctor'."
  exit 1
fi

if [[ $QUIET -eq 0 ]]; then
  echo ""
  if [[ $WARNINGS -gt 0 ]]; then
    echo "${GRN}Ready.${RST} ($WARNINGS warning(s) — not blocking.)"
  else
    echo "${GRN}Ready.${RST} Run 'make dev'."
  fi
fi
exit 0
