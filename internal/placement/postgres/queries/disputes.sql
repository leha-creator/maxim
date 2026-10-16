-- Споры (04 §5). У размещения не больше одного (I-11).

-- name: InsertDispute :exec
INSERT INTO dispute (
    id, placement_id, reason, opened_by_type, claim_id, violation_id, deadline_at, suggested_share, created_at, updated_at
) VALUES (
    @id, @placement_id, @reason, @opened_by_type, @claim_id, @violation_id, @deadline_at, @suggested_share, @created_at, @updated_at
);

-- name: GetDisputeByPlacement :one
SELECT * FROM dispute WHERE placement_id = @placement_id;

-- name: ListDisputesByPlacements :many
SELECT * FROM dispute WHERE placement_id = ANY(@placement_ids::uuid[]);
