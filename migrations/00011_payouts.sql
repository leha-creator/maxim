-- Выплаты блогерам через Рокет Ворк (04 §7, 05 §4, 06 §2.3, этап «Выплаты блогерам»):
-- исполнители Рокет Ворк и попытки выплат. Права роли приложения выдаёт ALTER DEFAULT PRIVILEGES
-- из 00002; строки выплат и исполнителей не удаляются (история денег, 06 §2.2).

-- +goose Up
-- 04 §7: зеркало исполнителя в Рокет Ворк. Для юрлица строка не создаётся (A-36).
CREATE TABLE rocketwork_contractor (
    id                uuid PRIMARY KEY,
    organization_id   uuid NOT NULL REFERENCES organization (id),
    external_id       text CHECK (char_length(external_id) BETWEEN 1 AND 200),
    status            text NOT NULL CHECK (status IN ('invited', 'pending', 'active', 'blocked')),
    -- Причина блокировки — наш код (R-2), например tax_status_lost.
    blocked_reason    text CHECK (char_length(blocked_reason) <= 200),
    -- Ссылка онбординга — для блогера; в журнал выше DEBUG не пишется (R-18).
    onboarding_url    text CHECK (char_length(onboarding_url) <= 2000),
    status_checked_at timestamptz,
    last_error        text CHECK (char_length(last_error) <= 1000),
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL,
    CONSTRAINT rocketwork_contractor_organization_id_key UNIQUE (organization_id),
    CONSTRAINT rocketwork_contractor_external_id_key UNIQUE (external_id),
    CONSTRAINT rocketwork_contractor_blocked_reason CHECK (status = 'blocked' OR blocked_reason IS NULL)
);

CREATE INDEX rocketwork_contractor_status_idx ON rocketwork_contractor (status, status_checked_at);

-- 04 §7: одна строка — одна попытка выплаты (R-6, R-9).
CREATE TABLE payout (
    id                       uuid PRIMARY KEY,
    placement_id             uuid NOT NULL REFERENCES placement (id),
    blogger_organization_id  uuid NOT NULL REFERENCES organization (id),
    -- R-3: rocketwork — через сервис; bank_transfer — юрлицу переводом вне сервиса (A-36).
    method                   text NOT NULL CHECK (method IN ('rocketwork', 'bank_transfer')),
    rocketwork_contractor_id uuid REFERENCES rocketwork_contractor (id),
    amount                   bigint NOT NULL CHECK (amount > 0),
    attempt                  integer NOT NULL CHECK (attempt >= 1),
    status                   text NOT NULL CHECK (status IN ('created', 'sent', 'succeeded', 'failed')),
    external_id              text CHECK (char_length(external_id) BETWEEN 1 AND 200),
    provider_fee             bigint CHECK (provider_fee >= 0),
    receipt_url              text CHECK (char_length(receipt_url) <= 2000),
    -- Номер платёжного поручения для bank_transfer (R-3).
    bank_reference           text CHECK (char_length(bank_reference) <= 200),
    failure_code             text CHECK (failure_code IN (
        'contractor_not_active', 'tax_status_lost', 'income_limit_exceeded', 'insufficient_deposit', 'provider_error')),
    failure_message          text CHECK (char_length(failure_message) <= 1000),
    sent_at                  timestamptz,
    completed_at             timestamptz,
    -- R-6: передаётся в Рокет Ворк; у каждой попытки свой.
    idempotency_key          text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 200),
    lock_version             integer NOT NULL DEFAULT 0,
    -- Администратор, создавший попытку повтором (R-9); пусто — система.
    created_by               uuid REFERENCES "user" (id),
    created_at               timestamptz NOT NULL,
    updated_at               timestamptz NOT NULL,
    CONSTRAINT payout_idempotency_key_key UNIQUE (idempotency_key),
    CONSTRAINT payout_external_id_key UNIQUE (external_id),
    CONSTRAINT payout_attempt_key UNIQUE (placement_id, attempt),
    CONSTRAINT payout_method_contractor CHECK (
        (method = 'rocketwork') = (rocketwork_contractor_id IS NOT NULL)),
    CONSTRAINT payout_succeeded_completed CHECK (status <> 'succeeded' OR completed_at IS NOT NULL),
    CONSTRAINT payout_failed_code CHECK ((status = 'failed') = (failure_code IS NOT NULL))
);

-- 04 §7: не больше одной живой выплаты размещения.
CREATE UNIQUE INDEX payout_placement_live_key ON payout (placement_id) WHERE status IN ('created', 'sent', 'succeeded');
CREATE INDEX payout_organization_list_idx ON payout (blogger_organization_id, created_at DESC, id DESC);
CREATE INDEX payout_status_created_at_idx ON payout (status, created_at);

-- Основание проводки payout (00010 оставил поле без внешнего ключа до этого этапа).
ALTER TABLE ledger_transaction ADD CONSTRAINT ledger_transaction_payout_id_fkey
    FOREIGN KEY (payout_id) REFERENCES payout (id) NOT VALID;
ALTER TABLE ledger_transaction VALIDATE CONSTRAINT ledger_transaction_payout_id_fkey;

-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('REVOKE DELETE, TRUNCATE ON rocketwork_contractor, payout FROM %I', app_role);
END
$$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE ledger_transaction DROP CONSTRAINT ledger_transaction_payout_id_fkey;
DROP TABLE payout;
DROP TABLE rocketwork_contractor;
