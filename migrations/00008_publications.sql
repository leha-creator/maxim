-- Публикации, нарушения и споры (04 §5, 03 §8, этап «Автопостинг и контроль»): сообщение
-- в канале по размещению, найденные контролем нарушения и спор, который открывает система
-- (P-14). Права роли приложения выдаёт ALTER DEFAULT PRIVILEGES из 00002; нарушения —
-- только добавление (R-20). Поля решения спора пишет этап «Приёмка, споры и возвраты».

-- +goose Up
CREATE TABLE publication (
    id               uuid PRIMARY KEY,
    -- I-11: у размещения не больше одной публикации.
    placement_id     uuid NOT NULL REFERENCES placement (id),
    channel_id       uuid NOT NULL REFERENCES channel (id),
    max_message_id   text NOT NULL CHECK (max_message_id <> ''),
    url              text,
    published_at     timestamptz NOT NULL,
    -- Снятие: при своём удалении — намерение до вызова MAX (05 §2.6 шаг 2, R-7).
    removed_at       timestamptz,
    removal_reason   text CHECK (removal_reason IN ('schedule', 'external', 'admin', 'refund')),
    -- T-10: удаление по сроку не удалось.
    removal_overdue  boolean NOT NULL DEFAULT false,
    views            integer CHECK (views >= 0),
    views_checked_at timestamptz,
    -- R-6: хеш сообщения в том виде, в каком его хранит MAX.
    content_hash     text NOT NULL CHECK (content_hash <> ''),
    created_at       timestamptz NOT NULL,
    updated_at       timestamptz NOT NULL,
    CONSTRAINT publication_placement_id_key UNIQUE (placement_id),
    CONSTRAINT publication_max_message_id_key UNIQUE (max_message_id),
    CONSTRAINT publication_removal_pair CHECK ((removed_at IS NULL) = (removal_reason IS NULL))
);

-- Живые публикации: сверка (T-09) и контроль топа по каналу.
CREATE INDEX publication_live_idx ON publication (id) WHERE removed_at IS NULL;
CREATE INDEX publication_live_channel_idx ON publication (channel_id) WHERE removed_at IS NULL;
-- Зависшие снятия (07 §3).
CREATE INDEX publication_removal_overdue_idx ON publication (updated_at) WHERE removal_overdue;

CREATE TABLE violation (
    id               uuid PRIMARY KEY,
    placement_id     uuid NOT NULL REFERENCES placement (id),
    kind             text NOT NULL CHECK (kind IN ('removed_early', 'content_modified', 'top_violated', 'bot_access_lost')),
    occurred_at      timestamptz NOT NULL,
    source           text NOT NULL CHECK (source IN ('event', 'reconciliation')),
    -- Без внешнего ключа: входящие события очищаются по сроку хранения (04 §8).
    inbound_event_id uuid,
    details          jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at       timestamptz NOT NULL
);

-- R-10: повтор разбора того же события второго нарушения не создаёт.
CREATE UNIQUE INDEX violation_placement_event_key ON violation (placement_id, inbound_event_id)
    WHERE inbound_event_id IS NOT NULL;
CREATE INDEX violation_placement_idx ON violation (placement_id, occurred_at);

CREATE TABLE dispute (
    id                 uuid PRIMARY KEY,
    -- I-11: у размещения не больше одного спора.
    placement_id       uuid NOT NULL REFERENCES placement (id),
    reason             text NOT NULL CHECK (reason IN (
        'removed_early', 'content_modified', 'top_violated', 'bot_access_lost', 'content_mismatch', 'other', 'admin')),
    opened_by_type     text NOT NULL CHECK (opened_by_type IN ('advertiser', 'system', 'admin')),
    -- Претензии — этап «Приёмка, споры и возвраты»; ключ появится вместе с таблицей claim.
    claim_id           uuid,
    violation_id       uuid REFERENCES violation (id),
    -- T-06.
    deadline_at        timestamptz NOT NULL,
    suggested_share    integer CHECK (suggested_share BETWEEN 0 AND 10000),
    resolution         text CHECK (resolution IN ('full_payout', 'proportional', 'full_refund')),
    performance_share  integer CHECK (performance_share BETWEEN 0 AND 10000),
    resolution_comment text,
    resolved_by        uuid REFERENCES "user" (id),
    resolved_at        timestamptz,
    created_at         timestamptz NOT NULL,
    updated_at         timestamptz NOT NULL,
    CONSTRAINT dispute_placement_id_key UNIQUE (placement_id),
    CONSTRAINT dispute_resolution_complete CHECK (resolution IS NULL OR
        (performance_share IS NOT NULL AND resolution_comment IS NOT NULL AND resolved_at IS NOT NULL))
);

-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON violation FROM %I', app_role);
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP TABLE dispute;
DROP TABLE violation;
DROP TABLE publication;
