-- Удержания по размещениям (04 §7, I-03): не больше одного активного на размещение.

-- name: GetActiveHoldForUpdate :one
SELECT * FROM hold WHERE placement_id = @placement_id AND status = 'active' FOR UPDATE;

-- name: ListActiveHolds :many
SELECT * FROM hold WHERE placement_id = ANY(@placement_ids::uuid[]) AND status = 'active';

-- name: InsertHold :exec
INSERT INTO hold (id, placement_id, post_version_id, organization_id, amount, status, closed_at, created_at, updated_at)
VALUES (@id, @placement_id, @post_version_id, @organization_id, @amount, 'active', NULL, @created_at, @created_at);

-- name: CloseHold :execrows
UPDATE hold SET status = @status, closed_at = @closed_at, updated_at = @closed_at
WHERE id = @id AND status = 'active';

-- name: SumActiveHolds :one
SELECT coalesce(sum(amount), 0)::bigint AS held FROM hold
WHERE organization_id = @organization_id AND status = 'active';
