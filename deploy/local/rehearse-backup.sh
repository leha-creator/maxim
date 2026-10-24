#!/usr/bin/env bash
# Репетиция резервного копирования и восстановления (07 §8, deploy/app/README.md).
#
#   deploy/local/rehearse-backup.sh [--keep] [--skip-build]
#
# Проект compose maxim-rehearsal-backup: PostgreSQL из maxim-postgres:rehearsal, MinIO,
# контейнер backup; схема — миграциями образа бэкенда maxim:rehearsal-v1. Проверяется:
#   1. физическая копия, WAL и логическая копия выгружаются, копия объектов делается;
#   2. restore-check.sh восстанавливает последнюю логическую копию и проходит все проверки;
#   3. restore-pitr.sh на момент T: строки, записанные до T, есть, после T — нет;
#   4. испорченная контрольная сумма копии — restore-check.sh завершается ошибкой.
# В конце `docker compose down -v` только своего проекта (--keep — оставить стек).
set -Eeuo pipefail

# pwd -W — путь вида D:/… в Git Bash на Windows (docker его понимает без преобразования).
SCRIPT_DIR=$(cd "$(dirname "$0")" && { pwd -W 2>/dev/null || pwd; })
ROOT=$(cd "$SCRIPT_DIR/../.." && { pwd -W 2>/dev/null || pwd; })
LOG_COMPONENT=rehearse-backup
# shellcheck source=deploy/scripts/lib.sh
. "$ROOT/deploy/scripts/lib.sh"

KEEP=0
SKIP_BUILD=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --keep) KEEP=1; shift ;;
    --skip-build) SKIP_BUILD=1; shift ;;
    -h|--help) sed -n '2,12p' "$0"; exit 0 ;;
    *) log error invalid_argument arg="$1"; exit 2 ;;
  esac
done

PROJECT=maxim-rehearsal-backup
STATE="$SCRIPT_DIR/.state/backup"
APP="$STATE/app"
TAG=rehearsal-v1

app() {
  MSYS2_ARG_CONV_EXCL='*' docker compose --project-directory "$APP" -f "$APP/compose.yml" -f "$APP/compose.rehearsal.yml" \
    --env-file "$APP/stack.env" "$@"
}
# sql <query> — запрос владельцем схемы к рабочей базе.
sql() { app exec -T postgres psql -X -v ON_ERROR_STOP=1 -U maxim -d maxim -At -c "$1" | tr -d '\r'; }
# in_backup <command...> — команда в контейнере backup (скрипты сами переходят на postgres).
in_backup() { app exec -T backup "$@"; }

down() {
  [[ -f $APP/compose.yml ]] && app --profile tools --profile slot-a --profile slot-b --profile mailpit down -v --remove-orphans >/dev/null 2>&1 || true
}
cleanup() {
  local rc=$?
  if ((rc != 0)); then
    log error rehearsal_failed exit_code="$rc"
    [[ -f $APP/compose.yml ]] && app logs --no-color --tail=30 backup 2>/dev/null || true
  fi
  if ((KEEP == 0)); then
    down
    rm -rf "$STATE"
  else
    log info rehearsal_kept app_dir="$APP" project="$PROJECT"
  fi
  exit "$rc"
}
trap cleanup EXIT

started=$(now_ms)
down
rm -rf "$STATE"

if ((SKIP_BUILD == 0)); then
  log info rehearsal_step step=build
  docker build -q --build-arg "VERSION=$TAG" -t "maxim:$TAG" "$ROOT" >/dev/null
  docker build -q -f "$ROOT/deploy/postgres/Dockerfile" -t maxim-postgres:rehearsal "$ROOT/deploy" >/dev/null
fi

