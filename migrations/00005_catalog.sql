-- Каталог (04 §3, 03 §2, S-02, S-13, S-16, S-32): справочник форматов, цены каналов,
-- интервалы недоступности, суточные снимки метрик. Права роли приложения выдаёт
-- ALTER DEFAULT PRIVILEGES из 00002; удаление цен и интервалов роли разрешено
-- (это не неизменяемые таблицы 04 §1).

-- +goose Up
CREATE TABLE placement_format (
    id             text PRIMARY KEY,
    name           text NOT NULL,
    top_hours      integer NOT NULL CHECK (top_hours > 0),
    duration_hours integer NOT NULL CHECK (duration_hours >= top_hours),
    is_active      boolean NOT NULL DEFAULT true,
    position       integer NOT NULL,
    created_at     timestamptz NOT NULL,
    updated_at     timestamptz NOT NULL
);

-- 03 §2, D-19: форматы — данные справочника, а не код.
INSERT INTO placement_format (id, name, top_hours, duration_hours, position, created_at, updated_at) VALUES
    ('f_1_24', '1/24', 1, 24, 0, now(), now()),
    ('f_2_48', '2/48', 2, 48, 1, now(), now()),
    ('f_3_72', '3/72', 3, 72, 2, now(), now());

CREATE TABLE channel_price (
    id         uuid PRIMARY KEY,
    channel_id uuid NOT NULL REFERENCES channel (id),
    format_id  text NOT NULL REFERENCES placement_format (id),
    -- Цена блогера без комиссии (04 §3): от 100 до 10 000 000 ₽.
    amount     bigint NOT NULL CHECK (amount BETWEEN 10000 AND 1000000000),
    is_enabled boolean NOT NULL,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT channel_price_channel_id_format_id_key UNIQUE (channel_id, format_id)
);

CREATE TABLE channel_unavailability (
    id         uuid PRIMARY KEY,
    channel_id uuid NOT NULL REFERENCES channel (id),
    starts_at  timestamptz NOT NULL,
    ends_at    timestamptz NOT NULL,
    created_at timestamptz NOT NULL,
    CONSTRAINT channel_unavailability_interval CHECK (starts_at < ends_at)
);

-- Живые интервалы канала и очистка прошедших (R-6).
CREATE INDEX channel_unavailability_channel_id_ends_at_idx ON channel_unavailability (channel_id, ends_at);
CREATE INDEX channel_unavailability_ends_at_idx ON channel_unavailability (ends_at);

-- R-19: снимок на сутки по Москве; повтор задания в те же сутки перезаписывает строку.
CREATE TABLE channel_metrics (
    id                uuid PRIMARY KEY,
    channel_id        uuid NOT NULL REFERENCES channel (id),
    date              date NOT NULL,
    subscribers_count integer NOT NULL CHECK (subscribers_count >= 0),
    avg_views         integer CHECK (avg_views >= 0),
    engagement_bp     integer CHECK (engagement_bp >= 0),
    posts_per_week    numeric(6, 2) CHECK (posts_per_week >= 0),
    anomaly_flags     text[] NOT NULL DEFAULT '{}'
        CHECK (anomaly_flags <@ ARRAY['subscribers_spike', 'views_disproportion']::text[]),
    created_at        timestamptz NOT NULL,
    updated_at        timestamptz NOT NULL,
    CONSTRAINT channel_metrics_channel_id_date_key UNIQUE (channel_id, date)
);

-- R-3: канал в active должен иметь включённую цену активного формата (04 §3). CHECK
-- не видит других таблиц, поэтому условие проверяет отложенный триггер при фиксации
-- транзакции. Основная проверка — channel/domain; триггер — страховка. Выключение
-- формата в справочнике (этап «API администратора») проверит свой сценарий.
-- +goose StatementBegin
CREATE FUNCTION channel_active_requires_price() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    ch uuid;
BEGIN
    IF TG_TABLE_NAME = 'channel' THEN
        ch := NEW.id;
    ELSE
        ch := OLD.channel_id;
    END IF;
    IF EXISTS (SELECT 1 FROM channel WHERE id = ch AND status = 'active')
       AND NOT EXISTS (
           SELECT 1 FROM channel_price p JOIN placement_format f ON f.id = p.format_id
           WHERE p.channel_id = ch AND p.is_enabled AND f.is_active) THEN
        RAISE EXCEPTION 'channel % is active without an enabled price', ch
            USING ERRCODE = 'check_violation', CONSTRAINT = 'channel_active_requires_price', TABLE = 'channel';
    END IF;
    RETURN NULL;
END
$$;
-- +goose StatementEnd

CREATE CONSTRAINT TRIGGER channel_active_requires_price
    AFTER INSERT OR UPDATE OF status ON channel
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION channel_active_requires_price();

CREATE CONSTRAINT TRIGGER channel_price_keeps_active
    AFTER UPDATE OR DELETE ON channel_price
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION channel_active_requires_price();

COMMENT ON CONSTRAINT channel_active_requirements ON channel IS
    'active (04 §3): организация, публичность, категория, права write и delete. Включённая цена — триггер channel_active_requires_price (00005), порог РКН — channel/domain (RKN_SUBSCRIBERS_THRESHOLD)';

-- Каталог: активные каналы по категории и подписчикам (07 §7: до 1 с на 2 000 каналах).
CREATE INDEX channel_catalog_idx ON channel (category_id, subscribers_count) WHERE status = 'active';
-- Очередь администратора (R-1, R-17): фильтры по состоянию и проверке РКН.
CREATE INDEX channel_status_rkn_idx ON channel (status, rkn_status, rkn_checked_at);

-- +goose Down
DROP INDEX channel_status_rkn_idx;
DROP INDEX channel_catalog_idx;
COMMENT ON CONSTRAINT channel_active_requirements ON channel IS NULL;
DROP TRIGGER channel_price_keeps_active ON channel_price;
DROP TRIGGER channel_active_requires_price ON channel;
DROP FUNCTION channel_active_requires_price();
DROP TABLE channel_metrics;
DROP TABLE channel_unavailability;
DROP TABLE channel_price;
DROP TABLE placement_format;
