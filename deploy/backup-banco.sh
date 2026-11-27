#!/usr/bin/env bash
# Daily copy of the main Ziv database (Fase 0 de segurança, ADR 048).
# pg_dumpall -> gzip -> age (public key only: the server cannot open its own
# copies) -> keeps the newest BACKUP_KEEP here. ziv-backup-enviar (root)
# then sends them to the bucket.
# Run by ziv-backup.timer (deploy/ziv-backup.{service,timer}); settings in
# ~/ziv/backup.env (deploy/backup.env.example). Any failing step fails the run.
set -euo pipefail
ENV_FILE="${BACKUP_ENV:-$HOME/ziv/backup.env}"
if [[ -f "$ENV_FILE" ]]; then
	# shellcheck disable=SC1090
	. "$ENV_FILE"
fi
BACKUP_DUMP_CMD="${BACKUP_DUMP_CMD:-podman exec ziv-db pg_dumpall -U postgres}"
BACKUP_RECIPIENT_FILE="${BACKUP_RECIPIENT_FILE:-$HOME/ziv/backup-recipient.txt}"
BACKUP_DIR="${BACKUP_DIR:-$HOME/backups}"
BACKUP_KEEP="${BACKUP_KEEP:-7}"
AGE="${AGE:-age}"
STAMP="${BACKUP_STAMP:-$(date -u +%Y%m%d-%H%M)}"

if [[ ! -s "$BACKUP_RECIPIENT_FILE" ]]; then
	echo "backup: falta a chave pública em $BACKUP_RECIPIENT_FILE" >&2
	exit 1
fi
umask 077
mkdir -p "$BACKUP_DIR"
out="$BACKUP_DIR/ziv-$STAMP.sql.gz.age"
part="$out.part"
trap 'rm -f "$part"' EXIT
eval "$BACKUP_DUMP_CMD" | gzip -9 | "$AGE" -R "$BACKUP_RECIPIENT_FILE" -o "$part"
mv "$part" "$out"
echo "backup: $out ($(wc -c <"$out") bytes)"

# Keep only the newest copies here (names sort by date).
mapfile -t old < <(ls -1 "$BACKUP_DIR"/ziv-*.sql.gz.age | sort | head -n "-$BACKUP_KEEP")
if ((${#old[@]})); then
	rm -f -- "${old[@]}"
fi

# Sending to the bucket is a separate job, as root (deploy/backup-enviar.sh):
# the user ziv runs the apps and cannot reach the VM identity.
