#!/usr/bin/env bash
# Проверка совместимости миграций с предыдущей версией кода (07 §8, выпуск без простоя).
#
#   check-migrations.sh [<base-ref>]     по умолчанию origin/master
#
# Миграции применяются до запуска новой версии, пока работает старая, поэтому раздел
# «+goose Up» новой миграции не должен ломать старый код («расширение»). Опасные операции —
# DROP TABLE, DROP COLUMN, RENAME, ALTER COLUMN … TYPE, SET NOT NULL, DROP CONSTRAINT —
# допустимы только в миграции «сжатия», выпускаемой после того, как ни одна работающая версия
# их не использует; такая миграция помечается строкой «-- release: contract».
# Изменение или удаление уже существующей миграции — ошибка всегда.
set -Eeuo pipefail

SCRIPT_DIR=$(cd "$(dirname "$0")" && pwd)
LOG_COMPONENT=check-migrations
# shellcheck source=deploy/scripts/lib.sh
. "$SCRIPT_DIR/lib.sh"

base=${1:-origin/master}
root=$(git -C "$SCRIPT_DIR" rev-parse --show-toplevel)
cd "$root"
if ! git rev-parse --verify --quiet "$base^{commit}" >/dev/null; then
  log error base_ref_missing base="$base" hint="fetch the base branch or pass a ref"
  exit 2
fi

errors=0
checked=0
while IFS=$'\t' read -r status path rest; do
  case "$path" in
    migrations/*.sql) ;;
    *) continue ;;
  esac
  case "$status" in
    A) ;;
    R*|C*)
      log error migration_changed file="$rest" from="$path" status="$status" hint="existing migrations are immutable; add a new one"
      errors=$((errors + 1)); continue ;;
    *)
      log error migration_changed file="$path" status="$status" hint="existing migrations are immutable; add a new one"
      errors=$((errors + 1)); continue ;;
  esac
  checked=$((checked + 1))
  if grep -qiE '^[[:space:]]*--[[:space:]]*release:[[:space:]]*contract[[:space:]]*$' "$path"; then
    log warn migration_contract file="$path" note="contract migration: release only after no running version uses the dropped objects"
    continue
  fi
  # Только раздел Up: от «-- +goose Up» до «-- +goose Down»; комментарии вне счёта.
  while IFS=: read -r line text; do
    log error migration_incompatible file="$path" line="$line" statement="$(printf '%s' "$text" | sed 's/^[[:space:]]*//' | cut -c1-200)" \
      hint="split into expand now and contract later, or mark the file with '-- release: contract'"
    errors=$((errors + 1))
  done < <(awk '
      /^[[:space:]]*--[[:space:]]*\+goose[[:space:]]+Up/   { up = 1; next }
      /^[[:space:]]*--[[:space:]]*\+goose[[:space:]]+Down/ { up = 0 }
      up {
        s = $0; sub(/--.*/, "", s); u = toupper(s)
        if (u ~ /DROP[[:space:]]+TABLE/ || u ~ /DROP[[:space:]]+COLUMN/ || u ~ /RENAME[[:space:]]/ ||
            u ~ /ALTER[[:space:]]+COLUMN[[:space:]]+[^[:space:]]+[[:space:]]+(SET[[:space:]]+DATA[[:space:]]+)?TYPE/ ||
            u ~ /SET[[:space:]]+NOT[[:space:]]+NULL/ || u ~ /DROP[[:space:]]+CONSTRAINT/)
          print NR ":" $0
      }' "$path")
done < <(git diff --name-status -M "$base...HEAD" -- migrations/)

if ((errors > 0)); then
  log error check_migrations_failed base="$base" checked="$checked" errors="$errors"
  exit 1
fi
log info check_migrations_ok base="$base" checked="$checked"
