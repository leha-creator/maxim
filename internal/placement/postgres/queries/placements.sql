-- Размещения (04 §5, 03). Состояние меняется только обновлением «если состояние и
-- lock_version равны ожидаемым» (03 §11) после placement/domain.Placement.Apply.

-- name: InsertPlacement :exec
INSERT INTO placement (
    id, order_id, channel_id, blogger_organization_id, format_id, top_hours, duration_hours,
    window_start, window_end, time_from, time_to, post_version_id, status, status_changed_at, status_reason,
    sent_to_blogger_at, fee_rate_bp, blogger_amount, fee_amount, total_amount, accept_deadline_at,
    schedule_deadline_at, publish_at, remove_at, lock_version, created_at, updated_at
) VALUES (
    @id, @order_id, @channel_id, @blogger_organization_id, @format_id, @top_hours, @duration_hours,
    @window_start, @window_end, @time_from, @time_to, @post_version_id, @status, @status_changed_at, @status_reason,
    @sent_to_blogger_at, @fee_rate_bp, @blogger_amount, @fee_amount, @total_amount, @accept_deadline_at,
    @schedule_deadline_at, @publish_at, @remove_at, @lock_version, @created_at, @updated_at
);

-- name: GetPlacement :one
SELECT * FROM placement WHERE id = @id;

-- name: GetPlacementForUpdate :one
SELECT * FROM placement WHERE id = @id FOR UPDATE;

-- name: GetAdvertiserPlacement :one
SELECT p.* FROM placement p JOIN "order" o ON o.id = p.order_id
WHERE p.id = @id AND o.organization_id = @organization_id;

-- name: GetAdvertiserPlacementForUpdate :one
-- Размещение заказа организации рекламодателя (07 §4).
SELECT p.* FROM placement p JOIN "order" o ON o.id = p.order_id
WHERE p.id = @id AND o.organization_id = @organization_id
FOR UPDATE OF p;

-- name: GetBloggerPlacement :one
-- R-16: блогер видит размещение, отправленное ему хотя бы раз.
SELECT * FROM placement
WHERE id = @id AND blogger_organization_id = @organization_id AND sent_to_blogger_at IS NOT NULL;

-- name: GetBloggerPlacementForUpdate :one
SELECT * FROM placement
WHERE id = @id AND blogger_organization_id = @organization_id AND sent_to_blogger_at IS NOT NULL
FOR UPDATE;

-- name: UpdatePlacement :execrows
-- Суммы и снимки не обновляются (I-05); условие — ожидаемые состояние и lock_version (03 §11).
UPDATE placement SET
    post_version_id = @post_version_id, status = @status, status_changed_at = @status_changed_at,
    status_reason = @status_reason, sent_to_blogger_at = @sent_to_blogger_at,
    accept_deadline_at = @accept_deadline_at, schedule_deadline_at = @schedule_deadline_at,
    publish_at = @publish_at, remove_at = @remove_at, published_at = @published_at, removed_at = @removed_at,
    claim_deadline_at = @claim_deadline_at, final_views = @final_views, lock_version = @lock_version,
    updated_at = @updated_at
WHERE id = @id AND status = @prev_status AND lock_version = @prev_lock_version;

-- name: ListPlacementsByOrder :many
SELECT * FROM placement WHERE order_id = @order_id ORDER BY created_at, id;

-- name: ListPlacementsByOrderForUpdate :many
SELECT * FROM placement WHERE order_id = @order_id ORDER BY id FOR UPDATE;

-- name: ListBloggerPlacements :many
-- R-16: только отправленные блогеру; новые сверху, ключевой курсор (created_at, id).
SELECT * FROM placement
WHERE blogger_organization_id = @organization_id
  AND sent_to_blogger_at IS NOT NULL
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('channel_id')::uuid IS NULL OR channel_id = sqlc.narg('channel_id')::uuid)
  AND (sqlc.narg('after_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('after_created_at')::timestamptz, sqlc.narg('after_id')::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @lim;

-- name: BusyIntervals :many
-- I-06: интервалы топа [начало, начало + top_hours) размещений канала в scheduled и published,
-- пересекающие [from, to). У опубликованного начало — published_at.
SELECT COALESCE(published_at, publish_at)::timestamptz AS starts_at, top_hours
FROM placement
WHERE channel_id = @channel_id AND status IN ('scheduled', 'published') AND id <> @exclude_id
  AND COALESCE(published_at, publish_at) < @to_at::timestamptz
  AND COALESCE(published_at, publish_at) + top_hours * interval '1 hour' > @from_at::timestamptz
ORDER BY starts_at;

-- name: ListPlacementsByChannel :many
SELECT * FROM placement WHERE channel_id = @channel_id AND status = ANY(@statuses::text[]) ORDER BY id;

-- name: ScheduledOverlapping :many
-- R-8 этапа «Каталог»: запланированные размещения, чей срок [publish_at, remove_at) пересекает [from, to).
SELECT id FROM placement
WHERE channel_id = @channel_id AND status = 'scheduled'
  AND publish_at < @to_at::timestamptz AND remove_at > @from_at::timestamptz
ORDER BY publish_at;

-- name: CountPlacementsByStatus :many
-- 07 §5.5: число размещений по состояниям.
SELECT status, count(*)::bigint AS n FROM placement GROUP BY status;

-- name: CountPlacementsByOrders :many
SELECT order_id, status, count(*)::bigint AS n FROM placement
WHERE order_id = ANY(@order_ids::uuid[]) GROUP BY order_id, status;

-- name: OverdueScheduled :many
-- 07 §3: размещение в scheduled с прошедшим publish_at.
SELECT id FROM placement WHERE status = 'scheduled' AND publish_at < @before ORDER BY publish_at LIMIT 1000;

-- name: ScheduledWithoutPublication :many
-- 05 §2.8 шаг 5: scheduled с прошедшим publish_at без публикации.
SELECT p.id, p.publish_at FROM placement p
WHERE p.status = 'scheduled' AND p.publish_at < @before
  AND NOT EXISTS (SELECT 1 FROM publication pub WHERE pub.placement_id = p.id)
ORDER BY p.publish_at LIMIT 1000;
