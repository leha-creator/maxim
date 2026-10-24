-- Выборки разделов администратора (этап «API администратора»): очередь модерации поста,
-- размещения и споры с фильтрами и общим числом строк (total). Курсор — смещение (adminpage).

-- name: ListAdminOrders :many
-- moderation — по началу периода публикации, затем по времени ожидания; marking и all — по ожиданию.
WITH q AS (
    SELECT o.id, o.number, o.organization_id, o.status, o.total_amount, o.created_at,
        c.name AS campaign_name,
        (SELECT min(p.window_start) FROM placement p WHERE p.order_id = o.id)::timestamptz AS publish_from,
        (SELECT count(*) FROM placement p WHERE p.order_id = o.id)::int AS placements_count,
        (CASE WHEN o.status = 'moderation' THEN pv.created_at ELSE o.updated_at END)::timestamptz AS waiting_since
    FROM "order" o
    JOIN campaign c ON c.id = o.campaign_id
    JOIN organization org ON org.id = o.organization_id
    JOIN post_version pv ON pv.id = o.current_post_version_id
    WHERE (sqlc.narg(status)::text IS NULL OR o.status = sqlc.narg(status)::text)
      AND (sqlc.narg(query)::text IS NULL OR o.number::text = sqlc.narg(query)::text OR org.inn = sqlc.narg(query)::text)
)
SELECT q.*, count(*) OVER ()::int AS total
FROM q
ORDER BY CASE WHEN sqlc.arg(by_deadline)::boolean THEN q.publish_from END, q.waiting_since, q.id
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: ListAdminPlacements :many
-- Размещения по фильтрам, новые сверху; removal_overdue — пост жив после remove_at или снятие
-- по сроку не удалось (T-10).
SELECT sqlc.embed(p), o.number AS order_number, o.organization_id AS advertiser_id,
    (pub.id IS NOT NULL AND (pub.removal_overdue OR (pub.removed_at IS NULL AND p.remove_at < sqlc.arg(now)::timestamptz)))::boolean AS removal_overdue,
    count(*) OVER ()::int AS total
FROM placement p
JOIN "order" o ON o.id = p.order_id
LEFT JOIN publication pub ON pub.placement_id = p.id
WHERE (sqlc.narg(status)::text IS NULL OR p.status = sqlc.narg(status)::text)
  AND (sqlc.narg(channel_id)::uuid IS NULL OR p.channel_id = sqlc.narg(channel_id)::uuid)
  AND (sqlc.narg(order_id)::uuid IS NULL OR p.order_id = sqlc.narg(order_id)::uuid)
  AND (sqlc.narg(created_from)::timestamptz IS NULL OR p.created_at >= sqlc.narg(created_from)::timestamptz)
  AND (sqlc.narg(created_to)::timestamptz IS NULL OR p.created_at < sqlc.narg(created_to)::timestamptz)
  AND (NOT sqlc.arg(only_removal_overdue)::boolean OR (pub.id IS NOT NULL
       AND (pub.removal_overdue OR (pub.removed_at IS NULL AND p.remove_at < sqlc.arg(now)::timestamptz))))
ORDER BY p.created_at DESC, p.id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: ListAdminDisputes :many
-- open — ближайший срок первым; resolved и all — новые первыми.
SELECT sqlc.embed(d), p.status AS placement_status, p.total_amount, p.channel_id, p.blogger_organization_id,
    o.number AS order_number, o.organization_id AS advertiser_id, count(*) OVER ()::int AS total
FROM dispute d
JOIN placement p ON p.id = d.placement_id
JOIN "order" o ON o.id = p.order_id
WHERE (sqlc.arg(state)::text = 'all'
       OR (sqlc.arg(state)::text = 'open' AND d.resolution IS NULL)
       OR (sqlc.arg(state)::text = 'resolved' AND d.resolution IS NOT NULL))
  AND (sqlc.narg(overdue_at)::timestamptz IS NULL OR (d.resolution IS NULL AND d.deadline_at <= sqlc.narg(overdue_at)::timestamptz))
ORDER BY CASE WHEN sqlc.arg(state)::text = 'open' THEN d.deadline_at END,
    d.created_at DESC, d.id
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: GetDisputeByID :one
SELECT * FROM dispute WHERE id = @id;

-- name: CountAdminQueues :one
-- Счётчики разделов администратора (R-16): заказы на модерации, открытые и просроченные споры,
-- просроченные снятия.
SELECT
    (SELECT count(*) FROM "order" WHERE status = 'moderation')::int AS orders_moderation,
    (SELECT count(*) FROM dispute WHERE resolution IS NULL)::int AS disputes_open,
    (SELECT count(*) FROM dispute WHERE resolution IS NULL AND deadline_at <= sqlc.arg(now)::timestamptz)::int AS disputes_overdue,
    (SELECT count(*) FROM publication pub JOIN placement p ON p.id = pub.placement_id
     WHERE pub.removal_overdue OR (pub.removed_at IS NULL AND p.remove_at < sqlc.arg(now)::timestamptz))::int AS removals_overdue;
