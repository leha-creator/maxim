-- Заказы и размещения (04 §5, 03, этап «Заказ и размещения»): корзина, заказ, размещение
-- и неизменяемая история его состояний; ссылка кампании на заказ (R-1). Права роли
-- приложения выдаёт ALTER DEFAULT PRIVILEGES из 00002; история — только добавление (04 §1).

-- +goose Up
CREATE TABLE cart_item (
    id                 uuid PRIMARY KEY,
    organization_id    uuid NOT NULL REFERENCES organization (id),
    channel_id         uuid NOT NULL REFERENCES channel (id),
    format_id          text NOT NULL REFERENCES placement_format (id),
    -- Итог с комиссией, показанный рекламодателю; сравнивается при оформлении (R-7).
    shown_total_amount bigint NOT NULL CHECK (shown_total_amount > 0),
    created_by         uuid NOT NULL REFERENCES "user" (id),
    created_at         timestamptz NOT NULL,
    updated_at         timestamptz NOT NULL,
    -- R-8: один канал — одна позиция корзины организации.
    CONSTRAINT cart_item_organization_id_channel_id_key UNIQUE (organization_id, channel_id)
);

CREATE INDEX cart_item_channel_id_idx ON cart_item (channel_id);

-- R-21: номер заказа — человекочитаемый, с 1001; берётся до создания версии поста (UTM-метки).
CREATE SEQUENCE order_number_seq START WITH 1001;

CREATE TABLE "order" (
    id                      uuid PRIMARY KEY,
    number                  bigint NOT NULL,
    organization_id         uuid NOT NULL REFERENCES organization (id),
    campaign_id             uuid NOT NULL REFERENCES campaign (id),
    post_id                 uuid NOT NULL REFERENCES post (id),
    current_post_version_id uuid NOT NULL REFERENCES post_version (id),
    -- Производное состояние (03 §1.2), пересчитывается после каждого перехода.
    status                  text NOT NULL CHECK (status IN (
        'moderation', 'rejected', 'marking', 'in_progress', 'completed', 'cancelled')),
    total_amount            bigint NOT NULL CHECK (total_amount >= 0),
    -- R-18: ключ идемпотентности оформления.
    idempotency_key         text CHECK (char_length(idempotency_key) BETWEEN 1 AND 64),
    -- R-18: ключ последней правки поста после оформления (S-29): повтор возвращает заказ.
    post_version_idempotency_key text CHECK (char_length(post_version_idempotency_key) BETWEEN 1 AND 64),
    created_by              uuid NOT NULL REFERENCES "user" (id),
    created_at              timestamptz NOT NULL,
    updated_at              timestamptz NOT NULL,
    CONSTRAINT order_number_key UNIQUE (number),
    -- D-16: в первой версии у кампании один заказ.
    CONSTRAINT order_campaign_id_key UNIQUE (campaign_id),
    CONSTRAINT order_organization_id_idempotency_key_key UNIQUE (organization_id, idempotency_key)
);

-- Сброс номера вместе с таблицей (TRUNCATE … RESTART IDENTITY в тестах).
ALTER SEQUENCE order_number_seq OWNED BY "order".number;

CREATE INDEX order_organization_list_idx ON "order" (organization_id, created_at DESC, id DESC);
CREATE INDEX order_current_post_version_id_idx ON "order" (current_post_version_id);

-- R-1: кампания знает свой заказ; post о заказах не знает, поле пишет post/app.MarkCampaignOrdered.
ALTER TABLE campaign ADD COLUMN order_id uuid REFERENCES "order" (id);
ALTER TABLE campaign ADD CONSTRAINT campaign_ordered_has_order CHECK (status = 'draft' OR order_id IS NOT NULL);

