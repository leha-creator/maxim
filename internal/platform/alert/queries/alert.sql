-- Оповещения администратору (07 §3, R-4).

-- name: UpsertAlert :one
INSERT INTO alert (id, reason, object_key, level, summary, first_raised_at, last_raised_at, raised_count)
VALUES (@id, @reason, @object_key, @level, @summary, @now::timestamptz, @now::timestamptz, 1)
ON CONFLICT (reason, object_key) DO UPDATE
SET level = EXCLUDED.level, summary = EXCLUDED.summary, last_raised_at = EXCLUDED.last_raised_at,
    raised_count = alert.raised_count + 1, resolved_at = NULL
RETURNING id, raised_count, last_sent_at;

-- name: ClaimAlertSend :execrows
-- Отправка не чаще раза в интервал по причине и объекту: условное обновление решает гонку экземпляров.
UPDATE alert SET last_sent_at = @now::timestamptz
WHERE id = @id AND (last_sent_at IS NULL OR last_sent_at <= @sent_before::timestamptz);

-- name: GetAlert :one
SELECT * FROM alert WHERE id = @id;

-- name: ResolveAlert :execrows
UPDATE alert SET resolved_at = @now::timestamptz
WHERE reason = @reason AND object_key = @object_key AND resolved_at IS NULL;

-- name: OpenAlertKeys :many
SELECT object_key FROM alert WHERE reason = @reason AND resolved_at IS NULL;

-- name: DeleteResolvedAlertsBefore :execrows
DELETE FROM alert
WHERE id IN (SELECT a.id FROM alert a WHERE a.resolved_at < @before::timestamptz LIMIT @batch);
