-- Каналы и подключение (04 §3, 05 §2.4, S-12, S-25): справочник категорий, канал,
-- поиск кода подключения по хешу. Права роли приложения выдаёт ALTER DEFAULT PRIVILEGES из 00002.

-- +goose Up
CREATE TABLE channel_category (
    id         uuid PRIMARY KEY,
    slug       text NOT NULL UNIQUE,
    name       text NOT NULL,
    position   integer NOT NULL,
    is_active  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

-- R-13: начальный список категорий (A-14) — тот же, что в тестовом сервере mockapi.
-- Идентификаторы постоянные: на них ссылаются тесты и кабинеты в разработке.
INSERT INTO channel_category (id, slug, name, position, created_at, updated_at) VALUES
    ('0190a000-0000-7000-8000-000000000201', 'food',     'Еда и кулинария', 0, now(), now()),
    ('0190a000-0000-7000-8000-000000000202', 'tech',     'Технологии',      1, now(), now()),
    ('0190a000-0000-7000-8000-000000000203', 'travel',   'Путешествия',     2, now(), now()),
    ('0190a000-0000-7000-8000-000000000204', 'business', 'Бизнес',          3, now(), now()),
    ('0190a000-0000-7000-8000-000000000205', 'kids',     'Дети и родители', 4, now(), now()),
    ('0190a000-0000-7000-8000-000000000206', 'sport',    'Спорт',           5, now(), now()),
    ('0190a000-0000-7000-8000-000000000207', 'auto',     'Авто',            6, now(), now()),
    ('0190a000-0000-7000-8000-000000000208', 'humor',    'Юмор',            7, now(), now());

CREATE TABLE channel (
    id                         uuid PRIMARY KEY,
    organization_id            uuid REFERENCES organization (id),
    max_chat_id                bigint NOT NULL UNIQUE,
    title                      text,
    max_description            text,
    icon_url                   text,
    link                       text,
    is_public                  boolean NOT NULL DEFAULT false,
    added_by_max_user_id       bigint NOT NULL,
    description                text,
    category_id                uuid REFERENCES channel_category (id),
    status                     text NOT NULL CHECK (status IN (
        'unclaimed', 'pending_rights', 'moderation', 'active', 'rejected', 'suspended', 'disconnected', 'blocked')),
    status_reason              text,
    -- R-9: состояние до удаления бота; по нему повторное добавление возвращает канал в active.
    status_before_disconnect   text CHECK (status_before_disconnect IN (
        'unclaimed', 'pending_rights', 'moderation', 'active', 'rejected', 'suspended', 'blocked')),
    -- R-12: с какого момента канал ждёт кода подключения; через CHANNEL_UNCLAIMED_TTL бот покидает канал.
    unclaimed_since            timestamptz,
    bot_is_admin               boolean NOT NULL DEFAULT false,
    bot_permissions            text[] NOT NULL DEFAULT '{}',
    bot_permissions_checked_at timestamptz,
    subscribers_count          integer NOT NULL DEFAULT 0 CHECK (subscribers_count >= 0),
    rkn_number                 text,
    rkn_status                 text NOT NULL DEFAULT 'not_required' CHECK (rkn_status IN ('not_required', 'pending', 'confirmed', 'rejected')),
    rkn_checked_at             timestamptz,
    rkn_checked_by             uuid REFERENCES "user" (id),
    rating                     smallint NOT NULL DEFAULT 100 CHECK (rating BETWEEN 0 AND 100),
    moderated_at               timestamptz,
    moderated_by               uuid REFERENCES "user" (id),
    lock_version               integer NOT NULL DEFAULT 0,
    created_at                 timestamptz NOT NULL,
    updated_at                 timestamptz NOT NULL,
    CONSTRAINT channel_unclaimed_without_owner CHECK (status <> 'unclaimed' OR organization_id IS NULL),
    -- Ограничение active из 04 §3 без двух условий: включённая цена появится в этапе
    -- «Каталог» (его миграция дополнит проверку), порог РКН — параметр конфигурации
    -- (07 §8), его проверяет channel/domain.
    CONSTRAINT channel_active_requirements CHECK (status <> 'active' OR (
        organization_id IS NOT NULL AND is_public AND category_id IS NOT NULL
        AND bot_permissions @> ARRAY['write', 'delete']))
);

CREATE INDEX channel_organization_id_idx ON channel (organization_id);
CREATE INDEX channel_status_unclaimed_since_idx ON channel (status, unclaimed_since);

-- R-2: поиск живого кода подключения по хешу текста сообщения.
CREATE INDEX auth_code_channel_link_hash_idx ON auth_code (code_hash)
    WHERE purpose = 'channel_link' AND consumed_at IS NULL;
CREATE INDEX auth_code_channel_link_org_idx ON auth_code (organization_id, created_at)
    WHERE purpose = 'channel_link';

-- +goose Down
DROP INDEX auth_code_channel_link_org_idx;
DROP INDEX auth_code_channel_link_hash_idx;
DROP TABLE channel;
DROP TABLE channel_category;
