#!/usr/bin/env bash
# Real end-to-end flow of Etapa 2 (ADR 044): a script with a test plan and a
# vault key is tested by the real Agente (built from this repo, on this
# computer) and goes to the air by itself; then a version that exits with
# code 1 is refused and nothing changes in the air.
#
# Needs a test Hub with DEV_MODE (never on 8080) and the ziv-db container:
#   HUB=http://localhost:8092 AGENTE=<built agent binary> bash scripts/e2e-teste-script.sh
set -euo pipefail

HUB="${HUB:-http://localhost:8092}"
AGENTE="${AGENTE:?build it with: (cd backend && go build -o <path> ./cmd/agent)}"
DB="${DB_CONTAINER:-ziv-db}"
WORK="$(mktemp -d)"
JAR="$WORK/cookies"
SECRET="valor-secreto-e2e-$RANDOM$RANDOM"
AGENT_PID=""
cleanup() {
  [[ -n "$AGENT_PID" ]] && kill "$AGENT_PID" 2>/dev/null || true
  [[ -n "${KEEP_LOG:-}" ]] && cp "$WORK/home/.apphub-agente/agente.log" "$KEEP_LOG" 2>/dev/null || true
  rm -rf "$WORK"
}
trap cleanup EXIT

fail() { echo "FALHOU: $*" >&2; exit 1; }
api() { # method path [json]
  if [[ $# -ge 3 ]]; then
    curl -s -b "$JAR" -c "$JAR" -X "$1" -H 'Content-Type: application/json' -d "$3" "$HUB$2"
  else
    curl -s -b "$JAR" -c "$JAR" -X "$1" "$HUB$2"
  fi
}
sql() { podman exec "$DB" psql -U postgres -tAc "$1" | tr -d '[:space:]'; }
b64() { printf '%s' "$1" | base64 -w0; }

api POST /api/dev/login '{}' >/dev/null

# 1. The real Agente, paired with the dev person.
# Computers of earlier runs may still look connected for a while: only one
# paired now counts.
known=$(api GET /api/computers/me | python3 -c 'import json,sys; print(max((c["id"] for c in json.load(sys.stdin)["computers"]), default=0))')
mkdir -p "$WORK/home"
# A fresh HOME keeps this Agente apart from any other; the real python goes
# first in PATH because version-manager shims need the real HOME.
PY_DIR="$(dirname "$(python3 -c 'import sys; print(sys.executable)')")"
HOME="$WORK/home" PATH="$PY_DIR:$PATH" APPHUB_URL="$HUB" "$AGENTE" &
AGENT_PID=$!
LOG="$WORK/home/.apphub-agente/agente.log"
code=""
for _ in $(seq 1 30); do
  code=$(grep -o 'codigo=[A-Z0-9]*' "$LOG" 2>/dev/null | head -1 | cut -d= -f2) || true
  [[ -n "$code" ]] && break
  sleep 1
done
[[ -n "$code" ]] || fail "o Agente não pediu pareamento"
api POST "/api/pairings/$code/approve" '{}' >/dev/null
pc=""
for _ in $(seq 1 30); do
  pc=$(api GET /api/computers/me | python3 -c 'import json,sys; cs=[c for c in json.load(sys.stdin)["computers"] if c["connected"] and c["agent_version"]=="0.4.0" and c["id"] > '"$known"']; print(max((c["id"] for c in cs), default=""))')
  [[ -n "$pc" ]] && break
  sleep 1
done
[[ -n "$pc" ]] || fail "o Agente não conectou"
echo "Agente conectado: computador $pc"

# 2. A script with a test plan and a required vault key, born under test.
MAIN_OK='import os, sys
senha = os.environ["E2E_SENHA"]
assert os.environ.get("ZIV_TESTE") == "1", "sem modo teste"
print("Modo teste, pasta:", os.environ["ZIV_PASTA_TESTE"])
print("Senha usada:", senha)
open("relatorio.pdf", "w").write("%PDF-1.4 relatorio de teste")
print("Concluído")
'
MANIFEST=$(python3 - <<EOF
import json
print(json.dumps({"name": "E2E teste de script $RANDOM", "summary": "Gera um relatório para o teste de ponta a ponta.",
  "kind": "script", "entrypoint": "main.py", "area": "Research",
  "keys": [{"name": "E2E_SENHA", "label": "Senha do e2e", "required": True}],
  "teste": {"acessa": ["nada externo (chave E2E_SENHA)"], "pula": "nada",
            "espera": [{"arquivos": "*.pdf", "no_minimo": 1}, {"log_contem": "Concluído"}], "tempo_maximo_min": 2}}))
EOF
)
app=$(api POST /api/scripts/import "{\"manifest\": $MANIFEST, \"files\": {\"main.py\": {\"content_base64\": \"$(b64 "print('rascunho inicial')")\"}}, \"in_test\": true}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["id"])')
echo "App $app criado em teste"
api PUT "/api/apps/$app/keys/E2E_SENHA" "{\"value\": \"$SECRET\"}" >/dev/null
# The dev company may require an admin's approval before any app runs, the
# test included (ADR 044): the dev person is an admin and approves.
approve() {
  api PUT "/api/security/apps/$app" '{"classification": "internal", "purpose": "Teste de ponta a ponta", "external_domains": [], "approved": true}' >/dev/null
}
approve

draft_of() { # main.py content -> draft id
  api POST "/api/apps/$app/drafts" "{\"files\": {\"main.py\": {\"content_base64\": \"$(b64 "$1")\"}}, \"note\": \"$2\"}" | python3 -c 'import json,sys; print(json.load(sys.stdin)["draft_id"])'
}
wait_draft() { # draft id -> final status
  local st=""
  for _ in $(seq 1 120); do
    st=$(api GET "/api/apps/$app/drafts/$1" | python3 -c 'import json,sys; d=json.load(sys.stdin); print(d["status"])')
    [[ "$st" == "no_ar" || "$st" == "reprovada" || "$st" == "cancelada" ]] && { echo "$st"; return; }
    sleep 2
  done
  echo "$st"
}

d1=$(draft_of "$MAIN_OK" "Primeira versão")
stage=$(api POST "/api/apps/$app/drafts/$d1/test" | python3 -c 'import json,sys; print(json.load(sys.stdin)["stage"])')
[[ "$stage" == "esperando_confirmacao" ]] || fail "etapa $stage, esperava esperando_confirmacao"

# 3. The owner confirms the plan on the site, with this computer.
fp=$(api GET "/api/apps/$app/test-plan" | python3 -c 'import json,sys; print(json.load(sys.stdin)["fingerprint"])')
api POST "/api/apps/$app/test-plan/confirm" "{\"fingerprint\": \"$fp\", \"computer_id\": $pc}" >/dev/null

# 4. Wait: the Agente tests and the Hub publishes.
st=$(wait_draft "$d1")
[[ "$st" == "no_ar" ]] || fail "rascunho $d1 terminou $st: $(sql "SELECT status_reason || ' | ' || left(test_log, 400) FROM app_drafts WHERE id=$d1")"
[[ "$(sql "SELECT tested FROM app_versions WHERE app_id=$app ORDER BY number DESC LIMIT 1")" == "t" ]] || fail "versão no ar sem a marca testada"
[[ "$(sql "SELECT in_test FROM apps WHERE id=$app")" == "f" ]] || fail "app continua em teste"
log=$(podman exec "$DB" psql -U postgres -tAc "SELECT test_log FROM app_drafts WHERE id=$d1")
grep -q "Modo teste" <<<"$log" || fail "o log não mostra o modo teste: $log"
grep -q "$SECRET" <<<"$log" && fail "o valor da chave apareceu no log do teste"
grep -q "$SECRET" "$WORK"/home/.apphub-agente/testes/*/*/.apphub-console.txt 2>/dev/null && fail "o valor da chave ficou no console do PC"
notice=$(api GET /api/notices | python3 -c "import json,sys; print(next((n['text'] for n in json.load(sys.stdin) if n['app_id']==$app), ''))")
[[ "$notice" == *"versão 2 no ar — testada:"* ]] || fail "aviso não apareceu: $notice"
echo "Passou: versão no ar testada; aviso: $notice"

# 5. A version that exits with code 1 is refused; nothing changes in the air.
approve # the new version in the air needs the approval again
before=$(sql "SELECT count(*) FROM app_versions WHERE app_id=$app")
d2=$(draft_of 'import sys
print("Quebrou")
sys.exit(1)
' "Versão quebrada")
api POST "/api/apps/$app/drafts/$d2/test" >/dev/null
st=$(wait_draft "$d2")
[[ "$st" == "reprovada" ]] || fail "rascunho quebrado terminou $st"
reason=$(podman exec "$DB" psql -U postgres -tAc "SELECT status_reason FROM app_drafts WHERE id=$d2")
grep -q "código de saída 1" <<<"$reason" || fail "reprovada pelo motivo errado: $reason"
[[ "$(sql "SELECT count(*) FROM app_versions WHERE app_id=$app")" == "$before" ]] || fail "a versão quebrada foi ao ar"
echo "Passou: versão quebrada reprovada, nada mudou no ar"
echo "E2E OK (app $app)"
