# shellcheck shell=bash
# Общее для скриптов резервного копирования (контейнер backup, deploy/app/README.md).
#
# Переменные окружения (compose задаёт из stack.env и app.env):
#   PGHOST, PGPORT, PGUSER=maxim_backup, PGPASSWORD, PGDATABASE — база-источник;
#   BACKUP_S3_ENDPOINT, BACKUP_S3_ACCESS_KEY, BACKUP_S3_SECRET_KEY, BACKUP_S3_BUCKET,
#   BACKUP_S3_USE_SSL, BACKUP_PREFIX — корзина копий;
#   S3_ENDPOINT, S3_ACCESS_KEY, S3_SECRET_KEY, S3_USE_SSL, S3_BUCKET_PRIVATE, S3_BUCKET_PUBLIC —
#   хранилище приложения (копия объектов, проверка восстановления);
#   SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASSWORD, SMTP_TLS, MAIL_FROM, ALERT_EMAILS — письмо
#   о сбое.
# Ключи и пароли в журнал не пишутся.

# shellcheck source=deploy/scripts/lib.sh
. /usr/local/lib/maxim/lib.sh

BACKUP_STATE_DIR=${BACKUP_STATE_DIR:-/var/lib/backup-state}
BACKUP_TMP_DIR=${BACKUP_TMP_DIR:-/var/lib/backup-tmp}
WAL_ARCHIVE_DIR=${WAL_ARCHIVE_DIR:-/var/lib/postgresql/wal-archive}
BACKUP_ENV_FILE=${BACKUP_ENV_FILE:-/tmp/backup.env}
LOG_COMPONENT=${LOG_COMPONENT:-backup}

# cleanup_add <path> — удалить путь при выходе из скрипта (один trap на все пути).
CLEANUP_PATHS=()
cleanup_add() {
  CLEANUP_PATHS+=("$1")
}
cleanup_run() {
  local p
  for p in "${CLEANUP_PATHS[@]}"; do
    [[ -n $p ]] && rm -rf -- "$p"
  done
}
trap cleanup_run EXIT

# as_postgres "$@" — скрипты работают от postgres: запуск от root (cron, docker compose exec)
# повторяется через gosu с теми же аргументами.
as_postgres() {
  if [[ $(id -u) -eq 0 ]]; then
    exec gosu postgres "$0" "$@"
  fi
}

# backup_root — путь копий окружения в корзине: backup/<bucket>/<prefix>.
backup_root() {
  printf 'backup/%s/%s' "${BACKUP_S3_BUCKET:?BACKUP_S3_BUCKET is required}" "${BACKUP_PREFIX:?BACKUP_PREFIX is required}"
}

# s3_url <endpoint> <use_ssl> — адрес для mc alias.
s3_url() {
  local endpoint=$1 ssl=${2:-true}
  case "$endpoint" in
    http://*|https://*) printf '%s' "$endpoint" ;;
    *) if [[ $ssl == true ]]; then printf 'https://%s' "$endpoint"; else printf 'http://%s' "$endpoint"; fi ;;
  esac
}

# mc_setup — алиасы backup (корзина копий) и app (хранилище приложения, если задано).
# Конфигурация mc — во временном каталоге процесса: ключи не остаются на томе.
mc_setup() {
  MC_CONFIG_DIR=$(mktemp -d /tmp/mc.XXXXXX)
  export MC_CONFIG_DIR
  cleanup_add "$MC_CONFIG_DIR"
  mc --config-dir "$MC_CONFIG_DIR" alias set backup "$(s3_url "${BACKUP_S3_ENDPOINT:?BACKUP_S3_ENDPOINT is required}" "${BACKUP_S3_USE_SSL:-true}")" \
    "${BACKUP_S3_ACCESS_KEY:?}" "${BACKUP_S3_SECRET_KEY:?}" --api S3v4 >/dev/null
  if [[ -n ${S3_ENDPOINT:-} && -n ${S3_ACCESS_KEY:-} ]]; then
    mc --config-dir "$MC_CONFIG_DIR" alias set app "$(s3_url "$S3_ENDPOINT" "${S3_USE_SSL:-true}")" \
      "$S3_ACCESS_KEY" "${S3_SECRET_KEY:-}" --api S3v4 >/dev/null
  fi
}

# m — mc с конфигурацией процесса.
m() {
  mc --config-dir "$MC_CONFIG_DIR" --no-color "$@"
}

