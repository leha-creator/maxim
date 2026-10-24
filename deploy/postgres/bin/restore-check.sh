#!/usr/bin/env bash
# Проверка восстановления (07 §8): последняя логическая копия (или --key) восстанавливается во
# временный кластер PostgreSQL внутри контейнера backup и проверяется:
#   1. контрольная сумма файла копии;
#   2. возраст копии не больше BACKUP_MAX_AGE_HOURS (только для последней копии);
#   3. версия миграций совпадает с манифестом копии;
#   4. обязательные таблицы на месте;
#   5. журнал проводок сходится — те же запросы, что у задания check_ledger_balance
#      (internal/ledger/postgres/queries/reports.sql: SumBalancesBySide, ListCacheMismatches,
#      ListNegativeLiabilities; править вместе);
#   6. файлы медиатеки из базы (до 50 последних) есть в копии объектов.
# Данные не покидают сервер: кластер живёт в BACKUP_TMP_DIR и удаляется при выходе.
# Длительность восстановления — оценка времени восстановления (цель 07 §8 — до 4 часов).
#
#   restore-check.sh [--key db/dump/YYYY/MM/DD/maxim-<ts>] [--keep]
set -Eeuo pipefail
LOG_COMPONENT=restore-check
# shellcheck source=deploy/postgres/bin/backup-lib.sh
. /usr/local/bin/backup-lib.sh
as_postgres "$@"

key=""
keep=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --key) key=${2:-}; shift 2 ;;
    --keep) keep=1; shift ;;
    -h|--help) sed -n '2,17p' "$0"; exit 0 ;;
    *) log error invalid_argument arg="$1"; exit 2 ;;
  esac
done

REQUIRED_TABLES=(goose_db_version "user" organization legal_document legal_acceptance audit_log job
  channel post_version "order" placement publication creative ledger_account ledger_transaction
  ledger_entry hold invoice bank_transaction media_file rocketwork_contractor payout)
PORT=${RESTORE_CHECK_PORT:-5433}

start=$(now_ms)
mc_setup
root=$(backup_root)
work=$(mktemp -d "$BACKUP_TMP_DIR/restore-check.XXXXXX")
data="$work/data"
failures=0

stop_cluster() {
  if [[ -f $data/postmaster.pid ]]; then
    pg_ctl -D "$data" -m immediate -w stop >/dev/null 2>&1 || true
  fi
}
on_exit() {
  stop_cluster
  if ((keep)); then
    log info restore_check_kept dir="$work"
  else
    rm -rf -- "$work"
  fi
  cleanup_run
}
trap on_exit EXIT
trap 'fail_job restore_check restore_check_error "line ${LINENO}: ${BASH_COMMAND}"' ERR

step() {
  local name=$1 ok=$2
  shift 2
  if [[ $ok == true ]]; then
    log info restore_check_step step="$name" ok=true "$@"
  else
    failures=$((failures + 1))
    log error restore_check_step step="$name" ok=false "$@"
  fi
}

q() {
  psql -X -At -v ON_ERROR_STOP=1 -h "$work" -p "$PORT" -U postgres -d maxim -c "$1"
}

# 1. Какую копию проверяем.
check_age=1
if [[ -z $key ]]; then
  key=$(m ls --recursive --json "$root/db/dump/" 2>/dev/null \
    | jq -r 'select(.type == "file") | .key | select(endswith(".json"))' \
    | sed 's/\.json$//' | sort -t/ -k4 | tail -n 1)
  [[ -n $key ]] && key="db/dump/$key"
else
  check_age=0
fi
if [[ -z $key ]]; then
  fail_job restore_check restore_check_no_dump "no logical backup found under ${root#backup/}/db/dump"
fi
log info restore_check_started dump_key="$key"

m cp --quiet "$root/$key.json" "$work/manifest.json" >/dev/null
m cp --quiet "$root/$key.dump" "$work/maxim.dump" >/dev/null
m cp --quiet "$root/$key.dump.sha256" "$work/maxim.dump.sha256" >/dev/null
download_ms=$(($(now_ms) - start))

want_sha=$(cut -d' ' -f1 "$work/maxim.dump.sha256")
got_sha=$(sha256sum "$work/maxim.dump" | cut -d' ' -f1)
if [[ $want_sha != "$got_sha" ]]; then
  step checksum false expected="$want_sha" actual="$got_sha"
  fail_job restore_check restore_check_checksum "checksum mismatch for $key"
fi
step checksum true sha256="$got_sha" download_ms="$download_ms"

created=$(jq -r '.created_at' "$work/manifest.json")
created_unix=$(date -u -d "${created:0:4}-${created:4:2}-${created:6:2} ${created:9:2}:${created:11:2}:${created:13:2}" +%s)
age_h=$((($(date +%s) - created_unix) / 3600))
if ((check_age)); then
  max_h=${BACKUP_MAX_AGE_HOURS:-26}
  if ((age_h > max_h)); then
    step dump_age false age_h="$age_h" max_h="$max_h"
  else
    step dump_age true age_h="$age_h" max_h="$max_h"
  fi
fi

