package handlers

import (
	"context"
	"slices"
	"time"

	"maxim/mockapi/internal/api"
	"maxim/mockapi/internal/store"
)

// Операции администратора. Матрица ролей — как у бэкенда (R-1): admin — moderator или senior,
// senior — только senior; иначе 403 с details.reason = admin_required.

// admin требует роль администратора.
func admin(ctx context.Context, tx *store.Tx) (*store.User, error) {
	return adminWith(ctx, tx, false)
}

// senior требует роль senior.
func senior(ctx context.Context, tx *store.Tx) (*store.User, error) {
	return adminWith(ctx, tx, true)
}

func adminWith(ctx context.Context, tx *store.Tx, seniorOnly bool) (*store.User, error) {
	u, err := user(ctx, tx)
	if err != nil {
		return nil, err
	}
	if u.AdminRole == "" || (seniorOnly && u.AdminRole != "senior") {
		tx.Log().Warn("admin access denied", "op", "Admin.Access", "user_id", u.ID, "senior_only", seniorOnly)
		e := store.Forbidden("admin role is required")
		e.Details = map[string]any{"reason": "admin_required"}
		return nil, e
	}
	return u, nil
}

// maskINN — видны первые 2 и последние 4 цифры (07 §5.4).
func maskINN(inn string) string {
	if len(inn) <= 6 {
		return inn
	}
	b := []byte(inn)
	for i := 2; i < len(b)-4; i++ {
		b[i] = '*'
	}
	return string(b)
}

func adminOrgRef(tx *store.Tx, c *store.Channel) *api.AdminOrganizationRef {
	if c.OrgID == nil {
		return nil
	}
	o, err := tx.Org(*c.OrgID)
	if err != nil {
		return nil
	}
	return &api.AdminOrganizationRef{Id: o.ID, Name: o.Name, Inn: maskINN(o.INN)}
}

// channelUpdatedAt — время последнего изменения; до первого изменения — создание.
func channelUpdatedAt(c *store.Channel) time.Time {
	if c.UpdatedAt.IsZero() {
		return c.CreatedAt
	}
	return c.UpdatedAt
}

func adminChannel(tx *store.Tx, c *store.Channel) api.AdminChannel {
	b := bloggerChannel(tx, c)
	out := api.AdminChannel{
		Id: b.Id, Title: b.Title, IconUrl: b.IconUrl, Link: b.Link, IsPublic: b.IsPublic, Status: b.Status,
		StatusReason: b.StatusReason, Description: b.Description, CategoryId: b.CategoryId,
		SubscribersCount: b.SubscribersCount, BotIsAdmin: b.BotIsAdmin, BotPermissions: b.BotPermissions,
		MissingPermissions: b.MissingPermissions, RknNumber: b.RknNumber, RknStatus: b.RknStatus, Rating: b.Rating,
		Prices: b.Prices, Unavailability: b.Unavailability,
		Organization: adminOrgRef(tx, c), MaxDescription: strPtr(c.MaxDescription), AvgViews: ptr(c.AvgViews),
		PostsPerWeek: ptr(c.PostsPerWeek), MetricsHistory: metricsHistory(tx, c), RknCheckedAt: c.RknCheckedAt,
		RknRecheckDue: tx.RknRecheckDue(c), ModeratedAt: c.ModeratedAt, ModerationHistory: []api.ChannelModerationEntry{},
	}
	if c.Subscribers > 0 {
		out.EngagementBp = ptr(c.AvgViews * 10000 / c.Subscribers)
	}
	if c.AnomalyFlags != nil {
		flags := []api.AdminChannelAnomalyFlags{}
		for _, f := range c.AnomalyFlags {
			flags = append(flags, api.AdminChannelAnomalyFlags(f))
		}
		out.AnomalyFlags = &flags
	}
	for _, m := range c.Moderation {
		e := api.ChannelModerationEntry{Action: api.ChannelModerationEntryAction(m.Action), ActorType: "admin",
			ActorUserId: ptr(m.ActorUserID), Comment: strPtr(m.Comment), CreatedAt: m.CreatedAt}
		if m.Result != "" {
			e.Result = ptr(api.ChannelModerationEntryResult(m.Result))
		}
		out.ModerationHistory = append(out.ModerationHistory, e)
	}
	return out
}

