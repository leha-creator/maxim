-- Каналы для модуля размещений (этап «Заказ и размещения»): корзина и оформление.

-- name: ListChannelsByIDs :many
SELECT * FROM channel WHERE id = ANY(@ids::uuid[]) ORDER BY id;
