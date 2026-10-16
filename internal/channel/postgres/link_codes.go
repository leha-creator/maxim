package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/leha-creator/maxim/internal/channel/domain"
	"github.com/leha-creator/maxim/internal/channel/postgres/sqlc"
)

// LinkCodes — коды подключения канала (R-15). Хеш кода в журнал не пишется.
type LinkCodes struct{ s *Store }

func (r *LinkCodes) Create(ctx context.Context, c *domain.LinkCode) error {
	const op = "Channel.LinkCodes.Create"
	r.s.debug(ctx, op, "auth_code_id", c.ID, "organization_id", c.OrganizationID)
	err := r.s.q(ctx).InsertLinkCode(ctx, sqlc.InsertLinkCodeParams{
		ID: c.ID, OrganizationID: &c.OrganizationID, CodeHash: c.CodeHash, ExpiresAt: c.ExpiresAt, CreatedAt: c.CreatedAt,
	})
	if err != nil {
		return r.s.fail(ctx, op, err)
	}
	return nil
}

func (r *LinkCodes) ConsumeLive(ctx context.Context, orgID uuid.UUID, now time.Time) error {
	const op = "Channel.LinkCodes.ConsumeLive"
	r.s.debug(ctx, op, "organization_id", orgID)
	if err := r.s.q(ctx).ConsumeLiveLinkCodes(ctx, sqlc.ConsumeLiveLinkCodesParams{OrganizationID: &orgID, Now: now}); err != nil {
		return r.s.fail(ctx, op, err)
	}
	return nil
}

func (r *LinkCodes) FindLiveForUpdate(ctx context.Context, codeHash string, now time.Time) (*domain.LinkCode, error) {
	const op = "Channel.LinkCodes.FindLiveForUpdate"
	row, err := r.s.q(ctx).FindLiveLinkCodeForUpdate(ctx, sqlc.FindLiveLinkCodeForUpdateParams{CodeHash: codeHash, Now: now})
	if err != nil {
		r.s.debug(ctx, op, "found", false)
		return nil, r.s.fail(ctx, op, err)
	}
	r.s.debug(ctx, op, "found", true, "auth_code_id", row.ID)
	c := &domain.LinkCode{
		ID: row.ID, CodeHash: row.CodeHash, ExpiresAt: row.ExpiresAt, ConsumedAt: row.ConsumedAt,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	if row.OrganizationID != nil {
		c.OrganizationID = *row.OrganizationID
	}
	return c, nil
}

func (r *LinkCodes) Consume(ctx context.Context, c *domain.LinkCode) error {
	const op = "Channel.LinkCodes.Consume"
	r.s.debug(ctx, op, "auth_code_id", c.ID)
	if err := r.s.q(ctx).ConsumeLinkCode(ctx, sqlc.ConsumeLinkCodeParams{ID: c.ID, ConsumedAt: c.ConsumedAt, UpdatedAt: c.UpdatedAt}); err != nil {
		return r.s.fail(ctx, op, err)
	}
	return nil
}

func (r *LinkCodes) CountSince(ctx context.Context, orgID uuid.UUID, since time.Time) (int, error) {
	const op = "Channel.LinkCodes.CountSince"
	n, err := r.s.q(ctx).CountLinkCodesSince(ctx, sqlc.CountLinkCodesSinceParams{OrganizationID: &orgID, Since: since})
	if err != nil {
		return 0, r.s.fail(ctx, op, err)
	}
	r.s.debug(ctx, op, "organization_id", orgID, "count", n)
	return int(n), nil
}

// Categories — справочник категорий каналов.
type Categories struct{ s *Store }

func (r *Categories) ListActive(ctx context.Context) ([]domain.Category, error) {
	const op = "Channel.Categories.ListActive"
	rows, err := r.s.q(ctx).ListActiveCategories(ctx)
	if err != nil {
		return nil, r.s.fail(ctx, op, err)
	}
	r.s.debug(ctx, op, "count", len(rows))
	out := make([]domain.Category, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Category{ID: row.ID, Slug: row.Slug, Name: row.Name, Position: int(row.Position)})
	}
	return out, nil
}

func (r *Categories) ExistsActive(ctx context.Context, id uuid.UUID) (bool, error) {
	const op = "Channel.Categories.ExistsActive"
	ok, err := r.s.q(ctx).CategoryExistsActive(ctx, id)
	if err != nil {
		return false, r.s.fail(ctx, op, err)
	}
	r.s.debug(ctx, op, "category_id", id, "exists", ok)
	return ok, nil
}