mkdir -p "$APP"
cp "$ROOT/deploy/app/compose.yml" "$SCRIPT_DIR/compose.rehearsal.yml" "$APP/"
cp "$SCRIPT_DIR/rehearsal.app.env" "$APP/app.env"
sed -e "s/^COMPOSE_PROJECT_NAME=.*/COMPOSE_PROJECT_NAME=$PROJECT/" -e "s/^EDGE_ALIAS_PREFIX=.*/EDGE_ALIAS_PREFIX=$PROJECT/" \
  "$SCRIPT_DIR/rehearsal.stack.env" > "$APP/stack.env"
printf 'MAXIM_TAG_NEW=%s\n' "$TAG" >> "$APP/stack.env"

log info rehearsal_step step=stack_up
app up -d --quiet-pull --wait postgres backup mailpit >/dev/null
app run --rm -T migrate >/dev/null

# Данные: пользователь, организация, файл медиатеки с объектом в хранилище и таблица меток
# времени для проверки восстановления на момент.
log info rehearsal_step step=seed
sql "INSERT INTO \"user\" (id, email, status, created_at, updated_at)
       VALUES ('00000000-0000-4000-8000-000000000001', 'rehearsal@maxim.local', 'active', now(), now());
     INSERT INTO organization (id, legal_type, inn, name, contact_name, phone, status, created_at, updated_at)
       VALUES ('00000000-0000-4000-8000-000000000002', 'legal_entity', '7700000000', 'ООО «Репетиция»',
               'Иван Иванов', '+79990000000', 'active', now(), now());
     INSERT INTO media_file (id, organization_id, storage_key, mime, size_bytes, width, height, sha256,
                             original_name, created_by, created_at)
       VALUES ('00000000-0000-4000-8000-000000000003', '00000000-0000-4000-8000-000000000002',
               'media/rehearsal/one.png', 'image/png', 4, 1, 1, 'rehearsal', 'one.png',
               '00000000-0000-4000-8000-000000000001', now());
     CREATE TABLE rehearsal_marker (n integer PRIMARY KEY, at timestamptz NOT NULL DEFAULT clock_timestamp());
     INSERT INTO rehearsal_marker (n) VALUES (1);" >/dev/null
# shellcheck disable=SC2016 # переменные раскрываются в контейнере
in_backup sh -c 'set -e; d=$(mktemp -d); mc --config-dir "$d" alias set app "http://$S3_ENDPOINT" "$S3_ACCESS_KEY" "$S3_SECRET_KEY" >/dev/null;
  printf "png!" | mc --config-dir "$d" pipe "app/$S3_BUCKET_PRIVATE/media/rehearsal/one.png" >/dev/null; rm -rf "$d"'

log info rehearsal_step step=base_backup
in_backup /usr/local/bin/backup-base.sh

sql "INSERT INTO rehearsal_marker (n) VALUES (2)" >/dev/null
sleep 2
T=$(sql "SELECT to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD HH24:MI:SS')")
sleep 2
sql "INSERT INTO rehearsal_marker (n) VALUES (3)" >/dev/null
log info rehearsal_step step=target_time target_time="$T"

# WAL с записью после T должен оказаться в корзине: закрыть сегмент, дождаться архива, выгрузить.
seg=$(sql "SELECT pg_walfile_name(pg_switch_wal())")
for _ in $(seq 1 60); do
  last=$(sql "SELECT coalesce(last_archived_wal, '') FROM pg_stat_archiver")
  [[ -n $last && ! $last < $seg ]] && break
  sleep 1
done
[[ -n $last && ! $last < $seg ]] || { log error wal_not_archived segment="$seg" last="$last"; exit 1; }
in_backup /usr/local/bin/wal-upload.sh

log info rehearsal_step step=dump_and_objects
in_backup /usr/local/bin/backup-dump.sh
in_backup /usr/local/bin/backup-objects.sh

log info rehearsal_step step=restore_check
t=$(now_ms)
in_backup /usr/local/bin/restore-check.sh
restore_ms=$(($(now_ms) - t))

