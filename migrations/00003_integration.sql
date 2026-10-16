-- Интеграционный слой и планировщик (04 §8, 05 §1, 07 §2–§3): входящие события,
-- исходящие запросы, ограничитель MAX, предохранитель, оповещения, отметки
-- экземпляров worker. Права роли приложения выдаёт ALTER DEFAULT PRIVILEGES из 00002.

-- +goose Up
CREATE TABLE inbound_event (
    id           uuid PRIMARY KEY,
    source       text NOT NULL CHECK (source IN ('max', 'rocketwork')),
    external_key text NOT NULL,
    event_type   text NOT NULL,
    payload      jsonb NOT NULL,
    status       text NOT NULL CHECK (status IN ('received', 'processed', 'ignored', 'failed')),
    processed_at timestamptz,
    error        text,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL,
    CONSTRAINT inbound_event_source_external_key_key UNIQUE (source, external_key)
);

CREATE INDEX inbound_event_status_created_at_idx ON inbound_event (status, created_at);

CREATE TABLE outbound_request (
    id              uuid PRIMARY KEY,
    system          text NOT NULL CHECK (system IN ('max', 'yandex_ord', 'rocketwork')),
    operation       text NOT NULL,
    idempotency_key text,
    attempt         integer NOT NULL CHECK (attempt > 0),
    request         jsonb NOT NULL,
    response_status integer,
    response_body   jsonb,
    duration_ms     integer,
    error           text,
    object_type     text,
    object_id       text,
    created_at      timestamptz NOT NULL
);

CREATE INDEX outbound_request_created_at_idx ON outbound_request (created_at);
CREATE INDEX outbound_request_object_idx ON outbound_request (object_type, object_id);

-- R-1: корзина токенов общего ограничителя (05 §2.8); ключ — max:global, max:chat:{chat_id}.
CREATE TABLE rate_limit_bucket (
    key         text PRIMARY KEY,
    tokens      double precision NOT NULL,
    refilled_at timestamptz NOT NULL
);

-- R-3: предохранитель по внешней системе (05 §1), общий для всех экземпляров.
CREATE TABLE outbound_circuit (
    system               text PRIMARY KEY CHECK (system IN ('max', 'yandex_ord', 'rocketwork')),
    consecutive_failures integer NOT NULL DEFAULT 0 CHECK (consecutive_failures >= 0),
    open_until           timestamptz,
    updated_at           timestamptz NOT NULL
);

-- R-4: оповещения администратору, не чаще раза в час по причине (07 §3).
CREATE TABLE alert (
    id              uuid PRIMARY KEY,
    reason          text NOT NULL,
    object_key      text NOT NULL DEFAULT '',
    level           text NOT NULL CHECK (level IN ('warn', 'error')),
    summary         text NOT NULL,
    first_raised_at timestamptz NOT NULL,
    last_raised_at  timestamptz NOT NULL,
    last_sent_at    timestamptz,
    raised_count    integer NOT NULL CHECK (raised_count > 0),
    resolved_at     timestamptz,
    CONSTRAINT alert_reason_object_key_key UNIQUE (reason, object_key)
);

CREATE INDEX alert_resolved_at_idx ON alert (resolved_at);

-- R-6: отметки экземпляров worker — по ним считается простой (07 §2).
CREATE TABLE worker_heartbeat (
    instance   text PRIMARY KEY,
    started_at timestamptz NOT NULL,
    seen_at    timestamptz NOT NULL
);

CREATE INDEX job_status_finished_at_idx ON job (status, finished_at);

-- +goose Down
DROP INDEX job_status_finished_at_idx;
DROP TABLE worker_heartbeat;
DROP TABLE alert;
DROP TABLE outbound_circuit;
DROP TABLE rate_limit_bucket;
DROP TABLE outbound_request;
DROP TABLE inbound_event;
