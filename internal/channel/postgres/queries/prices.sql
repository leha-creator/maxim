-- Цены каналов по форматам (04 §3, S-13, R-5).

-- name: ListPricesByChannels :many
SELECT p.channel_id, p.format_id, p.amount, p.is_enabled, f.is_active AS format_active
FROM channel_price p JOIN placement_format f ON f.id = p.format_id
WHERE p.channel_id = ANY(@channel_ids::uuid[])
ORDER BY p.channel_id, f.position, f.id;

-- name: DeletePricesExcept :exec
-- Формат, которого нет в новом наборе, удаляется (R-5). Проверку active закрепляет
-- отложенный триггер channel_price_keeps_active (R-3).
DELETE FROM channel_price
WHERE channel_id = @channel_id AND NOT (format_id = ANY(@format_ids::text[]));

-- name: UpsertPrice :exec
INSERT INTO channel_price (id, channel_id, format_id, amount, is_enabled, created_at, updated_at)
VALUES (@id, @channel_id, @format_id, @amount, @is_enabled, @now::timestamptz, @now::timestamptz)
ON CONFLICT (channel_id, format_id) DO UPDATE
SET amount = EXCLUDED.amount, is_enabled = EXCLUDED.is_enabled, updated_at = EXCLUDED.updated_at;
