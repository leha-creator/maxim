-- API администратора (этап «API администратора и отчётность ОРД»): переписка по спору (R-9),
-- запросы второго подтверждения ручных денежных операций (R-2), блокировка и обезличивание
-- пользователя (R-12, R-26). Архив категорий каналов — существующий channel_category.is_active.

-- +goose Up
-- R-9: сообщение администратора стороне спора или обеим; ответ стороны — администратору.
CREATE TABLE dispute_message (
    id             uuid PRIMARY KEY,
    dispute_id     uuid NOT NULL REFERENCES dispute (id),
    author_type    text NOT NULL CHECK (author_type IN ('admin', 'advertiser', 'blogger')),
    author_user_id uuid NOT NULL REFERENCES "user" (id),
    -- Адресат — только у сообщения администратора.
    recipient      text CHECK (recipient IN ('advertiser', 'blogger', 'both')),
    text           text NOT NULL CHECK (char_length(text) BETWEEN 1 AND 2000),
    created_at     timestamptz NOT NULL,
    CONSTRAINT dispute_message_recipient CHECK ((author_type = 'admin') = (recipient IS NOT NULL))
);

CREATE INDEX dispute_message_dispute_idx ON dispute_message (dispute_id, created_at);

-- R-2: операция выше порога ждёт подтверждения другим senior; исполняется в той же транзакции,
-- что и решение. payload — что исполнить (счета, выплата, банковский документ).
CREATE TABLE admin_approval (
    id              uuid PRIMARY KEY,
    kind            text NOT NULL CHECK (kind IN ('ledger_manual', 'deposit_topup', 'payout_mark_paid')),
    payload         jsonb NOT NULL,
    amount          bigint NOT NULL CHECK (amount > 0),
    summary         text NOT NULL CHECK (char_length(summary) BETWEEN 1 AND 500),
    reason          text NOT NULL CHECK (char_length(reason) BETWEEN 1 AND 1000),
    idempotency_key text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 200),
    requested_by    uuid NOT NULL REFERENCES "user" (id),
    status          text NOT NULL CHECK (status IN ('pending', 'approved', 'rejected', 'expired')),
    decided_by      uuid REFERENCES "user" (id),
    decided_at      timestamptz,
    decision_reason text CHECK (char_length(decision_reason) <= 1000),
    result_id       uuid,
    expires_at      timestamptz NOT NULL,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    CONSTRAINT admin_approval_idempotency_key_key UNIQUE (idempotency_key),
    -- Решение — с автором и временем; подтверждает другой администратор (автор может только отозвать).
    CONSTRAINT admin_approval_decided CHECK (
        (status IN ('approved', 'rejected')) = (decided_by IS NOT NULL AND decided_at IS NOT NULL)),
    CONSTRAINT admin_approval_other_admin CHECK (status <> 'approved' OR decided_by <> requested_by),
    CONSTRAINT admin_approval_result CHECK ((status = 'approved') = (result_id IS NOT NULL))
);

CREATE INDEX admin_approval_status_idx ON admin_approval (status, created_at);

-- R-12, R-26: блокировка администратором и обезличивание по запросу субъекта.
ALTER TABLE "user" ADD COLUMN blocked_at timestamptz;
ALTER TABLE "user" ADD COLUMN blocked_reason text CHECK (char_length(blocked_reason) <= 500);
ALTER TABLE "user" ADD COLUMN anonymized_at timestamptz;
ALTER TABLE "user" ADD CONSTRAINT user_blocked_at CHECK (blocked_at IS NULL OR status = 'blocked');

-- Поиск организаций администратором по наименованию (adm/organizations).
CREATE INDEX organization_name_lower_idx ON organization (lower(name));

-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON dispute_message FROM %I', app_role);
    EXECUTE format('REVOKE DELETE, TRUNCATE ON admin_approval FROM %I', app_role);
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP INDEX organization_name_lower_idx;
ALTER TABLE "user" DROP CONSTRAINT user_blocked_at;
ALTER TABLE "user" DROP COLUMN anonymized_at;
ALTER TABLE "user" DROP COLUMN blocked_reason;
ALTER TABLE "user" DROP COLUMN blocked_at;
DROP TABLE admin_approval;
DROP TABLE dispute_message;
