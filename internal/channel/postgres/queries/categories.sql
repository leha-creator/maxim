-- Справочник категорий каналов (04 §3, A-14).

-- name: ListActiveCategories :many
SELECT id, slug, name, position FROM channel_category WHERE is_active ORDER BY position, slug;

-- name: CategoryExistsActive :one
SELECT EXISTS (SELECT 1 FROM channel_category WHERE id = @id AND is_active);