log info rehearsal_step step=restore_pitr target_time="$T"
t=$(now_ms)
in_backup /usr/local/bin/restore-pitr.sh --target-time "$T" --pgdata /var/lib/backup-tmp/pitr/data --port 5434 --keep-running
pitr_ms=$(($(now_ms) - t))
rows=$(app exec -T -u postgres backup psql -X -At -h /var/lib/backup-tmp/pitr -p 5434 -U maxim -d maxim \
  -c "SELECT string_agg(n::text, ',' ORDER BY n) FROM rehearsal_marker" | tr -d '\r')
app exec -T -u postgres backup sh -c 'pg_ctl -D /var/lib/backup-tmp/pitr/data -m fast -w stop >/dev/null; rm -rf /var/lib/backup-tmp/pitr'
if [[ $rows != "1,2" ]]; then
  log error rehearsal_pitr_mismatch expected="1,2" got="$rows" target_time="$T"
  exit 1
fi

log info rehearsal_step step=corrupt_checksum
# shellcheck disable=SC2016 # переменные раскрываются в контейнере
in_backup sh -c 'set -e; d=$(mktemp -d); key=$(jq -r .key /var/lib/backup-state/dump.ok.json);
  mc --config-dir "$d" alias set backup "http://$BACKUP_S3_ENDPOINT" "$BACKUP_S3_ACCESS_KEY" "$BACKUP_S3_SECRET_KEY" >/dev/null;
  printf "%064d  corrupted.dump\n" 0 | mc --config-dir "$d" pipe "backup/$key.sha256" >/dev/null; rm -rf "$d"'
if out=$(in_backup /usr/local/bin/restore-check.sh 2>&1); then
  log error rehearsal_corruption_not_detected
  exit 1
fi
if ! grep -q '"step":"checksum","ok":false' <<< "$out"; then
  log error rehearsal_corruption_wrong_reason output="$(tail -c 1000 <<< "$out")"
  exit 1
fi

# Очистка со сроком 0 суток: из двух логических копий остаётся новая, единственная физическая
# остаётся, WAL до её начального сегмента удаляется, сам начальный сегмент — нет.
log info rehearsal_step step=prune
sleep 1
in_backup /usr/local/bin/backup-dump.sh >/dev/null
in_backup env BACKUP_DUMP_RETENTION_DAYS=0 BACKUP_BASE_RETENTION_DAYS=0 /usr/local/bin/backup-prune.sh
# shellcheck disable=SC2016 # переменные раскрываются в контейнере
listing=$(in_backup bash -c '. /usr/local/bin/backup-lib.sh; mc_setup; r=$(backup_root);
  echo "dumps=$(m ls -r --json "$r/db/dump/" | jq -r "select(.key|endswith(\".dump\"))|.key" | wc -l)";
  echo "bases=$(m ls --json "$r/db/base/" | jq -r .key | wc -l)";
  start=$(m cat "$r/db/base/$(m ls --json "$r/db/base/" | jq -r .key | head -n 1)manifest.json" | jq -r .start_segment);
  echo "start=$start"; m ls --json "$r/db/wal/" | jq -r .key | sed "s/^/wal=/"' | tr -d '\r')
start=$(sed -n 's/^start=//p' <<< "$listing")
if ! grep -qx 'dumps=1' <<< "$listing" || ! grep -qx 'bases=1' <<< "$listing" \
  || ! grep -qx "wal=$start" <<< "$listing" \
  || [[ -n $(sed -n 's/^wal=\([0-9A-F]\{24\}\)$/\1/p' <<< "$listing" | awk -v s="$start" '$0 < s') ]]; then
  log error rehearsal_prune_mismatch listing="$(tr '\n' ' ' <<< "$listing")"
  exit 1
fi

log info rehearsal_result ok=true restore_check_ms="$restore_ms" restore_pitr_ms="$pitr_ms" pitr_rows="$rows" \
  target_time="$T" duration_ms="$(($(now_ms) - started))"
