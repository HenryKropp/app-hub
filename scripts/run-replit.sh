#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

if [[ -z "${DATABASE_URL:-}" ]]; then
  echo "DATABASE_URL is required. Connect a PostgreSQL database in Replit." >&2
  exit 1
fi

mkdir -p generated-artifacts
if [[ ! -f dist/agent/AppHubAgente.exe ]] || [[ -n "$(find backend -type f \( -name '*.go' -o -name 'go.mod' -o -name 'go.sum' \) -newer dist/agent/AppHubAgente.exe -print -quit 2>/dev/null)" ]]; then
  bash scripts/build-agent.sh
fi
if [[ ! -f frontend/dist/index.html ]] || [[ -n "$(find frontend/src frontend/index.html frontend/vite.config.ts frontend/package.json frontend/package-lock.json frontend/tsconfig*.json -type f -newer frontend/dist/index.html -print -quit 2>/dev/null)" ]]; then
  npm run build --prefix frontend
fi
if [[ ! -x generated-artifacts/app-hub-server ]] || [[ -n "$(find backend -type f \( -name '*.go' -o -name '*.sql' -o -name 'go.mod' -o -name 'go.sum' \) -newer generated-artifacts/app-hub-server -print -quit 2>/dev/null)" ]]; then
  (cd backend && go build -o ../generated-artifacts/app-hub-server ./cmd/server)
fi
# Replit reaches the server from outside the machine (ADR 048 default is 127.0.0.1).
export LISTEN_HOST="${LISTEN_HOST:-0.0.0.0}"
exec ./generated-artifacts/app-hub-server
