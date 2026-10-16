-- Заказы (04 §5, R-18, R-21). Кабинетные выборки ограничены организацией (07 §4).

-- name: NextOrderNumber :one
SELECT nextval('order_number_seq')::bigint AS number;

-- name: InsertOrder :exec
INSERT INTO "order" (
    id, number, organization_id, campaign_id, post_id, current_post_version_id, status, total_amount,
    idempotency_key, created_by, created_at, updated_at
) VALUES (
    @id, @number, @organization_id, @campaign_id, @post_id, @current_post_version_id, @status, @total_amount,
    @idempotency_key, @created_by, @created_at, @updated_at
);

-- name: GetOrder :one
SELECT * FROM "order" WHERE id = @id AND organization_id = @organization_id;

-- name: GetOrderForUpdate :one
SELECT * FROM "order" WHERE id = @id AND organization_id = @organization_id FOR UPDATE;

-- name: GetOrderByIDForUpdate :one
SELECT * FROM "order" WHERE id = @id FOR UPDATE;

-- name: ListOrders :many
-- Новые сверху, ключевой курсор (created_at, id), необязательный фильтр состояния.
SELECT * FROM "order"
WHERE organization_id = @organization_id
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('after_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('after_created_at')::timestamptz, sqlc.narg('after_id')::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @lim;

-- name: FindOrderByIdempotencyKey :one
SELECT * FROM "order" WHERE organization_id = @organization_id AND idempotency_key = @idempotency_key;

-- name: FindOrderByCurrentVersion :one
-- R-5: событие версии поста относится к заказу, у которого она текущая.
SELECT * FROM "order" WHERE current_post_version_id = @version_id;

-- name: GetOrderByNumber :one
-- Команда разработчика maxim dev post-version (R-23).
SELECT * FROM "order" WHERE number = @number;

-- name: GetOrderByID :one
SELECT * FROM "order" WHERE id = @id;

-- name: UpdateOrder :exec
UPDATE "order" SET status = @status, current_post_version_id = @current_post_version_id,
    post_version_idempotency_key = @post_version_idempotency_key, updated_at = @updated_at
WHERE id = @id;

-- name: FindOrderByPlacementVersion :one
-- Этап «Маркировка», R-12: заказ размещений версии поста (версия может быть уже не текущей).
SELECT o.* FROM "order" o
WHERE o.id = (SELECT p.order_id FROM placement p WHERE p.post_version_id = @version_id LIMIT 1);
