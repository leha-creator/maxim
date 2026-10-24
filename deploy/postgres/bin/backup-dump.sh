#!/usr/bin/env bash
# Ежедневная логическая копия базы (07 §8: полная копия, хранение 30 суток):
# pg_dump -Fc → <root>/db/dump/YYYY/MM/DD/maxim-<ts>.dump, .sha256 и манифест .json.
set -Eeuo pipefail
LOG_COMPONENT=backup-dump
# shellcheck source=deploy/postgres/bin/backup-lib.sh
. /usr/local/bin/backup-lib.sh
as_postgres "$@"
trap 'fail_job dump dump_failed "line ${LINENO}: ${BASH_COMMAND}"' ERR

start=$(now_ms)
ts=$(date -u +%Y%m%dT%H%M%SZ)
day=$(date -u +%Y/%m/%d)
mc_setup
work=$(mktemp -d "$BACKUP_TMP_DIR/dump.XXXXXX")
cleanup_add "$work"
file="$work/maxim-$ts.dump"
key="$(backup_root)/db/dump/$day/maxim-$ts"

log info dump_started database="${PGDATABASE:-}" key="${key#backup/}.dump"
# Версия схемы исходной базы (параметры подключения — из PG*).
# shellcheck disable=SC2119
version=$(migration_version)
server_version=$(psql -X -At -c 'SHOW server_version')
pg_dump -Fc -Z 6 --no-password -f "$file"
bytes=$(wc -c < "$file")
sha=$(sha256sum "$file" | cut -d' ' -f1)
printf '%s  %s\n' "$sha" "maxim-$ts.dump" > "$file.sha256"
jq -n --arg key "${key#backup/}.dump" --arg sha "$sha" --arg at "$ts" --arg mv "$version" \
  --arg sv "$server_version" --argjson bytes "$bytes" --argjson ms "$(($(now_ms) - start))" \
  '{kind:"dump", created_at:$at, key:$key, bytes:$bytes, sha256:$sha, migration_version:$mv, server_version:$sv, duration_ms:$ms}' \
  > "$file.json"
m cp --quiet "$file" "$key.dump" >/dev/null
m cp --quiet "$file.sha256" "$key.dump.sha256" >/dev/null
# Манифест — последним: по нему проверка восстановления находит законченные копии.
m cp --quiet "$file.json" "$key.json" >/dev/null
remote=$(m stat --no-list --json "$key.dump" | jq -r '.size')
[[ $remote == "$bytes" ]] || fail_job dump dump_size_mismatch "uploaded ${remote} bytes, expected ${bytes}"

state_write dump true key="${key#backup/}.dump" bytes="$bytes" migration_version="$version"
log info dump_done key="${key#backup/}.dump" bytes="$bytes" migration_version="$version" duration_ms="$(($(now_ms) - start))"
