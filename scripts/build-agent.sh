#!/usr/bin/env bash
# Compiles the Agente do Hub for Windows into dist/agent/AppHubAgente.exe.
# HUB_URL is the address the agent uses to reach the Hub's Go server.
# Releasing a new agent: bump agentproto.Version, run this script, then restart
# the Hub. Connected agents (0.2.0+) update themselves; same version = no update.
set -euo pipefail
ROOT="$(git rev-parse --show-toplevel)"
HUB_URL="${HUB_URL:-http://localhost:8080}"
mkdir -p "$ROOT/dist/agent"
cd "$ROOT/backend"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 mise x -- go build -trimpath \
  -ldflags "-H=windowsgui -X main.hubURL=$HUB_URL" \
  -o "$ROOT/dist/agent/AppHubAgente.exe" ./cmd/agent
echo "$ROOT/dist/agent/AppHubAgente.exe"