func (h *Handlers) ListAdminChannels(ctx context.Context, req api.ListAdminChannelsRequestObject) (api.ListAdminChannelsResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	if _, err := admin(ctx, tx); err != nil {
		return nil, err
	}
	p := req.Params
	var found []*store.Channel
	for _, c := range tx.D.Channels {
		switch {
		case c.Status == "unclaimed",
			p.Status != nil && string(*p.Status) != c.Status,
			p.RknStatus != nil && string(*p.RknStatus) != c.RknStatus,
			p.RknRecheckDue != nil && *p.RknRecheckDue != tx.RknRecheckDue(c):
			continue
		}
		found = append(found, c)
	}
	// Старые изменения первыми: очередь модерации разбирается по порядку.
	found = slices.Clone(found)
	slices.SortStableFunc(found, func(a, b *store.Channel) int { return channelUpdatedAt(a).Compare(channelUpdatedAt(b)) })
	pg, next, total := page(found, p.Limit, p.Cursor)
	items := []api.AdminChannelListItem{}
	for _, c := range pg {
		items = append(items, api.AdminChannelListItem{
			Id: c.ID, Title: c.Title, Link: strPtr(c.Link), Organization: adminOrgRef(tx, c),
			Status: api.ChannelStatus(c.Status), StatusReason: strPtr(c.StatusReason), CategoryId: c.CategoryID,
			SubscribersCount: c.Subscribers, RknNumber: strPtr(c.RknNumber), RknStatus: api.RknStatus(c.RknStatus),
			RknRecheckDue: tx.RknRecheckDue(c), UpdatedAt: channelUpdatedAt(c),
		})
	}
	h.logger(ctx).Debug("[FIX] admin channels listed", "op", "Admin.ListChannels", "count", len(items), "total", *total)
	return api.ListAdminChannels200JSONResponse{Items: items, NextCursor: next, Total: total}, nil
}

// adminTarget — канал для операции администратора; unclaimed администратору не виден.
func adminTarget(tx *store.Tx, id api.Uuid) (*store.Channel, error) {
	c, err := tx.Channel(id)
	if err != nil {
		return nil, err
	}
	if c.Status == "unclaimed" {
		return nil, store.NotFound("channel", id)
	}
	return c, nil
}

func (h *Handlers) GetAdminChannel(ctx context.Context, req api.GetAdminChannelRequestObject) (api.GetAdminChannelResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	if _, err := admin(ctx, tx); err != nil {
		return nil, err
	}
	c, err := adminTarget(tx, req.Id)
	if err != nil {
		return nil, err
	}
	return api.GetAdminChannel200JSONResponse(adminChannel(tx, c)), nil
}

func (h *Handlers) ApproveChannel(ctx context.Context, req api.ApproveChannelRequestObject) (api.ApproveChannelResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := admin(ctx, tx)
	if err != nil {
		return nil, err
	}
	c, err := adminTarget(tx, req.Id)
	if err != nil {
		return nil, err
	}
	if err := tx.ApproveChannel(u.ID, c); err != nil {
		return nil, err
	}
	return api.ApproveChannel200JSONResponse(adminChannel(tx, c)), nil
}

func (h *Handlers) RejectChannel(ctx context.Context, req api.RejectChannelRequestObject) (api.RejectChannelResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := admin(ctx, tx)
	if err != nil {
		return nil, err
	}
	c, err := adminTarget(tx, req.Id)
	if err != nil {
		return nil, err
	}
	if err := tx.RejectChannel(u.ID, c, req.Body.Reason); err != nil {
		return nil, err
	}
	return api.RejectChannel200JSONResponse(adminChannel(tx, c)), nil
}

func (h *Handlers) SetChannelRknCheck(ctx context.Context, req api.SetChannelRknCheckRequestObject) (api.SetChannelRknCheckResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := admin(ctx, tx)
	if err != nil {
		return nil, err
	}
	c, err := adminTarget(tx, req.Id)
	if err != nil {
		return nil, err
	}
	comment := ""
	if req.Body.Comment != nil {
		comment = *req.Body.Comment
	}
	if err := tx.RknCheckChannel(u.ID, c, string(req.Body.Result), comment); err != nil {
		return nil, err
	}
	return api.SetChannelRknCheck200JSONResponse(adminChannel(tx, c)), nil
}
