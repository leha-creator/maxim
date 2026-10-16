-- Входящие события внешних систем (04 §8, 05 §2.2).

-- name: InsertInboundEvent :execrows
-- Повтор события с тем же ключом не создаёт вторую строку.
INSERT INTO inbound_event (id, source, external_key, event_type, payload, status, created_at, updated_at)
VALUES (@id, @source, @external_key, @event_type, @payload, 'received', @now::timestamptz, @now::timestamptz)
ON CONFLICT (source, external_key) DO NOTHING;

-- name: GetInboundEvent :one
SELECT * FROM inbound_event WHERE id = @id;

-- name: FinishInboundEvent :execrows
UPDATE inbound_event
SET status = @status, error = @error, processed_at = @now::timestamptz, updated_at = @now::timestamptz
WHERE id = @id AND status = 'received';

-- name: StaleInboundEvents :many
-- События в received дольше порога (07 §3, зависшие объекты).
SELECT id, source, event_type, created_at FROM inbound_event
WHERE status = 'received' AND created_at < @before::timestamptz
ORDER BY created_at
LIMIT 500;

-- name: OldestReceivedInboundEvent :many
SELECT source, min(created_at)::timestamptz AS oldest
FROM inbound_event
WHERE status = 'received'
GROUP BY source;

-- name: DeleteInboundEventsBefore :execrows
DELETE FROM inbound_event
WHERE id IN (SELECT e.id FROM inbound_event e
             WHERE e.status IN ('processed', 'ignored') AND e.created_at < @before::timestamptz LIMIT @batch);
