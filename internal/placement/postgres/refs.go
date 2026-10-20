package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// PlacementRef — заказ, канал и публикация размещения для других модулей (ledger, billing).
type PlacementRef struct {
	OrderID      uuid.UUID
	OrderNumber  int64
	ChannelTitle string
	PublishedAt  *time.Time
	RemovedAt    *time.Time
}

func utc(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

// Refs — сведения о размещениях по идентификаторам; без блокировки.
func (r *Placements) Refs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]PlacementRef, error) {
	const op = "Placement.Refs"
	out := make(map[uuid.UUID]PlacementRef, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := r.s.q(ctx).ListPlacementRefs(ctx, ids)
	if err != nil {
		return nil, r.s.fail(ctx, op, err)
	}
	for _, row := range rows {
		out[row.ID] = PlacementRef{OrderID: row.OrderID, OrderNumber: row.OrderNumber, ChannelTitle: row.ChannelTitle,
			PublishedAt: utc(row.PublishedAt), RemovedAt: utc(row.RemovedAt)}
	}
	return out, nil
}
