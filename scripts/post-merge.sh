#!/usr/bin/env bash
# Prepare merged code; workflow reconciliation starts the server afterward.
set -euo pipefail
cd "$(dirname "$0")/.."

# Restore exactly the locked dependencies without interactive audit requests.
npm ci --prefix frontend --prefer-offline --no-audit --no-fund \
  --fetch-retries=0 --fetch-timeout=20000

(cd backend && go mod download)
npm run build --prefix frontend
bash scripts/build-agent.sh
mkdir -p generated-artifacts
(cd backend && go build -o ../generated-artifacts/app-hub-server ./cmd/server)

# Embedded database migrations run once the server starts, as in normal startup.
echo "Post-merge preparation complete."
