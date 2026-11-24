#!/usr/bin/env bash
# Tests deploy/backup-banco.sh on this computer: dumps the local database with
# pg_dumpall, checks the copy is encrypted (no plain SQL in it), opens it with
# the private key and finds the database inside. No upload (BACKUP_DEST empty).
#   bash scripts/test-backup.sh        (needs .env.local with PG_BIN, age on PATH or AGE/AGE_KEYGEN)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
set -a; . "$ROOT/.env.local"; set +a
AGE="${AGE:-age}"
AGE_KEYGEN="${AGE_KEYGEN:-age-keygen}"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
"$AGE_KEYGEN" -o "$WORK/chave.txt" 2>/dev/null
"$AGE_KEYGEN" -y "$WORK/chave.txt" > "$WORK/recipient.txt"
cat > "$WORK/backup.env" <<EOF
BACKUP_DUMP_CMD='"$PG_BIN/pg_dumpall" -d "$DATABASE_URL"'
BACKUP_RECIPIENT_FILE=$WORK/recipient.txt
BACKUP_DIR=$WORK/backups
BACKUP_KEEP=2
BACKUP_DEST=
AGE=$AGE
EOF
for i in 1 2 3; do
	BACKUP_ENV="$WORK/backup.env" BACKUP_STAMP="2026112${i}-0630" bash "$ROOT/deploy/backup-banco.sh"
done
n=$(ls "$WORK/backups" | wc -l)
[[ $n -eq 2 ]] || { echo "FALHOU: guardou $n cópias, esperava 2"; exit 1; }
last="$WORK/backups/ziv-20261123-0630.sql.gz.age"
[[ -f "$last" ]] || { echo "FALHOU: falta a cópia mais nova"; exit 1; }
if grep -aq "CREATE ROLE\|CREATE DATABASE" "$last"; then echo "FALHOU: cópia em texto aberto"; exit 1; fi
"$AGE" -d -i "$WORK/chave.txt" -o "$WORK/aberta.sql.gz" "$last" || { echo "FALHOU: cópia não abre com a chave"; exit 1; }
gunzip -c "$WORK/aberta.sql.gz" > "$WORK/aberta.sql"
grep -q "CREATE ROLE" "$WORK/aberta.sql" || { echo "FALHOU: a cópia aberta não tem o banco"; exit 1; }
# A failing dump fails the run and leaves no copy behind.
sed -i 's|^BACKUP_DUMP_CMD=.*|BACKUP_DUMP_CMD=false|' "$WORK/backup.env"
if BACKUP_ENV="$WORK/backup.env" BACKUP_STAMP="20261124-0630" bash "$ROOT/deploy/backup-banco.sh" 2>/dev/null; then
	echo "FALHOU: dump com erro e a rodada passou"; exit 1
fi
if ls "$WORK/backups" | grep -q "20261124"; then echo "FALHOU: sobrou cópia da rodada com erro"; exit 1; fi
echo "ok: 2 cópias cifradas, a mais nova abre com a chave privada; dump com erro falha a rodada"
