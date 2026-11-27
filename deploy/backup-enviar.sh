#!/usr/bin/env bash
# Sends the daily encrypted copies (deploy/backup-banco.sh) to the bucket
# (Fase 0 de segurança, ADR 048). Runs as ROOT (ziv-backup-enviar.timer),
# installed at /usr/local/sbin/ziv-backup-enviar: the apps run as the user
# ziv, and that user is blocked from the Google metadata server
# (ziv-bloqueia-metadados.service), so an app cannot borrow the VM identity.
# The VM runs as the service account ziv-copias, which can only create
# objects in the bucket. Settings in /etc/ziv-backup-enviar.env (root-owned,
# never a file the ziv user can write: this script runs as root).
set -euo pipefail
CONF="${BACKUP_ENVIAR_CONF:-/etc/ziv-backup-enviar.env}"
if [[ -f "$CONF" ]]; then
	# shellcheck disable=SC1090
	. "$CONF"
fi
SRC="${BACKUP_SRC_DIR:-/home/ziv/backups}"
STATE="${BACKUP_STATE:-/var/lib/ziv-backup/enviados}"
BACKUP_DEST="${BACKUP_DEST:?BACKUP_DEST (gs://bucket/pasta)}"
METADATA="${BACKUP_METADATA_URL:-http://metadata.google.internal}"
STORAGE="${BACKUP_STORAGE_URL:-https://storage.googleapis.com}"

case "$BACKUP_DEST" in
gs://*) ;;
*)
	echo "enviar: destino desconhecido em BACKUP_DEST: $BACKUP_DEST" >&2
	exit 1
	;;
esac
dest="${BACKUP_DEST#gs://}"
bucket="${dest%%/*}"
prefix="${dest#"$bucket"}"
prefix="${prefix#/}"
prefix="${prefix%/}"

mkdir -p "$(dirname "$STATE")"
touch "$STATE"
token="$(curl -fsS -H "Metadata-Flavor: Google" \
	"$METADATA/computeMetadata/v1/instance/service-accounts/default/token" |
	sed -n 's/.*"access_token" *: *"\([^"]*\)".*/\1/p')"
if [[ -z "$token" ]]; then
	echo "enviar: sem token da identidade da máquina" >&2
	exit 1
fi

fail=0
shopt -s nullglob
for f in "$SRC"/ziv-*.sql.gz.age; do
	n="$(basename "$f")"
	# Only regular files with the expected name: this runs as root over a
	# folder the ziv user writes.
	[[ "$n" =~ ^ziv-[0-9]{8}-[0-9]{4}\.sql\.gz\.age$ ]] || continue
	[[ -f "$f" && ! -L "$f" ]] || continue
	grep -qxF "$n" "$STATE" && continue
	name="${prefix:+$prefix/}$n"
	# ifGenerationMatch=0: never overwrites; 412 means it is already there.
	code="$(curl -sS -o /dev/null -w '%{http_code}' -X POST --data-binary @"$f" \
		-H "Authorization: Bearer $token" -H "Content-Type: application/octet-stream" \
		"$STORAGE/upload/storage/v1/b/$bucket/o?uploadType=media&ifGenerationMatch=0&name=${name//\//%2F}")" || code=000
	case "$code" in
	200 | 412)
		echo "$n" >>"$STATE"
		echo "enviar: $n -> gs://$bucket/$name ($code)"
		;;
	*)
		echo "enviar: falhou $n (HTTP $code)" >&2
		fail=1
		;;
	esac
done
exit "$fail"
