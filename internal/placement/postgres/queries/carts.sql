-- Корзина (04 §5, R-7, R-8). Выборки ограничены организацией (07 §4).

-- name: ListCartItems :many
SELECT * FROM cart_item WHERE organization_id = @organization_id ORDER BY created_at, id;

-- name: GetCartItem :one
SELECT * FROM cart_item WHERE id = @id AND organization_id = @organization_id;

-- name: InsertCartItem :exec
INSERT INTO cart_item (id, organization_id, channel_id, format_id, shown_total_amount, created_by, created_at, updated_at)
VALUES (@id, @organization_id, @channel_id, @format_id, @shown_total_amount, @created_by, @created_at, @updated_at);

-- name: UpdateCartItem :execrows
UPDATE cart_item SET format_id = @format_id, shown_total_amount = @shown_total_amount, updated_at = @updated_at
WHERE id = @id AND organization_id = @organization_id;

-- name: DeleteCartItem :execrows
DELETE FROM cart_item WHERE id = @id AND organization_id = @organization_id;

-- name: ClearCart :exec
DELETE FROM cart_item WHERE organization_id = @organization_id;

-- name: CountCartItems :one
SELECT count(*)::integer AS n FROM cart_item WHERE organization_id = @organization_id;

-- name: ChannelsInCart :many
-- Признак in_cart каталога (R-8).
SELECT channel_id FROM cart_item WHERE organization_id = @organization_id AND channel_id = ANY(@channel_ids::uuid[]);
