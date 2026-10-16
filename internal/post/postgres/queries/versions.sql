-- Версии поста (04 §4, R-18). Версия в ready неизменяема — это закрепляет триггер базы.

-- name: NextVersionNumber :one
-- Вызывается под блокировкой поста (GetPostByCampaignForUpdate): номера не повторяются.
SELECT (COALESCE(MAX(number), 0) + 1)::integer AS next FROM post_version WHERE post_id = @post_id;

-- name: InsertVersion :exec
INSERT INTO post_version (
    id, post_id, number, status, content_doc, body, token_carrier, token_position,
    advertiser_name, advertiser_inn, lock_version, created_by, created_at
) VALUES (
    @id, @post_id, @number, @status, @content_doc, @body, @token_carrier, @token_position,
    @advertiser_name, @advertiser_inn, @lock_version, @created_by, @created_at
);

-- name: InsertVersionMedia :exec
INSERT INTO post_version_media (id, post_version_id, position, media_file_id, media_derivative_id, created_at)
VALUES (@id, @post_version_id, @position, @media_file_id, @media_derivative_id, @created_at);

-- name: GetVersion :one
SELECT * FROM post_version WHERE id = @id;

-- name: GetVersionForUpdate :one
SELECT * FROM post_version WHERE id = @id FOR UPDATE;

-- name: ListVersionsByPost :many
SELECT * FROM post_version WHERE post_id = @post_id ORDER BY number;

-- name: ListVersionMedia :many
SELECT * FROM post_version_media WHERE post_version_id = ANY(@version_ids::uuid[]) ORDER BY post_version_id, position;

-- name: UpdateVersionStatus :execrows
-- Оптимистическая блокировка (04 §1).
UPDATE post_version SET
    status = @status, final_text = @final_text, erid = @erid, moderated_by = @moderated_by,
    moderated_at = @moderated_at, rejection_reason = @rejection_reason, lock_version = @lock_version
WHERE id = @id AND lock_version = @prev_lock_version;
