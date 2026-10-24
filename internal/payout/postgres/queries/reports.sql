-- Контроль депозита и зависшие (R-12, R-14). Таблицы ledger_account, placement и
-- organization принадлежат другим модулям: здесь только чтение.

-- name: RocketworkPayable :one
-- Начислено и не выплачено блогерам, которым платим через Рокет Ворк (не юрлицам).
SELECT coalesce(sum(a.balance), 0)::bigint AS total
FROM ledger_account a
JOIN organization o ON o.id = a.organization_id
WHERE a.kind = 'blogger_payable' AND o.legal_type <> 'legal_entity';

-- name: RocketworkInFlight :one
-- Цена размещений таких блогеров в published и removed: будущие начисления (05 §4.3).
SELECT coalesce(sum(p.blogger_amount), 0)::bigint AS total
FROM placement p
JOIN organization o ON o.id = p.blogger_organization_id
WHERE p.status IN ('published', 'removed') AND o.legal_type <> 'legal_entity';

-- name: AcceptedUnpaid :many
-- T-12: размещение в accepted без успешной выплаты дольше порога.
SELECT p.id, p.status_changed_at FROM placement p
WHERE p.status = 'accepted' AND p.status_changed_at < @before
ORDER BY p.status_changed_at LIMIT 1000;

-- name: PayoutsInStatusBefore :many
SELECT id, updated_at FROM payout
WHERE status = @status AND (@method::text = '' OR method = @method) AND updated_at < @before
ORDER BY updated_at LIMIT 1000;
