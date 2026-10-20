-- Деньги (04 §7, 06 §2, этап «Приём оплаты и учёт денег»): журнал проводок, удержания,
-- счета на оплату, выписки и поступления, закрывающие документы.
-- Права роли приложения выдаёт ALTER DEFAULT PRIVILEGES из 00002. Журнал — только добавление
-- (06 §2.2 п. 5): у ledger_transaction и ledger_entry роль не может менять и удалять строки.
-- Остальные денежные таблицы не удаляются. Писать в ledger_* и hold может только ledger/postgres.

-- +goose Up
-- 06 §2.1: счёт создаётся при первом обращении; balance — кеш суммы проводок (06 §2.2 п. 6).
CREATE TABLE ledger_account (
    id              uuid PRIMARY KEY,
    kind            text NOT NULL CHECK (kind IN (
        'bank', 'rocketwork_deposit', 'payout_fees',
        'advertiser_balance', 'placement_hold', 'blogger_payable', 'platform_revenue')),
    organization_id uuid REFERENCES organization (id),
    placement_id    uuid REFERENCES placement (id),
    balance         bigint NOT NULL DEFAULT 0,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    CONSTRAINT ledger_account_key UNIQUE NULLS NOT DISTINCT (kind, organization_id, placement_id),
    -- Разрез соответствует виду (06 §2.1).
    CONSTRAINT ledger_account_scope CHECK (
        (kind IN ('advertiser_balance', 'blogger_payable')) = (organization_id IS NOT NULL)
        AND (kind = 'placement_hold') = (placement_id IS NOT NULL)),
    -- 06 §2.2 п. 4: остаток пассива не отрицателен.
    CONSTRAINT ledger_account_liability_balance CHECK (
        kind IN ('bank', 'rocketwork_deposit', 'payout_fees') OR balance >= 0)
);

CREATE TABLE ledger_transaction (
    id                  uuid PRIMARY KEY,
    kind                text NOT NULL CHECK (kind IN (
        'deposit', 'hold', 'hold_release', 'settlement', 'payout', 'deposit_topup', 'balance_return', 'manual')),
    -- Основание; заполняется подходящее. payout_id — без внешнего ключа до этапа выплат.
    placement_id        uuid REFERENCES placement (id),
    invoice_id          uuid,
    bank_transaction_id uuid,
    payout_id           uuid,
    actor_type          text NOT NULL CHECK (actor_type IN ('system', 'admin', 'advertiser')),
    actor_user_id       uuid REFERENCES "user" (id),
    reason              text CHECK (char_length(reason) <= 1000),
    -- 06 §2.2 п. 7, R-3: ключ «вид + основание».
    idempotency_key     text NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 200),
    -- R-2: отпечаток состава; повтор ключа с другим составом отклоняется.
    entries_hash        text NOT NULL,
    created_at          timestamptz NOT NULL,
    CONSTRAINT ledger_transaction_idempotency_key_key UNIQUE (idempotency_key),
    CONSTRAINT ledger_transaction_reason_required CHECK (
        kind NOT IN ('manual', 'balance_return') OR (reason IS NOT NULL AND char_length(reason) > 0))
);

CREATE INDEX ledger_transaction_placement_id_idx ON ledger_transaction (placement_id) WHERE placement_id IS NOT NULL;
CREATE INDEX ledger_transaction_kind_created_at_idx ON ledger_transaction (kind, created_at);
CREATE INDEX ledger_transaction_created_at_idx ON ledger_transaction (created_at DESC, id DESC);

CREATE TABLE ledger_entry (
    id                    uuid PRIMARY KEY,
    ledger_transaction_id uuid NOT NULL REFERENCES ledger_transaction (id),
    -- Порядок проводок внутри транзакции (правило вида, R-2).
    position              smallint NOT NULL CHECK (position >= 0),
    debit_account_id      uuid NOT NULL REFERENCES ledger_account (id),
    credit_account_id     uuid NOT NULL REFERENCES ledger_account (id),
    amount                bigint NOT NULL CHECK (amount > 0),
    created_at            timestamptz NOT NULL,
    CONSTRAINT ledger_entry_position_key UNIQUE (ledger_transaction_id, position),
    CONSTRAINT ledger_entry_accounts_differ CHECK (debit_account_id <> credit_account_id)
);

