-- Претензии рекламодателя (04 §5, S-08). Только добавление; одна на размещение.

-- name: InsertClaim :exec
INSERT INTO claim (id, placement_id, reason, text, created_by, created_at)
VALUES (@id, @placement_id, @reason, @text, @created_by, @created_at);

-- name: GetClaimByPlacement :one
SELECT * FROM claim WHERE placement_id = @placement_id;

-- name: ListClaimsByPlacements :many
SELECT * FROM claim WHERE placement_id = ANY(@placement_ids::uuid[]);
