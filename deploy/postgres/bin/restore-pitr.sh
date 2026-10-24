#!/usr/bin/env bash
# Восстановление на момент времени (07 §8, в пределах 7 суток): физическая копия, начатая до
# цели, + WAL из корзины копий до --target-time; затем promote. Используется при аварии и на
# квартальной проверке (deploy/app/README.md, «Восстановление»).
#
#   restore-pitr.sh --target-time "YYYY-MM-DD HH:MM:SS" --pgdata <пустой каталог>
#                   [--base <YYYYMMDDTHHMMSSZ>] [--port 5434] [--keep-running]
#
# Время цели — UTC. Без --keep-running сервер после восстановления останавливается, и каталог
# готов стать PGDATA нового контейнера postgres. В восстановленном кластере archive_mode
# выключен: он не пишет в архив WAL рабочего кластера.
set -Eeuo pipefail
LOG_COMPONENT=restore-pitr
# shellcheck source=deploy/postgres/bin/backup-lib.sh
. /usr/local/bin/backup-lib.sh
as_postgres "$@"

target=""
pgdata=""
base=""
port=5434
keep_running=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --target-time) target=${2:-}; shift 2 ;;
    --pgdata) pgdata=${2:-}; shift 2 ;;
    --base) base=${2:-}; shift 2 ;;
    --port) port=${2:-}; shift 2 ;;
    --keep-running) keep_running=1; shift ;;
    -h|--help) sed -n '2,12p' "$0"; exit 0 ;;
    *) log error invalid_argument arg="$1"; exit 2 ;;
  esac
done
# Допускается и вид RFC 3339: 2026-10-24T15:00:00Z.
target=${target/T/ }
target=${target%Z}
if [[ ! $target =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}\ [0-9]{2}:[0-9]{2}:[0-9]{2}$ || -z $pgdata ]]; then
  log error invalid_arguments target_time="$target" pgdata="$pgdata" hint="--target-time 'YYYY-MM-DD HH:MM:SS' --pgdata DIR"
  exit 2
fi
if [[ -e $pgdata && -n $(ls -A "$pgdata" 2>/dev/null) ]]; then
  log error pgdata_not_empty pgdata="$pgdata"
  exit 2
fi
target_unix=$(date -u -d "$target" +%s)

start=$(now_ms)
mc_setup
root=$(backup_root)
trap 'log error restore_pitr_failed line="${LINENO}" command="${BASH_COMMAND}"' ERR

# Физическая копия: заданная или самая поздняя, начатая раньше цели.
if [[ -z $base ]]; then
  while IFS= read -r b; do
    t=$(date -u -d "${b:0:4}-${b:4:2}-${b:6:2} ${b:9:2}:${b:11:2}:${b:13:2}" +%s)
    ((t < target_unix)) && base=$b
  done < <(m ls --recursive --json "$root/db/base/" 2>/dev/null | jq -r 'select(.type == "file") | .key' \
    | sed -n 's|^\([0-9]\{8\}T[0-9]\{6\}Z\)/manifest\.json$|\1|p' | sort)
fi
if [[ -z $base ]]; then
  log error restore_pitr_no_base target_time="$target"
  exit 1
fi
log info restore_pitr_started base="$base" target_time="$target" pgdata="$pgdata"

work=$(mktemp -d "$BACKUP_TMP_DIR/pitr.XXXXXX")
cleanup_add "$work"
m cp --quiet "$root/db/base/$base/base.tar.gz" "$work/base.tar.gz" >/dev/null
m cp --quiet "$root/db/base/$base/manifest.json" "$work/manifest.json" >/dev/null
install -d -m 0700 "$pgdata"
tar -xzf "$work/base.tar.gz" -C "$pgdata"
mkdir -p "$pgdata/pg_wal"
chmod 0700 "$pgdata"
rm -f "$pgdata/postmaster.pid"
cat >> "$pgdata/postgresql.auto.conf" <<CONF
# restore-pitr.sh: восстановление на момент времени из ${base}
restore_command = '/usr/local/bin/wal-fetch.sh %f %p'
recovery_target_time = '${target}+00'
recovery_target_action = 'promote'
archive_mode = 'off'
CONF
touch "$pgdata/recovery.signal"
log info restore_pitr_base_ready base="$base" start_segment="$(jq -r '.start_segment' "$work/manifest.json")" \
  download_ms="$(($(now_ms) - start))"

sock=$(dirname "$pgdata")
pg_ctl -D "$pgdata" -w -t 3600 -l "$work/postgres.log" \
  -o "-p $port -c listen_addresses='' -c unix_socket_directories='$sock'" start >/dev/null
# pg_ctl -w ждёт приёма соединений; promote наступает после достижения цели.
deadline=$(($(date +%s) + 3600))
until [[ $(psql -X -At -h "$sock" -p "$port" -U "${POSTGRES_USER:-maxim}" -d postgres -c 'SELECT pg_is_in_recovery()' 2>/dev/null) == f ]]; do
  if (($(date +%s) > deadline)); then
    log error restore_pitr_timeout note="still in recovery after 1 h" log_tail="$(tail -n 20 "$work/postgres.log")"
    pg_ctl -D "$pgdata" -m fast -w stop >/dev/null || true
    exit 1
  fi
  sleep 2
done
log info restore_pitr_promoted base="$base" target_time="$target" log_tail="$(grep -E 'recovery stopping|consistent recovery|selected new timeline' "$work/postgres.log" | tail -n 3)"

if ((keep_running)); then
  log info restore_pitr_done pgdata="$pgdata" socket_dir="$sock" port="$port" running=true duration_ms="$(($(now_ms) - start))"
else
  pg_ctl -D "$pgdata" -m fast -w stop >/dev/null
  log info restore_pitr_done pgdata="$pgdata" running=false duration_ms="$(($(now_ms) - start))"
fi
