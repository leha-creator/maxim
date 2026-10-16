-- Публикации (04 §5): сообщение в канале по размещению. У размещения не больше одной (I-11).

-- name: InsertPublication :exec
INSERT INTO publication (
    id, placement_id, channel_id, max_message_id, url, published_at, removed_at, removal_reason, removal_overdue,
    views, views_checked_at, content_hash, created_at, updated_at
) VALUES (
    @id, @placement_id, @channel_id, @max_message_id, @url, @published_at, @removed_at, @removal_reason, @removal_overdue,
    @views, @views_checked_at, @content_hash, @created_at, @updated_at
);

-- name: GetPublicationByPlacement :one
SELECT * FROM publication WHERE placement_id = @placement_id;

-- name: GetPublicationByPlacementForUpdate :one
SELECT * FROM publication WHERE placement_id = @placement_id FOR UPDATE;

-- name: GetPublicationForUpdate :one
SELECT * FROM publication WHERE id = @id FOR UPDATE;

-- name: GetPublicationByMessageID :one
SELECT * FROM publication WHERE max_message_id = @max_message_id;

-- name: UpdatePublication :exec
UPDATE publication SET
    removed_at = @removed_at, removal_reason = @removal_reason, removal_overdue = @removal_overdue,
    views = @views, views_checked_at = @views_checked_at, updated_at = @updated_at
WHERE id = @id;

-- name: ListLivePublications :many
-- Сверка (T-09): живые публикации по id, страницами.
SELECT * FROM publication WHERE removed_at IS NULL AND id > @after ORDER BY id LIMIT @lim;

-- name: ListLivePublicationsByChannel :many
-- Контроль топа (S-27): живые публикации канала с размещениями в published или disputed.
SELECT sqlc.embed(pub), sqlc.embed(p) FROM publication pub JOIN placement p ON p.id = pub.placement_id
WHERE pub.channel_id = @channel_id AND pub.removed_at IS NULL AND p.status IN ('published', 'disputed')
ORDER BY pub.published_at;

-- name: ListPublicationsByPlacements :many
SELECT * FROM publication WHERE placement_id = ANY(@placement_ids::uuid[]);

-- name: OverdueRemovals :many
-- 07 §3: публикации с removal_overdue дольше порога.
SELECT id FROM publication WHERE removal_overdue AND updated_at < @before ORDER BY updated_at LIMIT 1000;