# state_write <kind> <ok> [key=value ...] — файл состояния для backup-health.sh.
state_write() {
  local kind=$1 ok=$2 file
  shift 2
  file="$BACKUP_STATE_DIR/$kind.json"
  {
    printf '{"kind":"%s","ok":%s,"at":"%s","at_unix":%s' "$kind" "$ok" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$(date +%s)"
    local kv
    for kv in "$@"; do
      printf ',"%s":"%s"' "$(json_escape "${kv%%=*}")" "$(json_escape "${kv#*=}")"
    done
    printf '}\n'
  } > "$file.tmp" && mv "$file.tmp" "$file"
  # Последний успех хранится отдельно: по нему считается возраст копии.
  if [[ $ok == true ]]; then
    cp "$file" "$BACKUP_STATE_DIR/$kind.ok.json"
  fi
}

# state_age <kind> — секунды с последнего успеха; пусто, если успеха не было.
state_age() {
  local file="$BACKUP_STATE_DIR/$1.ok.json" at
  [[ -f $file ]] || return 0
  at=$(jq -r '.at_unix' "$file" 2>/dev/null) || return 0
  [[ $at =~ ^[0-9]+$ ]] && printf '%s' $(($(date +%s) - at))
}

# notify_failure <kind> <text> — письмо дежурным (ALERT_EMAILS) о сбое, не чаще раза в час
# на вид. Без SMTP или адресатов — только журнал.
notify_failure() {
  local kind=$1 text=$2 marker="$BACKUP_STATE_DIR/notified-$1" now last scheme url msg rcpt
  now=$(date +%s)
  if [[ -f $marker ]]; then
    last=$(cat "$marker" 2>/dev/null || echo 0)
    if [[ $last =~ ^[0-9]+$ ]] && ((now - last < 3600)); then
      log info notify_skipped kind="$kind" reason=rate_limited
      return 0
    fi
  fi
  if [[ -z ${SMTP_HOST:-} || -z ${ALERT_EMAILS:-} || -z ${MAIL_FROM:-} ]]; then
    log warn notify_skipped kind="$kind" reason=no_smtp_or_recipients
    return 0
  fi
  case "${SMTP_TLS:-starttls}" in
    tls) scheme=smtps ;;
    *) scheme=smtp ;;
  esac
  url="$scheme://$SMTP_HOST:${SMTP_PORT:-25}"
  msg=$(mktemp)
  {
    printf 'From: %s\r\n' "$MAIL_FROM"
    printf 'To: %s\r\n' "$ALERT_EMAILS"
    printf 'Subject: [maxim %s] backup failed: %s\r\n' "${BACKUP_PREFIX:-?}" "$kind"
    printf 'Content-Type: text/plain; charset=utf-8\r\n\r\n'
    printf 'Backup job "%s" failed on %s at %s UTC.\r\n\r\n%s\r\n\r\nLogs: docker compose logs backup\r\n' \
      "$kind" "${BACKUP_PREFIX:-?}" "$(date -u +%Y-%m-%dT%H:%M:%S)" "$text"
  } > "$msg"
  local args=(--silent --show-error --max-time 30 --url "$url" --mail-from "$MAIL_FROM" --upload-file "$msg")
  [[ ${SMTP_TLS:-starttls} == starttls ]] && args+=(--ssl-reqd)
  [[ -n ${SMTP_USER:-} ]] && args+=(--user "$SMTP_USER:${SMTP_PASSWORD:-}")
  IFS=',' read -ra rcpt <<< "$ALERT_EMAILS"
  local r
  for r in "${rcpt[@]}"; do
    r=${r// /}
    [[ -n $r ]] && args+=(--mail-rcpt "$r")
  done
  if curl "${args[@]}" 2>"$msg.err"; then
    printf '%s' "$now" > "$marker"
    log info notify_sent kind="$kind" recipients="${#rcpt[@]}"
  else
    log error notify_failed kind="$kind" error="$(head -c 300 "$msg.err")"
  fi
  rm -f "$msg" "$msg.err"
}

# fail_job <kind> <event> <text> — ERROR в журнал, состояние, письмо; код 1.
fail_job() {
  local kind=$1 event=$2 text=$3
  log error "$event" kind="$kind" error="$text"
  state_write "$kind" false error="$text"
  notify_failure "$kind" "$text"
  exit 1
}

# migration_version — версия схемы (goose); пусто, если миграций ещё не было.
migration_version() {
  psql -X -At -v ON_ERROR_STOP=1 "$@" -c \
    "SELECT CASE WHEN to_regclass('public.goose_db_version') IS NULL THEN '' ELSE (SELECT coalesce(max(version_id)::text, '') FROM goose_db_version WHERE is_applied) END"
}