CREATE INDEX ledger_entry_debit_idx ON ledger_entry (debit_account_id, created_at);
CREATE INDEX ledger_entry_credit_idx ON ledger_entry (credit_account_id, created_at);

-- 04 §7, R-6: удержание по размещению и версии поста, с которой оно создано.
CREATE TABLE hold (
    id              uuid PRIMARY KEY,
    placement_id    uuid NOT NULL REFERENCES placement (id),
    post_version_id uuid NOT NULL REFERENCES post_version (id),
    -- Рекламодатель: сумма «удержано» в кабинете без чтения таблиц размещений (R-9).
    organization_id uuid NOT NULL REFERENCES organization (id),
    amount          bigint NOT NULL CHECK (amount > 0),
    status          text NOT NULL CHECK (status IN ('active', 'released', 'distributed')),
    closed_at       timestamptz,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    CONSTRAINT hold_closed_at CHECK ((status = 'active') = (closed_at IS NULL))
);

-- I-03: не больше одного активного удержания.
CREATE UNIQUE INDEX hold_active_placement_key ON hold (placement_id) WHERE status = 'active';
CREATE INDEX hold_placement_id_idx ON hold (placement_id);
CREATE INDEX hold_active_organization_idx ON hold (organization_id) WHERE status = 'active';

-- R-1: номер счёта MX-NNNNNN из последовательности.
CREATE SEQUENCE invoice_number_seq START WITH 1;

CREATE TABLE invoice (
    id                      uuid PRIMARY KEY,
    number                  text NOT NULL,
    organization_id         uuid NOT NULL REFERENCES organization (id),
    amount                  bigint NOT NULL CHECK (amount > 0),
    status                  text NOT NULL CHECK (status IN ('issued', 'paid', 'expired', 'cancelled')),
    expires_at              timestamptz NOT NULL,
    paid_at                 timestamptz,
    paid_amount             bigint CHECK (paid_amount > 0),
    -- R-10, D-26: плательщик на момент выставления.
    payer_name              text NOT NULL,
    payer_inn               text NOT NULL,
    payer_bank_account      text,
    payer_bank_name         text,
    payer_bank_bik          text,
    payer_bank_corr_account text,
    pdf_storage_key         text NOT NULL,
    created_by              uuid REFERENCES "user" (id),
    cancel_reason           text CHECK (char_length(cancel_reason) <= 1000),
    created_at              timestamptz NOT NULL,
    updated_at              timestamptz NOT NULL,
    CONSTRAINT invoice_number_key UNIQUE (number),
    CONSTRAINT invoice_paid_pair CHECK ((status = 'paid') = (paid_at IS NOT NULL) AND (paid_at IS NULL) = (paid_amount IS NULL)),
    CONSTRAINT invoice_cancel_reason CHECK (status <> 'cancelled' OR cancel_reason IS NOT NULL)
);

ALTER SEQUENCE invoice_number_seq OWNED BY invoice.number;

CREATE INDEX invoice_organization_list_idx ON invoice (organization_id, created_at DESC, id DESC);
CREATE INDEX invoice_status_expires_at_idx ON invoice (status, expires_at);
CREATE INDEX invoice_issued_inn_idx ON invoice (payer_inn, amount) WHERE status = 'issued';

-- A-35, R-12: загруженный файл выписки и остатки для сверки с банком (06 §2.4).
CREATE TABLE bank_statement (
    id                uuid PRIMARY KEY,
    uploaded_by       uuid NOT NULL REFERENCES "user" (id),
    storage_key       text NOT NULL,
    file_sha256       text NOT NULL,
    account_number    text NOT NULL,
    date_from         date,
    date_to           date,
    opening_balance   bigint,
    closing_balance   bigint,
    incoming_count    integer NOT NULL DEFAULT 0 CHECK (incoming_count >= 0),
    matched_count     integer NOT NULL DEFAULT 0 CHECK (matched_count >= 0),
    unmatched_count   integer NOT NULL DEFAULT 0 CHECK (unmatched_count >= 0),
    duplicate_count   integer NOT NULL DEFAULT 0 CHECK (duplicate_count >= 0),
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL,
    CONSTRAINT bank_statement_file_sha256_key UNIQUE (file_sha256)
);

CREATE INDEX bank_statement_created_at_idx ON bank_statement (created_at DESC);

