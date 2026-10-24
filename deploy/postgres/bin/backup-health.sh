#!/usr/bin/env bash
# HEALTHCHECK контейнера backup: цикл выгрузки WAL жив и успешен (проход не старше 5 минут),
# логическая копия не старше BACKUP_MAX_AGE_HOURS (после первых суток работы).
set -Euo pipefail
LOG_COMPONENT=backup-health
# shellcheck source=deploy/postgres/bin/backup-lib.sh
. /usr/local/bin/backup-lib.sh
trap - EXIT

now=$(date +%s)
max_age=$((${BACKUP_MAX_AGE_HOURS:-26} * 3600))
wal="$BACKUP_STATE_DIR/wal.json"
if [[ ! -f $wal ]]; then
  echo "wal upload has not run yet"
  exit 1
fi
wal_at=$(jq -r '.at_unix' "$wal")
wal_ok=$(jq -r '.ok' "$wal")
if ((now - wal_at > 300)) || [[ $wal_ok != true ]]; then
  echo "wal upload stale or failing (ok=${wal_ok})"
  exit 1
fi
started=$(cat "$BACKUP_STATE_DIR/agent-started" 2>/dev/null || echo "$now")
age=$(state_age dump)
if [[ -z $age ]]; then
  if ((now - started > max_age)); then
    echo "no successful dump for $(((now - started) / 3600)) h"
    exit 1
  fi
elif ((age > max_age)); then
  echo "last successful dump is $((age / 3600)) h old"
  exit 1
fi
echo ok
