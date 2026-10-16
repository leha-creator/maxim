-- Кампании (04 §4, R-1). Выборки ограничены организацией (07 §4).

-- name: InsertCampaign :exec
INSERT INTO campaign (
    id, organization_id, name, status, date_from, date_to, time_from, time_to, extra_requirements,
    created_by, created_at, updated_at
) VALUES (
    @id, @organization_id, @name, @status, @date_from, @date_to, @time_from, @time_to, @extra_requirements,
    @created_by, @created_at, @updated_at
);

-- name: GetCampaign :one
SELECT * FROM campaign WHERE id = @id AND organization_id = @organization_id;

-- name: GetCampaignForUpdate :one
SELECT * FROM campaign WHERE id = @id AND organization_id = @organization_id FOR UPDATE;

-- name: ListCampaigns :many
-- R-17: новые сверху, ключевой курсор (created_at, id), необязательный фильтр состояния.
SELECT * FROM campaign
WHERE organization_id = @organization_id
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status')::text)
  AND (sqlc.narg('after_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('after_created_at')::timestamptz, sqlc.narg('after_id')::uuid))
ORDER BY created_at DESC, id DESC
LIMIT @lim;

-- name: UpdateCampaign :exec
UPDATE campaign SET
    name = @name, status = @status, date_from = @date_from, date_to = @date_to, time_from = @time_from,
    time_to = @time_to, extra_requirements = @extra_requirements, updated_at = @updated_at
WHERE id = @id AND organization_id = @organization_id;
