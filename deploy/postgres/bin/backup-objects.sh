#!/usr/bin/env bash
# Объектное хранилище (07 §8): версионирование корзин приложения и правило удаления прежних
# версий через 30 суток (однократно), затем копия корзин в <root>/objects/<bucket>/
# (mc mirror --overwrite, без удаления: удалённое у приложения остаётся в копии).
set -Eeuo pipefail
LOG_COMPONENT=backup-objects
# shellcheck source=deploy/postgres/bin/backup-lib.sh
. /usr/local/bin/backup-lib.sh
as_postgres "$@"
trap 'fail_job objects objects_failed "line ${LINENO}: ${BASH_COMMAND}"' ERR

start=$(now_ms)
mc_setup
if ! m alias list app >/dev/null 2>&1; then
  fail_job objects objects_no_source "S3_ENDPOINT and S3_ACCESS_KEY of the application storage are not set"
fi
root=$(backup_root)
total=0
for bucket in "${S3_BUCKET_PRIVATE:-}" "${S3_BUCKET_PUBLIC:-}"; do
  [[ -n $bucket ]] || continue
  marker="$BACKUP_STATE_DIR/versioning-$bucket"
  if [[ ! -f $marker ]]; then
    # Провайдер может не поддерживать версионирование или правила: WARN, копия всё равно идёт.
    if out=$(m version enable "app/$bucket" 2>&1); then
      log info versioning_enabled bucket="$bucket"
      if out=$(m ilm rule add --noncurrent-expire-days 30 "app/$bucket" 2>&1); then
        log info lifecycle_rule_added bucket="$bucket" noncurrent_expire_days=30
      else
        log warn lifecycle_rule_failed bucket="$bucket" error="${out:0:300}"
      fi
      touch "$marker"
    else
      log warn versioning_unsupported bucket="$bucket" error="${out:0:300}"
    fi
  fi
  b_start=$(now_ms)
  m mirror --overwrite --quiet "app/$bucket" "$root/objects/$bucket" >/dev/null
  count=$(m ls --recursive --json "$root/objects/$bucket/" 2>/dev/null | jq -s 'map(select(.type == "file")) | length')
  total=$((total + count))
  log info objects_mirrored bucket="$bucket" objects="$count" duration_ms="$(($(now_ms) - b_start))"
done
state_write objects true objects="$total"
log info objects_done objects="$total" duration_ms="$(($(now_ms) - start))"
