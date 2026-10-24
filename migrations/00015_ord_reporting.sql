-- Ежемесячная отчётность ОРД (этап «API администратора и отчётность ОРД», R-18 … R-24):
-- снимки показов на начало месяца, статистика по парам «креатив + площадка», акты по договорам
-- с разбивкой по креативам, состояние отчёта месяца. Поля синхронизации — как у объектов ОРД
-- этапа «Маркировка» (04 §6.1).

-- +goose Up
-- R-19: показы публикации на начало месяца period (первое число, по Москве). Показы месяца —
-- разница снимков; публикация, снятая в месяце, — итог минус снимок начала.
CREATE TABLE publication_views_snapshot (
    publication_id   uuid NOT NULL REFERENCES publication (id),
    period           date NOT NULL CHECK (extract(day FROM period) = 1),
    views            integer NOT NULL CHECK (views >= 0),
    views_checked_at timestamptz,
    created_at       timestamptz NOT NULL,
    PRIMARY KEY (publication_id, period)
);

-- R-20: строка статистики месяца по паре «креатив + площадка»; сумма — в месяце окончания размещения.
CREATE TABLE ord_statistics (
    id              uuid PRIMARY KEY,
    creative_id     uuid NOT NULL REFERENCES creative (id),
    ord_platform_id uuid NOT NULL REFERENCES ord_platform (id),
    -- Размещение, по которому строка создана первой (для экрана); пара может включать несколько.
    placement_id    uuid NOT NULL REFERENCES placement (id),
    period          date NOT NULL CHECK (extract(day FROM period) = 1),
    impressions     bigint NOT NULL CHECK (impressions >= 0),
    shown_from      date NOT NULL,
    shown_to        date NOT NULL CHECK (shown_to >= shown_from),
    amount          bigint NOT NULL CHECK (amount >= 0),
    ord_external_id text,
    sync_status     text NOT NULL CHECK (sync_status IN ('pending', 'sent', 'synced', 'error')),
    sync_request_id text,
    sync_error      text,
    sync_attempts   integer NOT NULL DEFAULT 0 CHECK (sync_attempts >= 0),
    synced_at       timestamptz,
    payload_hash    text,
    ord_status      text,
    sent_at         timestamptz,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    CONSTRAINT ord_statistics_pair_key UNIQUE (creative_id, ord_platform_id, period),
    CONSTRAINT ord_statistics_sent_at CHECK (sync_status NOT IN ('sent', 'synced') OR sent_at IS NOT NULL)
);

CREATE INDEX ord_statistics_period_idx ON ord_statistics (period);
CREATE INDEX ord_statistics_sync_status_idx ON ord_statistics (sync_status);

-- R-20, R-22: акт месяца по договору (изначальный — рекламодатель, с блогером — блогер).
CREATE TABLE ord_invoice (
    id              uuid PRIMARY KEY,
    ord_contract_id uuid NOT NULL REFERENCES ord_contract (id),
    period          date NOT NULL CHECK (extract(day FROM period) = 1),
    type            text NOT NULL CHECK (char_length(type) BETWEEN 1 AND 64),
    number          text NOT NULL CHECK (char_length(number) BETWEEN 1 AND 64),
    invoice_date    date NOT NULL,
    amount          bigint NOT NULL CHECK (amount > 0),
    ord_external_id text,
    sync_status     text NOT NULL CHECK (sync_status IN ('pending', 'sent', 'synced', 'error')),
    sync_request_id text,
    sync_error      text,
    sync_attempts   integer NOT NULL DEFAULT 0 CHECK (sync_attempts >= 0),
    synced_at       timestamptz,
    payload_hash    text,
    ord_status      text,
    sent_at         timestamptz,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    CONSTRAINT ord_invoice_number_key UNIQUE (number),
    CONSTRAINT ord_invoice_contract_period_key UNIQUE (ord_contract_id, period),
    CONSTRAINT ord_invoice_sent_at CHECK (sync_status NOT IN ('sent', 'synced') OR sent_at IS NOT NULL)
);

CREATE INDEX ord_invoice_period_idx ON ord_invoice (period);
CREATE INDEX ord_invoice_sync_status_idx ON ord_invoice (sync_status);

-- Разбивка акта по парам «креатив + площадка»; пересобирается при формировании отчёта.
CREATE TABLE ord_invoice_item (
    ord_invoice_id  uuid NOT NULL REFERENCES ord_invoice (id),
    creative_id     uuid NOT NULL REFERENCES creative (id),
    ord_platform_id uuid NOT NULL REFERENCES ord_platform (id),
    amount          bigint NOT NULL CHECK (amount >= 0),
    CONSTRAINT ord_invoice_item_key UNIQUE (ord_invoice_id, creative_id, ord_platform_id)
);

-- R-21: состояние отчёта месяца для экрана и проверки срока (R-23).
CREATE TABLE ord_report (
    period            date PRIMARY KEY CHECK (extract(day FROM period) = 1),
    built_at          timestamptz NOT NULL,
    built_by          uuid REFERENCES "user" (id),
    send_requested_at timestamptz,
    send_requested_by uuid REFERENCES "user" (id),
    updated_at        timestamptz NOT NULL
);

-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON publication_views_snapshot FROM %I', app_role);
    EXECUTE format('REVOKE TRUNCATE ON ord_statistics, ord_invoice, ord_invoice_item FROM %I', app_role);
    EXECUTE format('REVOKE DELETE, TRUNCATE ON ord_report FROM %I', app_role);
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE ord_report;
DROP TABLE ord_invoice_item;
DROP TABLE ord_invoice;
DROP TABLE ord_statistics;
DROP TABLE publication_views_snapshot;
