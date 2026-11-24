#!/usr/bin/env bash
# Compiles the Agente do Hub for Windows into dist/agent/AppHubAgente.exe.
# HUB_URL is the address the agent uses to reach the Hub's Go server.
# Releasing a new agent: bump agentproto.Version, run this script, then restart
# the Hub. Connected agents (0.2.0+) update themselves; same version = no update.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
HUB_URL="${HUB_URL:-${REPLIT_DEV_DOMAIN:+https://$REPLIT_DEV_DOMAIN}}"
HUB_URL="${HUB_URL:-http://localhost:8080}"
mkdir -p "$ROOT/dist/agent"
cd "$ROOT/backend"
# Updates are signed (ADR 048). Locally a test key is made once, outside git;
# production builds use the real key (scripts/deploy-gcp.sh).
KEY="${ZIV_AGENT_SIGNING_KEY:-$ROOT/generated-artifacts/agente-assinatura-teste.key}"
if [[ ! -f "$KEY" ]]; then
  mkdir -p "$(dirname "$KEY")"
  go run ./cmd/agentkey gerar "$KEY" >/dev/null
fi
PUB="$(go run ./cmd/agentkey publica "$KEY")"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-H=windowsgui -X main.hubURL=$HUB_URL -X main.updateKey=$PUB" \
  -o "$ROOT/dist/agent/AppHubAgente.exe" ./cmd/agent
go run ./cmd/agentkey assinar "$KEY" "$ROOT/dist/agent/AppHubAgente.exe"
echo "$ROOT/dist/agent/AppHubAgente.exe"