# 2. Временный кластер и восстановление.
restore_start=$(now_ms)
initdb -D "$data" -U postgres --auth=trust --encoding=UTF8 --locale=C --no-sync >/dev/null
pg_ctl -D "$data" -w -t 120 -l "$work/postgres.log" \
  -o "-p $PORT -c listen_addresses='' -c unix_socket_directories='$work' -c fsync=off -c full_page_writes=off" start >/dev/null
createdb -h "$work" -p "$PORT" -U postgres maxim
pg_restore -h "$work" -p "$PORT" -U postgres -d maxim --no-owner --no-privileges --exit-on-error -j 2 "$work/maxim.dump"
restore_ms=$(($(now_ms) - restore_start))
step restore true restore_ms="$restore_ms"

# 3. Версия миграций.
want_version=$(jq -r '.migration_version' "$work/manifest.json")
got_version=$(migration_version -h "$work" -p "$PORT" -U postgres -d maxim)
if [[ $want_version == "$got_version" ]]; then
  step migration_version true version="$got_version"
else
  step migration_version false expected="$want_version" actual="$got_version"
fi

# 4. Обязательные таблицы (до первой миграции схемы нет — это ошибка проверки).
missing=()
for t in "${REQUIRED_TABLES[@]}"; do
  [[ $(q "SELECT to_regclass('public.\"$t\"') IS NOT NULL") == t ]] || missing+=("$t")
done
if ((${#missing[@]} == 0)); then
  step tables true count="${#REQUIRED_TABLES[@]}"
else
  step tables false missing="${missing[*]}"
fi

# 5. Сходимость журнала проводок (06 §2.4).
if [[ $(q "SELECT to_regclass('public.ledger_account') IS NOT NULL") == t ]]; then
  sides=$(q "SELECT
      coalesce(sum(balance) FILTER (WHERE kind IN ('bank', 'rocketwork_deposit', 'payout_fees')), 0)::bigint || ' ' ||
      coalesce(sum(balance) FILTER (WHERE kind NOT IN ('bank', 'rocketwork_deposit', 'payout_fees')), 0)::bigint
    FROM ledger_account")
  assets=${sides% *}
  liabilities=${sides#* }
  mismatches=$(q "WITH sums AS (
      SELECT a.balance,
        ((CASE WHEN a.kind IN ('bank', 'rocketwork_deposit', 'payout_fees') THEN 1 ELSE -1 END) * (
          coalesce((SELECT sum(amount) FROM ledger_entry WHERE debit_account_id = a.id), 0) -
          coalesce((SELECT sum(amount) FROM ledger_entry WHERE credit_account_id = a.id), 0)))::bigint AS computed
      FROM ledger_account a)
    SELECT count(*) FROM sums WHERE balance <> computed")
  negatives=$(q "SELECT count(*) FROM ledger_account
    WHERE kind NOT IN ('bank', 'rocketwork_deposit', 'payout_fees') AND balance < 0")
  entries=$(q "SELECT count(*) FROM ledger_entry")
  if [[ $assets == "$liabilities" && $mismatches == 0 && $negatives == 0 ]]; then
    step ledger true assets="$assets" liabilities="$liabilities" entries="$entries"
  else
    step ledger false assets="$assets" liabilities="$liabilities" cache_mismatches="$mismatches" negative_liabilities="$negatives"
  fi
fi

# 6. Файлы медиатеки в копии объектов.
objects_checked=0
if [[ -n ${S3_BUCKET_PRIVATE:-} && $(q "SELECT to_regclass('public.media_file') IS NOT NULL") == t ]]; then
  mapfile -t keys < <(q "SELECT storage_key FROM media_file WHERE purged_at IS NULL ORDER BY created_at DESC LIMIT 50")
  mirrored=$(m ls --json "$root/objects/$S3_BUCKET_PRIVATE/" 2>/dev/null | head -c 1 || true)
  if ((${#keys[@]} == 0)); then
    log info restore_check_step step=objects ok=true objects_checked=0 note="no media files in the backup"
  elif [[ -z $mirrored ]]; then
    step objects false reason=no_object_mirror media_files="${#keys[@]}"
  else
    absent=()
    for k in "${keys[@]}"; do
      if m stat --no-list "$root/objects/$S3_BUCKET_PRIVATE/$k" >/dev/null 2>&1; then
        objects_checked=$((objects_checked + 1))
      else
        absent+=("$k")
      fi
    done
    if ((${#absent[@]} == 0)); then
      step objects true objects_checked="$objects_checked"
    else
      step objects false objects_checked="$objects_checked" absent="${#absent[@]}" first_absent="${absent[0]}"
    fi
  fi
else
  log warn restore_check_step step=objects ok=true note="S3_BUCKET_PRIVATE is not set: object check skipped"
fi

duration_ms=$(($(now_ms) - start))
if ((failures > 0)); then
  log error restore_check_result ok=false failures="$failures" dump_key="$key" dump_age_h="$age_h" \
    migration_version="$got_version" objects_checked="$objects_checked" duration_ms="$duration_ms"
  state_write restore_check false dump_key="$key" failures="$failures"
  notify_failure restore_check "restore check of $key failed: $failures check(s), see logs"
  exit 1
fi
state_write restore_check true dump_key="$key" duration_ms="$duration_ms"
log info restore_check_result ok=true dump_key="$key" dump_age_h="$age_h" migration_version="$got_version" \
  objects_checked="$objects_checked" restore_ms="$restore_ms" duration_ms="$duration_ms"
