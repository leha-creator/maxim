-- Медиатека, кампании и пост (04 §4, этап «Медиатека и пост»): оригиналы и производные
-- изображений, кампания с единым постом, версии поста. Права роли приложения выдаёт
-- ALTER DEFAULT PRIVILEGES из 00002.

-- +goose Up
CREATE TABLE media_file (
    id              uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organization (id),
    -- Ключ в закрытой области хранилища. Оригинал не меняется (D-04).
    storage_key     text NOT NULL UNIQUE,
    mime            text NOT NULL CHECK (mime IN ('image/jpeg', 'image/png', 'image/webp')),
    size_bytes      bigint NOT NULL CHECK (size_bytes BETWEEN 1 AND 52428800),
    -- R-3: размеры после поворота по EXIF.
    width           integer NOT NULL CHECK (width BETWEEN 1 AND 7680),
    height          integer NOT NULL CHECK (height BETWEEN 1 AND 7680),
    sha256          text NOT NULL,
    original_name   text NOT NULL,
    created_by      uuid NOT NULL REFERENCES "user" (id),
    created_at      timestamptz NOT NULL,
    deleted_at      timestamptz,
    -- R-19: объект удалён из хранилища заданием purge_media.
    purged_at       timestamptz,
    CONSTRAINT media_file_purged_after_delete CHECK (purged_at IS NULL OR deleted_at IS NOT NULL)
);

-- R-2: повтор того же файла в организации возвращает существующую запись.
CREATE UNIQUE INDEX media_file_org_sha256_key ON media_file (organization_id, sha256) WHERE deleted_at IS NULL;
-- R-21: список медиатеки, новые сверху.
CREATE INDEX media_file_org_list_idx ON media_file (organization_id, created_at DESC, id DESC) WHERE deleted_at IS NULL;
CREATE INDEX media_file_purge_idx ON media_file (deleted_at) WHERE deleted_at IS NOT NULL AND purged_at IS NULL;

CREATE TABLE media_derivative (
    id                 uuid PRIMARY KEY,
    media_file_id      uuid NOT NULL REFERENCES media_file (id),
    kind               text NOT NULL CHECK (kind IN ('preview', 'plain', 'labeled')),
    -- R-5: sha256 параметров построения (вид, версия шаблона, текст плашки).
    params_hash        text NOT NULL,
    -- R-6: pending — задание build_media_derivative ещё не построило файл.
    status             text NOT NULL CHECK (status IN ('pending', 'ready')),
    storage_key        text UNIQUE,
    mime               text CHECK (mime IN ('image/jpeg', 'image/png')),
    width              integer CHECK (width BETWEEN 1 AND 7680),
    height             integer CHECK (height BETWEEN 1 AND 7680),
    size_bytes         bigint CHECK (size_bytes BETWEEN 1 AND 52428800),
    -- R-7: копия в публичной области; появляется у производных, вошедших в версию поста.
    public_storage_key text UNIQUE,
    public_url         text,
    published_at       timestamptz,
    created_at         timestamptz NOT NULL,
    purged_at          timestamptz,
    CONSTRAINT media_derivative_triple_key UNIQUE (media_file_id, kind, params_hash),
    CONSTRAINT media_derivative_ready_fields CHECK (status <> 'ready' OR (
        storage_key IS NOT NULL AND mime IS NOT NULL AND width IS NOT NULL
        AND height IS NOT NULL AND size_bytes IS NOT NULL)),
    CONSTRAINT media_derivative_published_fields CHECK (published_at IS NULL OR (
        status = 'ready' AND public_storage_key IS NOT NULL AND public_url IS NOT NULL))
);

CREATE INDEX media_derivative_purge_idx ON media_derivative (created_at) WHERE purged_at IS NULL;

CREATE TABLE campaign (
    id                 uuid PRIMARY KEY,
    organization_id    uuid NOT NULL REFERENCES organization (id),
    name               text NOT NULL CHECK (char_length(name) BETWEEN 1 AND 200),
    status             text NOT NULL CHECK (status IN ('draft', 'ordered', 'archived')),
    -- A-39: период дат и времени суток (МСК); обязательны к оплате.
    date_from          date,
    date_to            date,
    time_from          time,
    time_to            time,
    extra_requirements text CHECK (char_length(extra_requirements) <= 2000),
    created_by         uuid NOT NULL REFERENCES "user" (id),
    created_at         timestamptz NOT NULL,
    updated_at         timestamptz NOT NULL,
    CONSTRAINT campaign_period_order CHECK (date_from <= date_to),
    CONSTRAINT campaign_time_order CHECK (time_from < time_to),
    CONSTRAINT campaign_time_pair CHECK ((time_from IS NULL) = (time_to IS NULL))
);

CREATE INDEX campaign_org_list_idx ON campaign (organization_id, created_at DESC, id DESC);

