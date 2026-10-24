#!/usr/bin/env bash
# Очистка корзины копий по срокам 07 §8:
#   логические копии старше BACKUP_DUMP_RETENTION_DAYS (30), физические — старше
#   BACKUP_BASE_RETENTION_DAYS (8); самая новая копия каждого вида остаётся всегда;
#   сегменты WAL раньше начала самой старой оставшейся физической копии.
set -Eeuo pipefail
LOG_COMPONENT=backup-prune
# shellcheck source=deploy/postgres/bin/backup-lib.sh
. /usr/local/bin/backup-lib.sh
as_postgres "$@"
trap 'fail_job prune prune_failed "line ${LINENO}: ${BASH_COMMAND}"' ERR

dump_days=${BACKUP_DUMP_RETENTION_DAYS:-30}
base_days=${BACKUP_BASE_RETENTION_DAYS:-8}
mc_setup
root=$(backup_root)
now=$(date +%s)

# list_keys <path> — ключи файлов под путём (рекурсивно, относительно пути).
list_keys() {
  m ls --recursive --json "$1/" 2>/dev/null | jq -r 'select(.type == "file") | .key'
}

# ts_unix <YYYYMMDDTHHMMSSZ> — время метки в секундах.
ts_unix() {
  local t=$1
  date -u -d "${t:0:4}-${t:4:2}-${t:6:2} ${t:9:2}:${t:11:2}:${t:13:2}" +%s
}

# Логические копии: группа файлов maxim-<ts>.{dump,dump.sha256,json}; самая новая остаётся.
mapfile -t dumps < <(list_keys "$root/db/dump" | sed -n 's|^\(.*/maxim-[0-9]\{8\}T[0-9]\{6\}Z\)\.json$|\1|p' | sort -t/ -k4)
removed_dumps=0
if ((${#dumps[@]} > 1)); then
  for d in "${dumps[@]:0:${#dumps[@]}-1}"; do
    ts=${d##*/maxim-}
    if (((now - $(ts_unix "$ts")) / 86400 >= dump_days)); then
      m rm --quiet "$root/db/dump/$d.dump" "$root/db/dump/$d.dump.sha256" "$root/db/dump/$d.json" >/dev/null
      removed_dumps=$((removed_dumps + 1))
      log info dump_removed key="$d"
    fi
  done
fi

# Физические копии: каталоги <ts>/ с manifest.json; самая новая остаётся.
mapfile -t bases < <(list_keys "$root/db/base" | sed -n 's|^\([0-9]\{8\}T[0-9]\{6\}Z\)/manifest\.json$|\1|p' | sort)
kept=()
removed_bases=0
for i in "${!bases[@]}"; do
  b=${bases[$i]}
  if ((i < ${#bases[@]} - 1)) && (((now - $(ts_unix "$b")) / 86400 >= base_days)); then
    m rm --recursive --force --quiet "$root/db/base/$b/" >/dev/null
    removed_bases=$((removed_bases + 1))
    log info base_removed key="$b"
  else
    kept+=("$b")
  fi
done

# WAL: всё до начального сегмента самой старой оставшейся физической копии.
removed_wal=0
if ((${#kept[@]} > 0)); then
  manifest="$BACKUP_TMP_DIR/prune-manifest.$$.json"
  cleanup_add "$manifest"
  m cp --quiet "$root/db/base/${kept[0]}/manifest.json" "$manifest" >/dev/null
  start_segment=$(jq -r '.start_segment' "$manifest")
  if [[ $start_segment =~ ^[0-9A-F]{24}$ ]]; then
    batch=()
    while IFS= read -r seg; do
      # Только сегменты (24 hex); файлы .history и .backup остаются.
      if [[ $seg =~ ^[0-9A-F]{24}$ && $seg < $start_segment ]]; then
        batch+=("$root/db/wal/$seg")
        if ((${#batch[@]} >= 200)); then
          m rm --quiet "${batch[@]}" >/dev/null
          removed_wal=$((removed_wal + ${#batch[@]}))
          batch=()
        fi
      fi
    done < <(list_keys "$root/db/wal")
    if ((${#batch[@]} > 0)); then
      m rm --quiet "${batch[@]}" >/dev/null
      removed_wal=$((removed_wal + ${#batch[@]}))
    fi
  fi
else
  log warn prune_no_base note="no physical backup yet: WAL is kept"
fi

state_write prune true removed_dumps="$removed_dumps" removed_bases="$removed_bases" removed_wal="$removed_wal"
log info prune_done dumps="${#dumps[@]}" removed_dumps="$removed_dumps" bases="${#bases[@]}" \
  removed_bases="$removed_bases" removed_wal="$removed_wal"
