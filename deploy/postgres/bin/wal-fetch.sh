#!/usr/bin/env bash
# restore_command при восстановлении на момент времени: wal-fetch.sh %f %p.
# Код 1 для отсутствующего сегмента — нормальный конец WAL для PostgreSQL.
set -Euo pipefail
LOG_COMPONENT=wal-fetch
# shellcheck source=deploy/postgres/bin/backup-lib.sh
. /usr/local/bin/backup-lib.sh
trap - EXIT

name=$1
target=$2
src="$(backup_root)/db/wal/$name"
if mc --config-dir "${MC_CONFIG_DIR:?MC_CONFIG_DIR is required}" --no-color cp --quiet "$src" "$target" >/dev/null 2>&1; then
  log debug wal_fetched segment="$name"
  exit 0
fi
log info wal_not_found segment="$name"
exit 1
