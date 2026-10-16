-- Производные изображений (04 §4, R-6, R-7, R-19).

-- name: EnsureDerivative :one
-- Идемпотентно по тройке (файл, вид, хеш). Очищенная запись (purge_media) снова ждёт построения.
INSERT INTO media_derivative (id, media_file_id, kind, params_hash, status, created_at)
VALUES (@id, @media_file_id, @kind, @params_hash, 'pending', @now)
ON CONFLICT (media_file_id, kind, params_hash) DO UPDATE SET purged_at = NULL
RETURNING *;

-- name: GetDerivative :one
SELECT * FROM media_derivative WHERE id = @id;

-- name: FindDerivative :one
SELECT * FROM media_derivative WHERE media_file_id = @media_file_id AND kind = @kind AND params_hash = @params_hash;

-- name: ListDerivativesByFiles :many
SELECT * FROM media_derivative WHERE media_file_id = ANY(@file_ids::uuid[]) AND kind = @kind;

-- name: MarkDerivativeReady :execrows
UPDATE media_derivative SET
    status = 'ready', storage_key = @storage_key, mime = @mime, width = @width, height = @height, size_bytes = @size_bytes
WHERE id = @id AND status = 'pending';

-- name: MarkDerivativePublished :execrows
UPDATE media_derivative SET public_storage_key = @public_storage_key, public_url = @public_url, published_at = @now
WHERE id = @id AND status = 'ready' AND published_at IS NULL;

-- name: ListDerivativesForPurge :many
-- R-19: построенные давно и не входящие ни в одну версию. Превью живого файла не удаляется:
-- оно нужно медиатеке.
SELECT d.* FROM media_derivative d
JOIN media_file f ON f.id = d.media_file_id
WHERE d.status = 'ready' AND d.purged_at IS NULL AND d.created_at < @before::timestamptz
  AND (d.kind <> 'preview' OR f.deleted_at IS NOT NULL)
  AND NOT EXISTS (SELECT 1 FROM post_version_media m WHERE m.media_derivative_id = d.id)
ORDER BY d.created_at, d.id
LIMIT @lim;

-- name: MarkDerivativePurged :exec
-- Объект удалён из хранилища: запись возвращается в pending, чтобы её можно было построить снова.
UPDATE media_derivative SET
    status = 'pending', storage_key = NULL, mime = NULL, width = NULL, height = NULL, size_bytes = NULL, purged_at = @now
WHERE id = @id AND published_at IS NULL;
