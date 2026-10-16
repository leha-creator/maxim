-- Исходящие запросы и предохранитель (05 §1, R-3).

-- name: InsertOutboundRequest :exec
INSERT INTO outbound_request (id, system, operation, idempotency_key, attempt, request, response_status,
    response_body, duration_ms, error, object_type, object_id, created_at)
VALUES (@id, @system, @operation, @idempotency_key, @attempt, @request, @response_status,
    @response_body, @duration_ms, @error, @object_type, @object_id, @created_at);

-- name: GetCircuit :one
SELECT consecutive_failures, open_until FROM outbound_circuit WHERE system = @system;

-- name: RecordCircuitFailure :one
-- Ошибка подряд; на пороге предохранитель открывается до open_until.
INSERT INTO outbound_circuit (system, consecutive_failures, open_until, updated_at)
VALUES (@system, 1, CASE WHEN 1 >= @threshold::integer THEN @open_until::timestamptz END, @now::timestamptz)
ON CONFLICT (system) DO UPDATE
SET consecutive_failures = outbound_circuit.consecutive_failures + 1,
    open_until = CASE WHEN outbound_circuit.consecutive_failures + 1 >= @threshold::integer
                      THEN @open_until::timestamptz ELSE outbound_circuit.open_until END,
    updated_at = @now::timestamptz
RETURNING consecutive_failures, open_until;

-- name: ResetCircuit :exec
UPDATE outbound_circuit
SET consecutive_failures = 0, open_until = NULL, updated_at = @now::timestamptz
WHERE system = @system AND (consecutive_failures <> 0 OR open_until IS NOT NULL);

-- name: DeleteOutboundRequestsBefore :execrows
-- Очистка (04 §8: хранится 180 суток), пачками.
DELETE FROM outbound_request
WHERE id IN (SELECT o.id FROM outbound_request o WHERE o.created_at < @before::timestamptz LIMIT @batch);
