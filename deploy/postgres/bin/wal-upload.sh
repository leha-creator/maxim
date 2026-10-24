#!/usr/bin/env bash
# Выгрузка сегментов WAL из локального архива (том wal-archive, archive_command) в
# <root>/db/wal/<сегмент>; выгруженный и проверенный по размеру файл удаляется локально.
# Один проход; цикл — backup-agent.sh. Потеря при аварии сервера — не больше
# archive_timeout (60 с) + BACKUP_WAL_INTERVAL (07 §8: не больше 5 минут).
set -Eeuo pipefail
LOG_COMPONENT=wal-upload
# shellcheck source=deploy/postgres/bin/backup-lib.sh
. /usr/local/bin/backup-lib.sh
as_postgres "$@"

mc_setup
dest="$(backup_root)/db/wal"
uploaded=0
pending=0
shopt -s nullglob
files=("$WAL_ARCHIVE_DIR"/*)
for f in "${files[@]}"; do
  name=$(basename "$f")
  [[ $name == *.tmp ]] && continue
  pending=$((pending + 1))
  size=$(wc -c < "$f")
  if ! err=$(m cp --quiet "$f" "$dest/$name" 2>&1 >/dev/null); then
    log error wal_upload_failed segment="$name" error="${err:0:300}"
    state_write wal false segment="$name" pending="$pending" error="${err:0:300}"
    notify_failure wal "upload of ${name} failed: ${err:0:300}"
    exit 1
  fi
  remote=$(m stat --no-list --json "$dest/$name" | jq -r '.size')
  if [[ $remote != "$size" ]]; then
    log error wal_upload_size_mismatch segment="$name" local="$size" remote="$remote"
    state_write wal false segment="$name" error=size_mismatch
    notify_failure wal "size mismatch for ${name}: local ${size}, remote ${remote}"
    exit 1
  fi
  rm -f "$f"
  uploaded=$((uploaded + 1))
  log debug wal_uploaded segment="$name" bytes="$size"
done
state_write wal true uploaded="$uploaded"
if ((uploaded > 0)); then
  log info wal_upload_done uploaded="$uploaded"
fi
