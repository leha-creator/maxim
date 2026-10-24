#!/usr/bin/env bash
# Точка входа контейнера backup (от root): расписание crond из BACKUP_*_CRON и цикл выгрузки
# WAL каждые BACKUP_WAL_INTERVAL секунд. Задания сами переходят на пользователя postgres.
# SIGTERM — последняя выгрузка WAL и выход.
set -Eeuo pipefail
LOG_COMPONENT=backup-agent
# shellcheck source=deploy/postgres/bin/backup-lib.sh
. /usr/local/bin/backup-lib.sh

interval=${BACKUP_WAL_INTERVAL:-30}
if [[ ! $interval =~ ^[1-9][0-9]*$ ]]; then
  log error invalid_config key=BACKUP_WAL_INTERVAL
  exit 1
fi
for v in BACKUP_S3_ENDPOINT BACKUP_S3_ACCESS_KEY BACKUP_S3_SECRET_KEY BACKUP_S3_BUCKET BACKUP_PREFIX PGHOST PGUSER PGPASSWORD; do
  if [[ -z ${!v:-} ]]; then
    log error invalid_config key="$v" reason=required
    exit 1
  fi
done

# Окружение для заданий crond (он его не передаёт): только нужные переменные, права 600.
umask 077
: > "$BACKUP_ENV_FILE"
while IFS= read -r name; do
  if [[ $name =~ ^(PG[A-Z]*|POSTGRES_USER|POSTGRES_DB|BACKUP_[A-Z0-9_]+|S3_[A-Z0-9_]+|SMTP_[A-Z]+|MAIL_FROM|ALERT_EMAILS|TZ)$ ]]; then
    value=${!name}
    q="'\\''"
    printf "export %s='%s'\n" "$name" "${value//\'/$q}" >> "$BACKUP_ENV_FILE"
  fi
done < <(compgen -e)
chown postgres:postgres "$BACKUP_ENV_FILE"

install -d -o postgres -g postgres -m 0700 "$BACKUP_STATE_DIR" "$BACKUP_TMP_DIR"
date +%s > "$BACKUP_STATE_DIR/agent-started"
chown postgres:postgres "$BACKUP_STATE_DIR/agent-started"

base_cron=${BACKUP_BASE_CRON:-30 0 * * *}
dump_cron=${BACKUP_DUMP_CRON:-0 1 * * *}
objects_cron=${BACKUP_OBJECTS_CRON:-0 2 * * *}
prune_cron=${BACKUP_PRUNE_CRON:-0 3 * * *}
restore_check_cron=${BACKUP_RESTORE_CHECK_CRON:-0 4 * * 0}
mkdir -p /etc/crontabs
sed -e "s|@BASE_CRON@|${base_cron}|" -e "s|@DUMP_CRON@|${dump_cron}|" \
  -e "s|@OBJECTS_CRON@|${objects_cron}|" -e "s|@PRUNE_CRON@|${prune_cron}|" \
  -e "s|@RESTORE_CHECK_CRON@|${restore_check_cron}|" \
  /etc/maxim/backup.crontab | grep -v '^#' > /etc/crontabs/root
crond -b -c /etc/crontabs -l 8
log info agent_started wal_interval_s="$interval" prefix="$BACKUP_PREFIX" base_cron="$base_cron" \
  dump_cron="$dump_cron" objects_cron="$objects_cron" prune_cron="$prune_cron" \
  restore_check_cron="$restore_check_cron"

stop=0
trap 'stop=1' TERM INT
while ((stop == 0)); do
  /usr/local/bin/wal-upload.sh || true
  # sleep в фоне: SIGTERM прерывает ожидание сразу.
  sleep "$interval" &
  wait $! || true
done
log info agent_stopping note="final WAL upload"
/usr/local/bin/wal-upload.sh || true
log info agent_stopped
