-- Нарушения (04 §5). Только добавление (R-20).

-- name: InsertViolation :execrows
-- R-10: повтор того же события второй записи не создаёт.
INSERT INTO violation (id, placement_id, kind, occurred_at, source, inbound_event_id, details, created_at)
VALUES (@id, @placement_id, @kind, @occurred_at, @source, @inbound_event_id, @details, @created_at)
ON CONFLICT (placement_id, inbound_event_id) WHERE inbound_event_id IS NOT NULL DO NOTHING;
