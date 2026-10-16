-- Аккаунты, организации, оферты (04 §2) и служебные таблицы job, audit_log (04 §8).
-- Правила 04 §1: id uuid v7 создаёт приложение, время — timestamptz UTC,
-- перечисления — text с check, внешние ключи без каскадного удаления.

-- +goose Up
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE "user" (
    id            uuid PRIMARY KEY,
    email         citext NOT NULL,
    admin_role    text CHECK (admin_role IN ('moderator', 'senior')),
    status        text NOT NULL CHECK (status IN ('active', 'blocked')),
    last_login_at timestamptz,
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL,
    CONSTRAINT user_email_key UNIQUE (email)
);

CREATE TABLE organization (
    id                       uuid PRIMARY KEY,
    legal_type               text NOT NULL CHECK (legal_type IN ('legal_entity', 'sole_proprietor', 'self_employed')),
    inn                      text NOT NULL,
    name                     text NOT NULL,
    contact_name             text NOT NULL,
    phone                    text NOT NULL,
    kpp                      text,
    address                  text,
    tax_regime               text CHECK (tax_regime IN ('osno', 'usn', 'ausn', 'eshn', 'psn', 'npd')),
    vat_rate                 text CHECK (vat_rate IN ('none', 'vat_5', 'vat_7', 'vat_22')),
    tax_document_storage_key text,
    tax_document_sha256      text,
    signer_position          text,
    signer_name              text,
    signer_basis_name        text,
    signer_basis_number      text,
    bank_account             text,
    bank_name                text,
    bank_corr_account        text,
    bank_bik                 text,
    is_platform              boolean NOT NULL DEFAULT false,
    status                   text NOT NULL CHECK (status IN ('active', 'blocked')),
    created_at               timestamptz NOT NULL,
    updated_at               timestamptz NOT NULL,
    CONSTRAINT organization_inn_key UNIQUE (inn),
    CONSTRAINT organization_inn_length CHECK (
        (legal_type = 'legal_entity' AND inn ~ '^[0-9]{10}$') OR
        (legal_type <> 'legal_entity' AND inn ~ '^[0-9]{12}$')),
    CONSTRAINT organization_tax_document CHECK (
        (tax_document_storage_key IS NULL) = (tax_document_sha256 IS NULL))
);

-- Организация-платформа — единственная строка (04 §2).
CREATE UNIQUE INDEX organization_is_platform_key ON organization (is_platform) WHERE is_platform;

CREATE TABLE organization_member (
    id              uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organization (id),
    user_id         uuid NOT NULL REFERENCES "user" (id),
    role            text NOT NULL CHECK (role IN ('owner')),
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    CONSTRAINT organization_member_organization_id_user_id_key UNIQUE (organization_id, user_id),
    -- A-13: в первой версии у пользователя одна организация.
    CONSTRAINT organization_member_user_id_key UNIQUE (user_id)
);

CREATE TABLE advertiser_profile (
    organization_id uuid PRIMARY KEY REFERENCES organization (id),
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL
);

CREATE TABLE blogger_profile (
    organization_id uuid PRIMARY KEY REFERENCES organization (id),
    full_name       text,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL
);

CREATE TABLE auth_code (
    id              uuid PRIMARY KEY,
    purpose         text NOT NULL CHECK (purpose IN ('login', 'channel_link')),
    email           citext,
    organization_id uuid REFERENCES organization (id),
    code_hash       text NOT NULL,
    expires_at      timestamptz NOT NULL,
    attempts        integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    consumed_at     timestamptz,
    -- R-2: адрес, с которого запрошен код, — для лимита кодов на IP в час (07 §4).
    requested_ip    inet,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    CONSTRAINT auth_code_target CHECK (
        (purpose = 'login' AND email IS NOT NULL) OR
        (purpose = 'channel_link' AND organization_id IS NOT NULL))
);

CREATE INDEX auth_code_email_created_at_idx ON auth_code (email, created_at);
CREATE INDEX auth_code_requested_ip_created_at_idx ON auth_code (requested_ip, created_at);

CREATE TABLE session (
    id         uuid PRIMARY KEY,
    user_id    uuid NOT NULL REFERENCES "user" (id),
    token_hash text NOT NULL,
    expires_at timestamptz NOT NULL,
    ip         inet,
    user_agent text,
    revoked_at timestamptz,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    CONSTRAINT session_token_hash_key UNIQUE (token_hash)
);

CREATE INDEX session_user_id_idx ON session (user_id);

CREATE TABLE legal_document (
    id           uuid PRIMARY KEY,
    kind         text NOT NULL CHECK (kind IN ('advertiser_offer', 'blogger_offer', 'platform_rules', 'privacy_policy')),
    version      integer NOT NULL CHECK (version > 0),
    published_at timestamptz NOT NULL,
    storage_key  text NOT NULL,
    sha256       text NOT NULL,
    created_at   timestamptz NOT NULL,
    CONSTRAINT legal_document_kind_version_key UNIQUE (kind, version)
);

-- Неизменяемая (04 §1).
CREATE TABLE legal_acceptance (
    id                uuid PRIMARY KEY,
    legal_document_id uuid NOT NULL REFERENCES legal_document (id),
    user_id           uuid NOT NULL REFERENCES "user" (id),
    organization_id   uuid NOT NULL REFERENCES organization (id),
    accepted_at       timestamptz NOT NULL,
    ip                inet,
    created_at        timestamptz NOT NULL,
    CONSTRAINT legal_acceptance_organization_id_legal_document_id_key UNIQUE (organization_id, legal_document_id)
);

-- Неизменяемая (04 §1, §8).
CREATE TABLE audit_log (
    id              uuid PRIMARY KEY,
    actor_type      text NOT NULL CHECK (actor_type IN ('advertiser', 'blogger', 'admin', 'system')),
    actor_user_id   uuid REFERENCES "user" (id),
    organization_id uuid REFERENCES organization (id),
    action          text NOT NULL,
    object_type     text NOT NULL,
    object_id       text NOT NULL,
    reason          text,
    data            jsonb NOT NULL,
    ip              inet,
    created_at      timestamptz NOT NULL
);

CREATE INDEX audit_log_object_idx ON audit_log (object_type, object_id);
CREATE INDEX audit_log_organization_id_created_at_idx ON audit_log (organization_id, created_at);

CREATE TABLE job (
    id           uuid PRIMARY KEY,
    kind         text NOT NULL,
    dedupe_key   text,
    payload      jsonb NOT NULL,
    run_at       timestamptz NOT NULL,
    status       text NOT NULL CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'cancelled')),
    attempts     integer NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    max_attempts integer NOT NULL CHECK (max_attempts > 0),
    last_error   text,
    locked_by    text,
    locked_at    timestamptz,
    finished_at  timestamptz,
    created_at   timestamptz NOT NULL,
    updated_at   timestamptz NOT NULL
);

CREATE INDEX job_status_run_at_idx ON job (status, run_at);
-- 07 §2: dedupe_key уникален среди живых заданий.
CREATE UNIQUE INDEX job_dedupe_key_live_key ON job (dedupe_key) WHERE status IN ('pending', 'running');

-- +goose Down
DROP TABLE job;
DROP TABLE audit_log;
DROP TABLE legal_acceptance;
DROP TABLE legal_document;
DROP TABLE session;
DROP TABLE auth_code;
DROP TABLE blogger_profile;
DROP TABLE advertiser_profile;
DROP TABLE organization_member;
DROP TABLE organization;
DROP TABLE "user";