CREATE TABLE placement (
    id                      uuid PRIMARY KEY,
    order_id                uuid NOT NULL REFERENCES "order" (id),
    channel_id              uuid NOT NULL REFERENCES channel (id),
    -- Снимок владельца канала при создании.
    blogger_organization_id uuid NOT NULL REFERENCES organization (id),
    -- Снимок формата (03 §2).
    format_id               text NOT NULL REFERENCES placement_format (id),
    top_hours               integer NOT NULL CHECK (top_hours > 0),
    duration_hours          integer NOT NULL CHECK (duration_hours >= top_hours),
    -- R-9: снимок периода кампании; window_end — 00:00 МСК дня после date_to.
    window_start            timestamptz NOT NULL,
    window_end              timestamptz NOT NULL,
    time_from               time NOT NULL,
    time_to                 time NOT NULL,
    post_version_id         uuid NOT NULL REFERENCES post_version (id),
    status                  text NOT NULL CHECK (status IN (
        'created', 'awaiting_blogger', 'awaiting_schedule', 'scheduled', 'published', 'removed', 'disputed',
        'accepted', 'paid', 'declined', 'expired', 'cancelled', 'failed', 'refunded')),
    status_changed_at       timestamptz NOT NULL,
    status_reason           text,
    -- R-16: первый P-02; с него размещение видно блогеру.
    sent_to_blogger_at      timestamptz,
    fee_rate_bp             integer NOT NULL CHECK (fee_rate_bp BETWEEN 0 AND 10000),
    -- I-05: суммы неизменяемы (триггер placement_amounts_immutable).
    blogger_amount          bigint NOT NULL CHECK (blogger_amount >= 0),
    fee_amount              bigint NOT NULL CHECK (fee_amount >= 0),
    total_amount            bigint NOT NULL CHECK (total_amount >= 0),
    accept_deadline_at      timestamptz,
    schedule_deadline_at    timestamptz,
    publish_at              timestamptz,
    remove_at               timestamptz,
    published_at            timestamptz,
    removed_at              timestamptz,
    claim_deadline_at       timestamptz,
    performance_share       integer CHECK (performance_share BETWEEN 0 AND 10000),
    payout_amount           bigint CHECK (payout_amount >= 0),
    fee_charged             bigint CHECK (fee_charged >= 0),
    refund_amount           bigint CHECK (refund_amount >= 0),
    final_views             integer CHECK (final_views >= 0),
    lock_version            integer NOT NULL DEFAULT 0,
    created_at              timestamptz NOT NULL,
    updated_at              timestamptz NOT NULL,
    -- I-11 в части размещения: один канал — одно размещение заказа (D-02).
    CONSTRAINT placement_order_id_channel_id_key UNIQUE (order_id, channel_id),
    CONSTRAINT placement_amounts_sum CHECK (total_amount = blogger_amount + fee_amount),
    CONSTRAINT placement_window_order CHECK (window_start < window_end),
    CONSTRAINT placement_time_order CHECK (time_from < time_to),
    CONSTRAINT placement_schedule_pair CHECK ((publish_at IS NULL) = (remove_at IS NULL) AND
        (publish_at IS NULL OR publish_at < remove_at)),
    CONSTRAINT placement_scheduled_has_time CHECK (status <> 'scheduled' OR publish_at IS NOT NULL)
);

CREATE INDEX placement_channel_id_status_idx ON placement (channel_id, status);
CREATE INDEX placement_status_accept_deadline_at_idx ON placement (status, accept_deadline_at);
CREATE INDEX placement_order_id_idx ON placement (order_id);
CREATE INDEX placement_post_version_id_idx ON placement (post_version_id);
-- R-16: кабинет блогера, новые сверху; до первого P-02 размещение блогеру не видно.
CREATE INDEX placement_blogger_list_idx ON placement (blogger_organization_id, created_at DESC, id DESC)
    WHERE sent_to_blogger_at IS NOT NULL;
-- 07 §3: зависшие в scheduled.
CREATE INDEX placement_scheduled_publish_at_idx ON placement (publish_at) WHERE status = 'scheduled';

