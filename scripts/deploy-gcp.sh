#!/usr/bin/env bash
# Publishes the current code to the Ziv server on Google Cloud (ADR 022).
# Builds here (the server has only 1 GB of RAM), copies with rsync and
# restarts the service. Migrations run by themselves when the server starts.
#
#   bash scripts/deploy-gcp.sh
#
# Needs the SSH key ~/.ssh/ziv-gcp (its public half is in the VM's metadata).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
HOST="${ZIV_HOST:-ziv@34.24.21.104}"
URL="${ZIV_URL:-https://zivapp.duckdns.org}"
KEY="${ZIV_SSH_KEY:-$HOME/.ssh/ziv-gcp}"
OUT="$(mktemp -d)"
trap 'rm -rf "$OUT"' EXIT

mkdir -p "$OUT/frontend" "$OUT/agent"
echo "1/4 site"
(cd "$ROOT/frontend" && mise x -- npm run build >/dev/null)
cp -r "$ROOT/frontend/dist" "$OUT/frontend/"
echo "2/4 servidor"
(cd "$ROOT/backend" && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 mise x -- go build -trimpath -o "$OUT/server" ./cmd/server)
echo "3/4 agente (aponta para $URL)"
(cd "$ROOT/backend" && GOOS=windows GOARCH=amd64 CGO_ENABLED=0 mise x -- go build -trimpath \
  -ldflags "-H=windowsgui -X main.hubURL=$URL" -o "$OUT/agent/AppHubAgente.exe" ./cmd/agent)
echo "4/4 enviando e reiniciando"
rsync -a --delete --exclude ziv.env --exclude .dbpw -e "ssh -i $KEY" "$OUT/" "$HOST:ziv/"
ssh -i "$KEY" "$HOST" 'systemctl --user restart ziv && sleep 4 && systemctl --user is-active ziv'
curl -fsS -o /dev/null -w "site: %{http_code}\n" "$URL/"