CREATE TABLE post (
    id              uuid PRIMARY KEY,
    organization_id uuid NOT NULL REFERENCES organization (id),
    -- D-16: у кампании один пост.
    campaign_id     uuid NOT NULL UNIQUE REFERENCES campaign (id),
    status          text NOT NULL CHECK (status IN ('draft', 'archived')),
    -- Документ редактора, файлы по порядку, место токена; формат — post/postgres.
    draft           jsonb NOT NULL,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL
);

-- R-15: удаление файла убирает его из черновиков всех кампаний организации.
CREATE INDEX post_organization_id_idx ON post (organization_id);

CREATE TABLE post_version (
    id               uuid PRIMARY KEY,
    post_id          uuid NOT NULL REFERENCES post (id),
    number           integer NOT NULL CHECK (number >= 1),
    status           text NOT NULL CHECK (status IN ('pending_moderation', 'rejected', 'approved', 'ready')),
    content_doc      jsonb NOT NULL,
    -- A-38: страховка; точный предел в кодовых единицах UTF-16 проверяет post/domain.
    body             text NOT NULL CHECK (char_length(body) <= 3800),
    final_text       text CHECK (char_length(final_text) <= 4000),
    erid             text,
    token_carrier    text NOT NULL CHECK (token_carrier IN ('text', 'media')),
    token_position   text NOT NULL CHECK (token_position IN ('link', 'above', 'below')),
    advertiser_name  text NOT NULL,
    advertiser_inn   text NOT NULL,
    moderated_by     uuid REFERENCES "user" (id),
    moderated_at     timestamptz,
    rejection_reason text,
    lock_version     integer NOT NULL DEFAULT 0,
    created_by       uuid NOT NULL REFERENCES "user" (id),
    created_at       timestamptz NOT NULL,
    CONSTRAINT post_version_number_key UNIQUE (post_id, number),
    CONSTRAINT post_version_ready_fields CHECK (status <> 'ready' OR (final_text IS NOT NULL AND erid IS NOT NULL)),
    CONSTRAINT post_version_rejected_reason CHECK (status <> 'rejected' OR rejection_reason IS NOT NULL)
);

CREATE TABLE post_version_media (
    id                  uuid PRIMARY KEY,
    post_version_id     uuid NOT NULL REFERENCES post_version (id),
    position            integer NOT NULL CHECK (position BETWEEN 0 AND 9),
    media_file_id       uuid NOT NULL REFERENCES media_file (id),
    -- Файл, который уходит в канал: labeled или plain.
    media_derivative_id uuid NOT NULL REFERENCES media_derivative (id),
    created_at          timestamptz NOT NULL,
    CONSTRAINT post_version_media_position_key UNIQUE (post_version_id, position)
);

CREATE INDEX post_version_media_file_idx ON post_version_media (media_file_id);
CREATE INDEX post_version_media_derivative_idx ON post_version_media (media_derivative_id);

-- R-18, I-09: версия в ready и её изображения не меняются ни кодом, ни прямым запросом (04 §1).
-- +goose StatementBegin
CREATE FUNCTION post_version_ready_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    version_id uuid;
BEGIN
    IF TG_TABLE_NAME = 'post_version' THEN
        IF OLD.status = 'ready' THEN
            RAISE EXCEPTION 'post_version % is ready and immutable', OLD.id
                USING ERRCODE = 'check_violation', CONSTRAINT = 'post_version_ready_immutable', TABLE = 'post_version';
        END IF;
    ELSE
        IF TG_OP = 'INSERT' THEN
            version_id := NEW.post_version_id;
        ELSE
            version_id := OLD.post_version_id;
        END IF;
        IF EXISTS (SELECT 1 FROM post_version WHERE id = version_id AND status = 'ready') THEN
            RAISE EXCEPTION 'post_version % is ready and immutable', version_id
                USING ERRCODE = 'check_violation', CONSTRAINT = 'post_version_ready_immutable', TABLE = 'post_version_media';
        END IF;
    END IF;
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END
$$;
-- +goose StatementEnd

CREATE TRIGGER post_version_ready_immutable
    BEFORE UPDATE OR DELETE ON post_version
    FOR EACH ROW EXECUTE FUNCTION post_version_ready_immutable();

CREATE TRIGGER post_version_media_ready_immutable
    BEFORE INSERT OR UPDATE OR DELETE ON post_version_media
    FOR EACH ROW EXECUTE FUNCTION post_version_ready_immutable();

-- +goose Down
DROP TRIGGER post_version_media_ready_immutable ON post_version_media;
DROP TRIGGER post_version_ready_immutable ON post_version;
DROP FUNCTION post_version_ready_immutable();
DROP TABLE post_version_media;
DROP TABLE post_version;
DROP TABLE post;
DROP TABLE campaign;
DROP TABLE media_derivative;
DROP TABLE media_file;