CREATE TABLE bank_transaction (
    id                uuid PRIMARY KEY,
    bank_statement_id uuid NOT NULL REFERENCES bank_statement (id),
    -- Защита от повторного зачисления (05 §5 шаг 1, R-12).
    external_id       text NOT NULL,
    operation_date    date NOT NULL,
    amount            bigint NOT NULL CHECK (amount > 0),
    payer_inn         text,
    payer_name        text,
    payer_account     text,
    purpose           text,
    match_status      text NOT NULL CHECK (match_status IN ('matched', 'unmatched', 'ignored')),
    invoice_id        uuid REFERENCES invoice (id),
    organization_id   uuid REFERENCES organization (id),
    -- Пусто — сопоставлено автоматически.
    matched_by        uuid REFERENCES "user" (id),
    ignored_reason    text CHECK (char_length(ignored_reason) <= 1000),
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL,
    CONSTRAINT bank_transaction_external_id_key UNIQUE (external_id),
    CONSTRAINT bank_transaction_matched_org CHECK (match_status <> 'matched' OR organization_id IS NOT NULL),
    CONSTRAINT bank_transaction_ignored_reason CHECK (match_status <> 'ignored' OR ignored_reason IS NOT NULL)
);

CREATE INDEX bank_transaction_match_status_idx ON bank_transaction (match_status, operation_date, id);
CREATE INDEX bank_transaction_statement_idx ON bank_transaction (bank_statement_id);

ALTER TABLE ledger_transaction
    ADD CONSTRAINT ledger_transaction_invoice_id_fkey FOREIGN KEY (invoice_id) REFERENCES invoice (id),
    ADD CONSTRAINT ledger_transaction_bank_transaction_id_fkey FOREIGN KEY (bank_transaction_id) REFERENCES bank_transaction (id);

-- R-16: отчёт агента и акт за месяц.
CREATE SEQUENCE closing_act_number_seq START WITH 1;

CREATE TABLE closing_act (
    id                     uuid PRIMARY KEY,
    organization_id        uuid NOT NULL REFERENCES organization (id),
    -- Первое число месяца.
    period                 date NOT NULL CHECK (extract(day FROM period) = 1),
    number                 text NOT NULL,
    placements_amount      bigint NOT NULL CHECK (placements_amount >= 0),
    fee_amount             bigint NOT NULL CHECK (fee_amount >= 0),
    report_pdf_storage_key text NOT NULL,
    act_pdf_storage_key    text NOT NULL,
    created_at             timestamptz NOT NULL,
    CONSTRAINT closing_act_number_key UNIQUE (number),
    CONSTRAINT closing_act_organization_id_period_key UNIQUE (organization_id, period)
);

ALTER SEQUENCE closing_act_number_seq OWNED BY closing_act.number;

CREATE INDEX closing_act_organization_list_idx ON closing_act (organization_id, period DESC, id DESC);

-- I-12, R-8: распределение удержания сходится для размещений в accepted, paid, refunded.
ALTER TABLE placement ADD CONSTRAINT placement_distribution_sum CHECK (
    status NOT IN ('accepted', 'paid', 'refunded')
    OR (performance_share IS NOT NULL AND payout_amount IS NOT NULL AND fee_charged IS NOT NULL
        AND refund_amount IS NOT NULL AND payout_amount + fee_charged + refund_amount = total_amount));

-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON ledger_transaction, ledger_entry FROM %I', app_role);
    EXECUTE format('REVOKE DELETE, TRUNCATE ON ledger_account, hold, invoice, bank_statement, bank_transaction, closing_act FROM %I', app_role);
    EXECUTE format('GRANT USAGE, SELECT ON SEQUENCE invoice_number_seq, closing_act_number_seq TO %I', app_role);
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('REVOKE ALL ON SEQUENCE invoice_number_seq, closing_act_number_seq FROM %I', app_role);
END
$$;
-- +goose StatementEnd
ALTER TABLE placement DROP CONSTRAINT placement_distribution_sum;
DROP TABLE closing_act;
DROP TABLE ledger_entry;
DROP TABLE ledger_transaction;
DROP TABLE bank_transaction;
DROP TABLE bank_statement;
DROP TABLE invoice;
DROP TABLE hold;
DROP TABLE ledger_account;
