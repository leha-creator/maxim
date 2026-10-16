-- Справочник форматов размещения (03 §2, 04 §3).

-- name: ListActiveFormats :many
SELECT id, name, top_hours, duration_hours, is_active, position FROM placement_format
WHERE is_active ORDER BY position, id;
