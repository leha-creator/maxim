-- Интервалы недоступности каналов (04 §3, S-13, R-6).

-- name: ListLiveUnavailability :many
SELECT id, starts_at, ends_at FROM channel_unavailability
WHERE channel_id = @channel_id AND ends_at > @now::timestamptz
ORDER BY starts_at, id;

-- name: InsertUnavailability :exec
INSERT INTO channel_unavailability (id, channel_id, starts_at, ends_at, created_at)
VALUES (@id, @channel_id, @starts_at, @ends_at, @created_at);

-- name: DeleteUnavailability :execrows
DELETE FROM channel_unavailability WHERE id = @id AND channel_id = @channel_id;

-- name: DeleteEndedUnavailability :execrows
-- Задание cleanup: интервалы, закончившиеся раньше before (R-6: через 30 суток).
DELETE FROM channel_unavailability
WHERE id IN (SELECT u.id FROM channel_unavailability u WHERE u.ends_at < @before::timestamptz LIMIT @batch);
