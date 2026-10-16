-- История состояний размещения (03 §12). Только добавление: права роли (04 §1).

-- name: InsertHistory :exec
INSERT INTO placement_state_history (
    id, placement_id, transition, from_status, to_status, actor_type, actor_user_id, reason_code, reason_text,
    job_id, idempotency_key, created_at
) VALUES (
    @id, @placement_id, @transition, @from_status, @to_status, @actor_type, @actor_user_id, @reason_code, @reason_text,
    @job_id, @idempotency_key, @created_at
);

-- name: ListHistoryByPlacements :many
SELECT * FROM placement_state_history WHERE placement_id = ANY(@placement_ids::uuid[]) ORDER BY created_at, id;

-- name: FindHistoryByKey :one
-- R-18: переход, уже выполненный с этим ключом.
SELECT * FROM placement_state_history WHERE placement_id = @placement_id AND idempotency_key = @idempotency_key;

-- name: ExistsHistoryKeyInOrder :one
SELECT EXISTS (
    SELECT 1 FROM placement_state_history h JOIN placement p ON p.id = h.placement_id
    WHERE p.order_id = @order_id AND h.idempotency_key = @idempotency_key
) AS found;
