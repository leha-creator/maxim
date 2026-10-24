#!/usr/bin/env bash
# Первый запуск кластера (пустой PGDATA): роли приложения и резервного копирования.
# Владелец схемы — POSTGRES_USER (его создаёт образ postgres); права роли приложения выдаёт
# миграция 00002_app_role_grants.sql. Пароли передаются переменными psql, не текстом команд.
set -Eeuo pipefail

: "${POSTGRES_APP_PASSWORD:?POSTGRES_APP_PASSWORD is required}"
: "${POSTGRES_BACKUP_PASSWORD:?POSTGRES_BACKUP_PASSWORD is required}"
app_role=${POSTGRES_APP_ROLE:-maxim_app}

echo "{\"level\":\"info\",\"component\":\"postgres-init\",\"event\":\"roles_create\",\"app_role\":\"${app_role}\",\"backup_role\":\"maxim_backup\"}"
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" \
  -v app_role="$app_role" -v app_password="$POSTGRES_APP_PASSWORD" \
  -v backup_password="$POSTGRES_BACKUP_PASSWORD" <<'SQL'
SELECT format('CREATE ROLE %I LOGIN PASSWORD %L', :'app_role', :'app_password') \gexec
-- Роль копирования: физическая копия (REPLICATION) и логическая (чтение всех данных).
SELECT format('CREATE ROLE maxim_backup LOGIN REPLICATION PASSWORD %L', :'backup_password') \gexec
GRANT pg_read_all_data TO maxim_backup;
SQL

# Физическая копия идёт по протоколу репликации: в pg_hba для него отдельная строка.
echo "host replication maxim_backup all scram-sha-256" >> "$PGDATA/pg_hba.conf"
echo '{"level":"info","component":"postgres-init","event":"roles_created"}'
