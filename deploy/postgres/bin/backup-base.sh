#!/usr/bin/env bash
# Ежедневная физическая копия для восстановления на момент времени (07 §8, 7 суток):
# pg_basebackup → <root>/db/base/<ts>/base.tar.gz и manifest.json. WAL берётся из архива
# (wal-upload.sh), поэтому -X none.
set -Eeuo pipefail
LOG_COMPONENT=backup-base
# shellcheck source=deploy/postgres/bin/backup-lib.sh
. /usr/local/bin/backup-lib.sh
as_postgres "$@"
trap 'fail_job base base_failed "line ${LINENO}: ${BASH_COMMAND}"' ERR

start=$(now_ms)
ts=$(date -u +%Y%m%dT%H%M%SZ)
mc_setup
work=$(mktemp -d "$BACKUP_TMP_DIR/base.XXXXXX")
cleanup_add "$work"
key="$(backup_root)/db/base/$ts"

log info base_started key="${key#backup/}"
pg_basebackup --no-password -D "$work/data" -Ft -z -X none -c fast -l "maxim-$ts" --no-sync
# Начальный сегмент WAL — из backup_label: очистка WAL оставляет всё, что нужно этой копии.
label=$(tar -xzOf "$work/data/base.tar.gz" backup_label)
start_segment=$(sed -n 's/^START WAL LOCATION: .* (file \([0-9A-F]\{24\}\))$/\1/p' <<< "$label")
start_time=$(sed -n 's/^START TIME: //p' <<< "$label")
[[ -n $start_segment ]] || fail_job base base_label_unreadable "no START WAL LOCATION in backup_label"
bytes=$(du -sb "$work/data" 2>/dev/null | cut -f1 || true)
[[ $bytes =~ ^[0-9]+$ ]] || bytes=$(find "$work/data" -type f -exec cat {} + | wc -c)
# Версия схемы исходной базы (параметры подключения — из PG*).
# shellcheck disable=SC2119
version=$(migration_version)
jq -n --arg at "$ts" --arg key "${key#backup/}" --arg seg "$start_segment" --arg st "$start_time" \
  --arg mv "$version" --argjson bytes "$bytes" --argjson ms "$(($(now_ms) - start))" \
  '{kind:"base", created_at:$at, key:$key, start_segment:$seg, start_time:$st, migration_version:$mv, bytes:$bytes, duration_ms:$ms}' \
  > "$work/manifest.json"
for f in "$work"/data/*; do
  m cp --quiet "$f" "$key/$(basename "$f")" >/dev/null
done
m cp --quiet "$work/manifest.json" "$key/manifest.json" >/dev/null

state_write base true key="${key#backup/}" start_segment="$start_segment"
log info base_done key="${key#backup/}" start_segment="$start_segment" bytes="$bytes" duration_ms="$(($(now_ms) - start))"