-- 03 §12: история состояний — только добавление (права роли ниже).
CREATE TABLE placement_state_history (
    id                  uuid PRIMARY KEY,
    placement_id        uuid NOT NULL REFERENCES placement (id),
    transition          text NOT NULL CHECK (transition ~ '^P-[0-9]{2}$'),
    from_status         text CHECK (from_status IN (
        'created', 'awaiting_blogger', 'awaiting_schedule', 'scheduled', 'published', 'removed', 'disputed',
        'accepted', 'paid', 'declined', 'expired', 'cancelled', 'failed', 'refunded')),
    to_status           text NOT NULL CHECK (to_status IN (
        'created', 'awaiting_blogger', 'awaiting_schedule', 'scheduled', 'published', 'removed', 'disputed',
        'accepted', 'paid', 'declined', 'expired', 'cancelled', 'failed', 'refunded')),
    actor_type          text NOT NULL CHECK (actor_type IN ('advertiser', 'blogger', 'admin', 'system')),
    actor_user_id       uuid REFERENCES "user" (id),
    reason_code         text,
    -- Комментарий блогера при отказе; в журнал приложения не пишется (07 §5.4).
    reason_text         text CHECK (char_length(reason_text) <= 1000),
    -- Источник перехода. Без внешних ключей: задания и запросы очищаются по сроку хранения.
    job_id              uuid,
    inbound_event_id    uuid,
    outbound_request_id uuid,
    -- R-18: ключ идемпотентности действия пользователя.
    idempotency_key     text CHECK (char_length(idempotency_key) BETWEEN 1 AND 64),
    created_at          timestamptz NOT NULL,
    CONSTRAINT placement_state_history_placement_id_idempotency_key_key UNIQUE (placement_id, idempotency_key)
);

CREATE INDEX placement_state_history_placement_id_created_at_idx ON placement_state_history (placement_id, created_at);

-- R-20, I-04: конечное состояние размещения не покидается ни кодом, ни прямым запросом.
-- +goose StatementBegin
CREATE FUNCTION placement_terminal_guard() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.status IN ('paid', 'declined', 'expired', 'cancelled', 'failed', 'refunded')
        AND NEW.status IS DISTINCT FROM OLD.status THEN
        RAISE EXCEPTION 'placement % is in terminal status %', OLD.id, OLD.status
            USING ERRCODE = 'check_violation', CONSTRAINT = 'placement_terminal_guard', TABLE = 'placement';
    END IF;
    RETURN NEW;
END
$$;
-- +goose StatementEnd

CREATE TRIGGER placement_terminal_guard
    BEFORE UPDATE ON placement
    FOR EACH ROW EXECUTE FUNCTION placement_terminal_guard();

-- R-20, I-05: суммы и ставка комиссии размещения после создания не меняются.
-- +goose StatementBegin
CREATE FUNCTION placement_amounts_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.blogger_amount IS DISTINCT FROM OLD.blogger_amount
        OR NEW.fee_amount IS DISTINCT FROM OLD.fee_amount
        OR NEW.total_amount IS DISTINCT FROM OLD.total_amount
        OR NEW.fee_rate_bp IS DISTINCT FROM OLD.fee_rate_bp THEN
        RAISE EXCEPTION 'placement % amounts are immutable', OLD.id
            USING ERRCODE = 'check_violation', CONSTRAINT = 'placement_amounts_immutable', TABLE = 'placement';
    END IF;
    RETURN NEW;
END
$$;
-- +goose StatementEnd

CREATE TRIGGER placement_amounts_immutable
    BEFORE UPDATE ON placement
    FOR EACH ROW EXECUTE FUNCTION placement_amounts_immutable();

-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON placement_state_history FROM %I', app_role);
    EXECUTE format('GRANT USAGE, SELECT ON SEQUENCE order_number_seq TO %I', app_role);
END
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('REVOKE ALL ON SEQUENCE order_number_seq FROM %I', app_role);
END
$$;
-- +goose StatementEnd
DROP TRIGGER placement_amounts_immutable ON placement;
DROP FUNCTION placement_amounts_immutable();
DROP TRIGGER placement_terminal_guard ON placement;
DROP FUNCTION placement_terminal_guard();
DROP TABLE placement_state_history;
DROP TABLE placement;
ALTER TABLE campaign DROP CONSTRAINT campaign_ordered_has_order;
ALTER TABLE campaign DROP COLUMN order_id;
DROP TABLE "order";
DROP SEQUENCE IF EXISTS order_number_seq;
DROP TABLE cart_item;
