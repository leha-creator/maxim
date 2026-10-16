-- Маркировка (04 §6, этап «Маркировка через Яндекс ОРД»): контрагенты, договоры, площадки
-- и креативы в ОРД с полями синхронизации 04 §6.1. Сверх них: ord_status — последний сырой
-- статус ОРД, sent_at — время отправки (расписание опроса и зависшие, R-2, R-10).
-- В ОРД передаются наши идентификаторы (R-1): ord_external_id хранит отправленный.
-- Права роли приложения выдаёт ALTER DEFAULT PRIVILEGES из 00002; удалять можно только
-- креатив без отправки (R-11), остальные строки не удаляются.
-- channel.ord_platform_ready_at — канал виден в каталоге только после регистрации площадки (R-14).

-- +goose Up
CREATE TABLE ord_organization (
    id              uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organization (id),
    -- R-6: рекламораспространитель — организация-блогер.
    is_rr           boolean NOT NULL DEFAULT false,
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
    CONSTRAINT ord_organization_organization_id_key UNIQUE (organization_id),
    CONSTRAINT ord_organization_sent_at CHECK (sync_status NOT IN ('sent', 'synced') OR sent_at IS NOT NULL)
);

CREATE TABLE ord_contract (
    id                         uuid PRIMARY KEY,
    -- A-31: initial — рекламодатель → платформа, publisher — платформа → блогер.
    kind                       text NOT NULL CHECK (kind IN ('initial', 'publisher')),
    client_organization_id     uuid NOT NULL REFERENCES organization (id),
    contractor_organization_id uuid NOT NULL REFERENCES organization (id),
    number                     text,
    signed_date                date,
    ord_external_id            text,
    sync_status                text NOT NULL CHECK (sync_status IN ('pending', 'sent', 'synced', 'error')),
    sync_request_id            text,
    sync_error                 text,
    sync_attempts              integer NOT NULL DEFAULT 0 CHECK (sync_attempts >= 0),
    synced_at                  timestamptz,
    payload_hash               text,
    ord_status                 text,
    sent_at                    timestamptz,
    created_at                 timestamptz NOT NULL,
    updated_at                 timestamptz NOT NULL,
    CONSTRAINT ord_contract_parties_key UNIQUE (kind, client_organization_id, contractor_organization_id),
    CONSTRAINT ord_contract_sent_at CHECK (sync_status NOT IN ('sent', 'synced') OR sent_at IS NOT NULL)
);

CREATE TABLE ord_platform (
    id              uuid PRIMARY KEY,
    channel_id      uuid NOT NULL REFERENCES channel (id),
    organization_id uuid NOT NULL REFERENCES organization (id),
    name            text NOT NULL,
    url             text NOT NULL,
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
    CONSTRAINT ord_platform_channel_id_key UNIQUE (channel_id),
    CONSTRAINT ord_platform_sent_at CHECK (sync_status NOT IN ('sent', 'synced') OR sent_at IS NOT NULL)
);

CREATE TABLE creative (
    id               uuid PRIMARY KEY,
    -- D-07: один креатив на версию поста.
    post_version_id  uuid NOT NULL REFERENCES post_version (id),
    -- Изначальный договор; задаётся, когда цепочка готова (R-4).
    ord_contract_id  uuid REFERENCES ord_contract (id),
    erid             text CHECK (erid <> ''),
    erid_received_at timestamptz,
    erir_status      text NOT NULL DEFAULT 'pending' CHECK (erir_status IN ('pending', 'success', 'error')),
    erir_error       text,
    ord_external_id  text,
    sync_status      text NOT NULL CHECK (sync_status IN ('pending', 'sent', 'synced', 'error')),
    sync_request_id  text,
    sync_error       text,
    sync_attempts    integer NOT NULL DEFAULT 0 CHECK (sync_attempts >= 0),
    synced_at        timestamptz,
    payload_hash     text,
    ord_status       text,
    sent_at          timestamptz,
    created_at       timestamptz NOT NULL,
    updated_at       timestamptz NOT NULL,
    CONSTRAINT creative_post_version_id_key UNIQUE (post_version_id),
    CONSTRAINT creative_sent_at CHECK (sync_status NOT IN ('sent', 'synced') OR sent_at IS NOT NULL),
    CONSTRAINT creative_erid_pair CHECK ((erid IS NULL) = (erid_received_at IS NULL))
);

CREATE UNIQUE INDEX creative_erid_key ON creative (erid) WHERE erid IS NOT NULL;
CREATE INDEX creative_ord_contract_id_idx ON creative (ord_contract_id);

-- Список ошибок S-20 и метрика числа объектов по состояниям.
CREATE INDEX ord_organization_sync_status_idx ON ord_organization (sync_status);
CREATE INDEX ord_contract_sync_status_idx ON ord_contract (sync_status);
CREATE INDEX ord_platform_sync_status_idx ON ord_platform (sync_status);
CREATE INDEX creative_sync_status_idx ON creative (sync_status);
-- Зависшие в sent (07 §3).
CREATE INDEX ord_organization_sent_idx ON ord_organization (sync_status, sent_at) WHERE sync_status = 'sent';
CREATE INDEX ord_contract_sent_idx ON ord_contract (sync_status, sent_at) WHERE sync_status = 'sent';
CREATE INDEX ord_platform_sent_idx ON ord_platform (sync_status, sent_at) WHERE sync_status = 'sent';
CREATE INDEX creative_sent_idx ON creative (sync_status, sent_at) WHERE sync_status = 'sent';
-- Креативы без erid (T-11).
CREATE INDEX creative_without_erid_idx ON creative (created_at) WHERE erid IS NULL;

ALTER TABLE channel ADD COLUMN ord_platform_ready_at timestamptz;
-- R-14: каналы, уже видимые в каталоге (данные разработки), остаются видимыми.
UPDATE channel SET ord_platform_ready_at = now() WHERE status = 'active';

-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('REVOKE DELETE, TRUNCATE ON ord_organization, ord_contract, ord_platform FROM %I', app_role);
    EXECUTE format('REVOKE TRUNCATE ON creative FROM %I', app_role);
END
$$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE channel DROP COLUMN ord_platform_ready_at;
DROP TABLE creative;
DROP TABLE ord_platform;
DROP TABLE ord_contract;
DROP TABLE ord_organization;
