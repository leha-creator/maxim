-- Уведомления (04 §8, 07 §6, этап «Уведомления и отчёт по кампании»): одна строка на адресата
-- и канал доставки. Кабинет создаётся сразу доставленным (sent), бот MAX и почта — pending и
-- отправляются заданием send_notification (R-4). Права роли приложения выдаёт
-- ALTER DEFAULT PRIVILEGES из 00002: удаление нужно очистке (R-16).

-- +goose Up
CREATE TABLE notification (
    id              uuid PRIMARY KEY,
    user_id         uuid NOT NULL REFERENCES "user" (id),
    -- Пусто у уведомлений администратора (R-1).
    organization_id uuid REFERENCES organization (id),
    channel         text NOT NULL CHECK (channel IN ('max_bot', 'email', 'cabinet')),
    template        text NOT NULL CHECK (template <> ''),
    payload         jsonb NOT NULL DEFAULT '{}'::jsonb,
    status          text NOT NULL CHECK (status IN ('pending', 'sent', 'failed', 'read')),
    object_type     text,
    object_id       text,
    -- R-10: повтор события-задания второго уведомления не создаёт.
    dedupe_key      text,
    -- R-3: адресат личного сообщения бота — channel.added_by_max_user_id.
    max_user_id     bigint,
    attempts        integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    last_error      text,
    sent_at         timestamptz,
    read_at         timestamptz,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    CONSTRAINT notification_read_cabinet CHECK (status <> 'read' OR channel = 'cabinet'),
    CONSTRAINT notification_read_at CHECK ((status = 'read') = (read_at IS NOT NULL)),
    CONSTRAINT notification_max_user CHECK ((channel = 'max_bot') = (max_user_id IS NOT NULL)),
    CONSTRAINT notification_object_pair CHECK ((object_type IS NULL) = (object_id IS NULL))
);

-- Кабинет: список пользователя, новые сверху, и непрочитанные (R-12).
CREATE INDEX notification_cabinet_idx ON notification (user_id, created_at DESC, id DESC) WHERE channel = 'cabinet';
CREATE INDEX notification_unread_idx ON notification (user_id) WHERE channel = 'cabinet' AND read_at IS NULL;
-- Отправка, метрики и очистка.
CREATE INDEX notification_status_idx ON notification (status, created_at);
CREATE UNIQUE INDEX notification_dedupe_key ON notification (user_id, channel, dedupe_key) WHERE dedupe_key IS NOT NULL;

-- +goose Down
DROP TABLE notification;
