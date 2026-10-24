-- Приёмка, споры и возвраты (03 §4, §6, §7, 04 §5, этап «Приёмка, споры и возвраты»): претензии
-- рекламодателя, ключ спора на претензию, согласованность решения и доли. Претензия неизменяема;
-- споры не удаляются (основание денег, 06 §2.2).

-- +goose Up
-- 04 §5, S-08: одна претензия на размещение (R-1).
CREATE TABLE claim (
    id           uuid PRIMARY KEY,
    placement_id uuid NOT NULL REFERENCES placement (id),
    reason       text NOT NULL CHECK (reason IN ('removed_early', 'top_violated', 'content_mismatch', 'other')),
    text         text NOT NULL CHECK (char_length(text) BETWEEN 1 AND 2000),
    created_by   uuid NOT NULL REFERENCES "user" (id),
    created_at   timestamptz NOT NULL,
    CONSTRAINT claim_placement_id_key UNIQUE (placement_id)
);

-- Основание спора — претензия (00008 оставил поле без внешнего ключа до этого этапа).
ALTER TABLE dispute ADD CONSTRAINT dispute_claim_id_fkey FOREIGN KEY (claim_id) REFERENCES claim (id) NOT VALID;
ALTER TABLE dispute VALIDATE CONSTRAINT dispute_claim_id_fkey;

-- 03 §4 «Исходы P-15»: доля соответствует решению; решение принимает администратор (R-8).
ALTER TABLE dispute ADD CONSTRAINT dispute_resolution_share CHECK (
    resolution IS NULL
    OR (resolution = 'full_payout' AND performance_share = 10000)
    OR (resolution = 'full_refund' AND performance_share = 0)
    OR (resolution = 'proportional' AND performance_share BETWEEN 1 AND 9999));
ALTER TABLE dispute ADD CONSTRAINT dispute_resolution_by CHECK (resolution IS NULL OR resolved_by IS NOT NULL);
ALTER TABLE dispute ADD CONSTRAINT dispute_resolution_comment_length CHECK (char_length(resolution_comment) <= 2000);

-- Очередь открытых споров по сроку T-06 (команда admin dispute list).
CREATE INDEX dispute_open_deadline_idx ON dispute (deadline_at) WHERE resolution IS NULL;

-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('REVOKE UPDATE, DELETE, TRUNCATE ON claim FROM %I', app_role);
    EXECUTE format('REVOKE DELETE, TRUNCATE ON dispute FROM %I', app_role);
END
$$;
-- +goose StatementEnd

-- +goose Down
DROP INDEX dispute_open_deadline_idx;
ALTER TABLE dispute DROP CONSTRAINT dispute_resolution_comment_length;
ALTER TABLE dispute DROP CONSTRAINT dispute_resolution_by;
ALTER TABLE dispute DROP CONSTRAINT dispute_resolution_share;
ALTER TABLE dispute DROP CONSTRAINT dispute_claim_id_fkey;
DROP TABLE claim;
-- +goose StatementBegin
DO $$
DECLARE
    app_role text := current_setting('maxim.app_role');
BEGIN
    EXECUTE format('GRANT DELETE ON dispute TO %I', app_role);
END
$$;
-- +goose StatementEnd
