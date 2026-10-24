-- Споры (04 §5). У размещения не больше одного (I-11).

-- name: InsertDispute :exec
INSERT INTO dispute (
    id, placement_id, reason, opened_by_type, claim_id, violation_id, deadline_at, suggested_share, created_at, updated_at
) VALUES (
    @id, @placement_id, @reason, @opened_by_type, @claim_id, @violation_id, @deadline_at, @suggested_share, @created_at, @updated_at
);

-- name: GetDisputeByPlacement :one
SELECT * FROM dispute WHERE placement_id = @placement_id;

-- name: GetDisputeByPlacementForUpdate :one
SELECT * FROM dispute WHERE placement_id = @placement_id FOR UPDATE;

-- name: ListDisputesByPlacements :many
SELECT * FROM dispute WHERE placement_id = ANY(@placement_ids::uuid[]);

-- name: ResolveDispute :execrows
-- S-18: решение окончательное — записывается один раз.
UPDATE dispute SET resolution = @resolution, performance_share = @performance_share,
    resolution_comment = @resolution_comment, resolved_by = @resolved_by, resolved_at = @resolved_at,
    updated_at = @updated_at
WHERE id = @id AND resolution IS NULL;

-- name: ListDisputesAdmin :many
-- Очередь споров администратора по сроку T-06: открытые (или все), просроченные к overdue_at.
SELECT * FROM dispute
WHERE (sqlc.arg(all_disputes)::boolean OR resolution IS NULL)
  AND (sqlc.narg(overdue_at)::timestamptz IS NULL OR (resolution IS NULL AND deadline_at <= sqlc.narg(overdue_at)))
ORDER BY deadline_at, id
LIMIT @lim;
