-- Права роли приложения (DB_APP_ROLE). Миграции выполняет владелец схемы,
-- приложение работает под отдельной ролью без права менять схему.
-- Неизменяемые таблицы (04 §1): только SELECT и INSERT — запрет закреплён правами (07 §4).
-- Роль создаёт администратор базы (в разработке — compose.dev.yml), миграция её не создаёт.
-- Имя роли приходит параметром сессии maxim.app_role: его выставляет `maxim migrate`.
--
-- Новая таблица получает полные права через ALTER DEFAULT PRIVILEGES ниже.
-- Миграция неизменяемой таблицы обязана сразу отозвать у роли UPDATE и DELETE.

-- +goose Up
-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('GRANT USAGE ON SCHEMA public TO %I', app_role);
    EXECUTE format('GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO %I', app_role);
    EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON legal_acceptance, audit_log FROM %I', app_role);
    EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO %I', app_role);
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('ALTER DEFAULT PRIVILEGES IN SCHEMA public REVOKE SELECT, INSERT, UPDATE, DELETE ON TABLES FROM %I', app_role);
    EXECUTE format('REVOKE ALL ON ALL TABLES IN SCHEMA public FROM %I', app_role);
    EXECUTE format('REVOKE USAGE ON SCHEMA public FROM %I', app_role);
END
$$;
-- +goose StatementEnd
