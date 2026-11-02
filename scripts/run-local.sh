#!/usr/bin/env bash
# Runs Ziv on this computer (Windows with Git Bash, Mac or Linux) at
# http://localhost:8080, with an agent that pairs with this local Hub.
# Settings come from .env.local (not committed); see docs/INFRASTRUCTURE.md.
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ -f .env.local ]]; then
  set -a
  # shellcheck disable=SC1091
  source .env.local
  set +a
fi

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "DATABASE_URL is required. Put it in .env.local (see docs/INFRASTRUCTURE.md)." >&2
  exit 1
fi

# Optional: start a local PostgreSQL kept outside the repository.
if [[ -n "${PG_BIN:-}" && -n "${PG_DATA:-}" ]]; then
  if ! "$PG_BIN/pg_ctl" -D "$PG_DATA" status >/dev/null 2>&1; then
    "$PG_BIN/pg_ctl" -D "$PG_DATA" -o "-p ${PG_PORT:-5433} -c listen_addresses=localhost" \
      -l "$PG_DATA/../pg.log" start >/dev/null
  fi
fi

export PORT="${PORT:-8080}"
export BASE_URL="${BASE_URL:-http://localhost:$PORT}"
export AGENT_BINARY_PATH="dist/agent/AppHubAgente.exe"

mkdir -p generated-artifacts
HUB_URL="$BASE_URL" bash scripts/build-agent.sh >/dev/null
npm run build --prefix frontend >/dev/null
SERVER="generated-artifacts/ziv-local$(cd backend && go env GOEXE)"
(cd backend && go build -o "../$SERVER" ./cmd/server)

echo "Ziv: $BASE_URL"
exec "./$SERVER"
