-- Локальная разработка: роль приложения. Миграции выполняет владелец (POSTGRES_USER),
-- права роли выдаёт миграция 00002_app_role_grants.sql.
CREATE ROLE maxim_app LOGIN PASSWORD 'maxim_app';
