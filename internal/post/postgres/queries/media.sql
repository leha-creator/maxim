-- Оригиналы медиатеки (04 §4). Кабинетные выборки ограничены организацией (07 §4).

-- name: InsertMediaFile :exec
INSERT INTO media_file (
    id, organization_id, storage_key, mime, size_bytes, width, height, sha256, original_name, created_by, created_at
) VALUES (
    @id, @organization_id, @storage_key, @mime, @size_bytes, @width, @height, @sha256, @original_name, @created_by, @created_at
);

-- name: GetMediaFile :one
SELECT * FROM media_file WHERE id = @id AND organization_id = @organization_id AND deleted_at IS NULL;

-- name: GetMediaFileAny :one
SELECT * FROM media_file WHERE id = @id;

-- name: FindMediaFileBySHA256 :one
SELECT * FROM media_file WHERE organization_id = @organization_id AND sha256 = @sha256 AND deleted_at IS NULL;

-- name: ListMediaFiles :many
-- R-21: новые сверху, ключевой курсор (created_at, id).
SELECT * FROM media_file
WHERE organization_id = @organization_id AND deleted_at IS NULL
  AND (sqlc.narg('after_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('after_created_at')::timestamptz, sqlc.narg('after_id')::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @lim;

-- name: ListLiveMediaFiles :many
SELECT * FROM media_file
WHERE organization_id = @organization_id AND id = ANY(@ids::uuid[]) AND deleted_at IS NULL;

-- name: SoftDeleteMediaFile :execrows
UPDATE media_file SET deleted_at = @now
WHERE id = @id AND organization_id = @organization_id AND deleted_at IS NULL;

-- name: ListMediaFilesForPurge :many
-- R-19: удалённые давно и не входящие ни в одну версию поста.
SELECT * FROM media_file f
WHERE f.deleted_at < @before::timestamptz AND f.purged_at IS NULL
  AND NOT EXISTS (SELECT 1 FROM post_version_media m WHERE m.media_file_id = f.id)
ORDER BY f.deleted_at, f.id
LIMIT @lim;

-- name: MarkMediaFilePurged :exec
UPDATE media_file SET purged_at = @now WHERE id = @id AND purged_at IS NULL;
