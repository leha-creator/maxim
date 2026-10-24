package handlers

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"maxim/mockapi/internal/api"
	"maxim/mockapi/internal/store"
)

// Модерация поста (S-17), размещения и ручные операции (S-19), споры и переписка (S-18).

// ---------------------------------------------------------------- заказы

func orderPublishFrom(tx *store.Tx, o *store.Order) *time.Time {
	var first *time.Time
	for _, p := range tx.OrderPlacements(o.ID) {
		if first == nil || p.WindowStart.Before(*first) {
			ws := p.WindowStart
			first = &ws
		}
	}
	return first
}

func adminOrderItem(tx *store.Tx, o *store.Order) api.AdminOrderListItem {
	s := orderSummary(tx, o)
	from := orderPublishFrom(tx, o)
	return api.AdminOrderListItem{
		Id: o.ID, Number: o.Number, Status: s.Status, Advertiser: orgRef(tx, o.OrgID), CampaignName: s.CampaignName,
		PlacementsCount: s.PlacementsCount, TotalAmount: s.TotalAmount, PublishFrom: from,
		Urgent:       from != nil && from.Before(tx.Now.Add(6*time.Hour)),
		WaitingSince: o.CurrentVersion().CreatedAt, CreatedAt: o.CreatedAt,
	}
}

func adminOrder(tx *store.Tx, o *store.Order) api.AdminOrder {
	out := recast[api.AdminOrder](adminOrderItem(tx, o))
	v := o.CurrentVersion()
	out.CampaignId = o.CampaignID
	out.CurrentPostVersion = postVersion(tx, o.OrgID, v)
	links := store.BodyLinks(v.Body)
	out.Links = &links
	out.Placements = []api.AdminPlacementListItem{}
	for _, p := range tx.OrderPlacements(o.ID) {
		out.Placements = append(out.Placements, adminPlacementItem(tx, p))
	}
	if v.Erid != "" {
		out.Creative = &api.OrdObject{Type: api.OrdObjectTypeCreative, Id: v.ID, Title: "Заказ № " + itoa(o.Number),
			Status: api.OrdSyncStatusSynced, Erid: strPtr(v.Erid), UpdatedAt: v.CreatedAt}
	}
	return out
}

func (h *Handlers) ListAdminOrders(ctx context.Context, req api.ListAdminOrdersRequestObject) (api.ListAdminOrdersResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	if _, err := admin(ctx, tx); err != nil {
		return nil, err
	}
	p := req.Params
	status := "moderation"
	if p.Status != nil {
		status = string(*p.Status)
	}
	var found []*store.Order
	for _, o := range tx.D.Orders {
		if status != "all" && o.Status != status {
			continue
		}
		if p.Query != nil && *p.Query != "" {
			adv, _ := tx.Org(o.OrgID)
			if itoa(o.Number) != *p.Query && (adv == nil || adv.INN != *p.Query) {
				continue
			}
		}
		found = append(found, o)
	}
	found = slices.Clone(found)
	slices.SortStableFunc(found, func(a, b *store.Order) int {
		if status == "moderation" {
			fa, fb := orderPublishFrom(tx, a), orderPublishFrom(tx, b)
			if fa != nil && fb != nil && !fa.Equal(*fb) {
				return fa.Compare(*fb)
			}
		}
		return a.CurrentVersion().CreatedAt.Compare(b.CurrentVersion().CreatedAt)
	})
	pg, next, total := page(found, p.Limit, p.Cursor)
	items := []api.AdminOrderListItem{}
	for _, o := range pg {
		items = append(items, adminOrderItem(tx, o))
	}
	return api.ListAdminOrders200JSONResponse{Items: items, NextCursor: next, Total: total}, nil
}

func (h *Handlers) GetAdminOrder(ctx context.Context, req api.GetAdminOrderRequestObject) (api.GetAdminOrderResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	if _, err := admin(ctx, tx); err != nil {
		return nil, err
	}
	o, err := tx.Order(req.Id)
	if err != nil {
		return nil, err
	}
	return api.GetAdminOrder200JSONResponse(adminOrder(tx, o)), nil
}

func moderationTarget(tx *store.Tx, id uuid.UUID) (*store.Order, error) {
	o, err := tx.Order(id)
	if err != nil {
		return nil, err
	}
	if o.Status != "moderation" {
		return nil, store.Conflict("invalid_transition", "order is not in moderation", map[string]any{"status": o.Status})
	}
	return o, nil
}

func (h *Handlers) ApproveOrderPost(ctx context.Context, req api.ApproveOrderPostRequestObject) (api.ApproveOrderPostResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := admin(ctx, tx)
	if err != nil {
		return nil, err
	}
	o, err := moderationTarget(tx, req.Id)
	if err != nil {
		return nil, err
	}
	if err := tx.ApprovePost(o); err != nil {
		return nil, err
	}
	tx.Audit(u.ID, &o.OrgID, "post_version.approved", "post_version", o.CurrentVersion().ID.String(), "", nil)
	h.logger(ctx).Warn("admin order post approved", "op", "Admin.ApproveOrderPost", "order_id", o.ID, "admin_user_id", u.ID)
	return api.ApproveOrderPost200JSONResponse(adminOrder(tx, o)), nil
}

func (h *Handlers) RejectOrderPost(ctx context.Context, req api.RejectOrderPostRequestObject) (api.RejectOrderPostResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := admin(ctx, tx)
	if err != nil {
		return nil, err
	}
	o, err := moderationTarget(tx, req.Id)
	if err != nil {
		return nil, err
	}
	if err := tx.RejectPost(o, req.Body.Reason); err != nil {
		return nil, err
	}
	tx.Audit(u.ID, &o.OrgID, "post_version.rejected", "post_version", o.CurrentVersion().ID.String(), req.Body.Reason, nil)
	h.logger(ctx).Warn("admin order post rejected", "op", "Admin.RejectOrderPost", "order_id", o.ID, "admin_user_id", u.ID)
	return api.RejectOrderPost200JSONResponse(adminOrder(tx, o)), nil
}

// ---------------------------------------------------------------- размещения

func removalOverdue(tx *store.Tx, p *store.Placement) bool {
	return p.Status == "published" && p.RemovedAt == nil && p.RemoveAt != nil && tx.Now.After(*p.RemoveAt)
}

func adminPlacementItem(tx *store.Tx, p *store.Placement) api.AdminPlacementListItem {
	t := timing(p)
	out := api.AdminPlacementListItem{
		Id: p.ID, OrderId: p.OrderID, Blogger: orgRef(tx, p.BloggerOrgID), Status: api.PlacementStatus(p.Status),
		StatusReason: strPtr(p.StatusReason), StatusChangedAt: p.StatusChangedAt, TotalAmount: p.TotalAmount,
		RefundAmount: p.RefundAmount, RemovalOverdue: ptr(removalOverdue(tx, p)), CreatedAt: p.CreatedAt,
		FormatId: t.FormatId, TopHours: t.TopHours, DurationHours: t.DurationHours, WindowStart: t.WindowStart,
		WindowEnd: t.WindowEnd, TimeFrom: t.TimeFrom, TimeTo: t.TimeTo, AcceptDeadlineAt: t.AcceptDeadlineAt,
		ScheduleDeadlineAt: t.ScheduleDeadlineAt, PublishAt: t.PublishAt, RemoveAt: t.RemoveAt,
		PublishedAt: t.PublishedAt, RemovedAt: t.RemovedAt, ClaimDeadlineAt: t.ClaimDeadlineAt,
	}
	if o, err := tx.Order(p.OrderID); err == nil {
		out.OrderNumber, out.Advertiser = o.Number, orgRef(tx, o.OrgID)
	}
	if c, err := tx.Channel(p.ChannelID); err == nil {
		out.Channel = channelRef(c)
	}
	return out
}

func adminDisputeBrief(tx *store.Tx, p *store.Placement) *api.AdminDisputeBrief {
	s := disputeSummary(p)
	if s == nil {
		return nil
	}
	d := p.Dispute
	out := recast[api.AdminDisputeBrief](s)
	out.Id = d.ID
	out.Overdue = d.Resolution == "" && tx.Now.After(d.DeadlineAt)
	out.SuggestedShare, out.PerformanceShare = d.SuggestedShare, d.PerformanceShare
	out.Comment = strPtr(d.Comment)
	out.ResolvedBy = userRefPtr(tx, d.ResolvedBy)
	return &out
}

func adminPayout(tx *store.Tx, po *store.Payout) api.AdminPayout {
	method := po.Method
	if method == "" {
		method = "rocketwork"
	}
	out := api.AdminPayout{Id: po.ID, PlacementId: po.PlacementID, Amount: po.Amount, Status: api.PayoutStatus(po.Status),
		Method: api.PayoutMethod(method), Attempt: po.Attempt, ReceiptUrl: strPtr(po.ReceiptURL), CreatedAt: po.CreatedAt,
		CompletedAt: po.CompletedAt, Blogger: orgRef(tx, po.OrgID), BankReference: strPtr(po.BankReference),
		FailureMessage: strPtr(po.FailureMessage)}
	if po.FailureCode != "" {
		out.FailureCode = ptr(api.PayoutFailureCode(po.FailureCode))
	}
	if p, err := tx.Placement(po.PlacementID); err == nil {
		if c, err := tx.Channel(p.ChannelID); err == nil {
			out.Channel = channelRef(c)
		}
		if o, err := tx.Order(p.OrderID); err == nil {
			out.OrderNumber = o.Number
		}
	}
	return out
}

// adminActions — действия по состоянию и роли (R-1).
func adminActions(tx *store.Tx, u *store.User, p *store.Placement) []api.AdminPlacementAction {
	out := []api.AdminPlacementAction{}
	sr := isSenior(u)
	if sr && store.Allowed("P-06:admin", p.Status) {
		out = append(out, api.AdminPlacementActionCancel)
	}
	if (p.Status == "published" || p.Status == "disputed") && p.PublishedAt != nil && p.RemovedAt == nil {
		out = append(out, api.AdminPlacementActionRemovePost)
	}
	if sr && store.Allowed("P-10:admin", p.Status) {
		out = append(out, api.AdminPlacementActionMarkRemoved)
	}
	if store.Allowed("P-14:admin", p.Status) {
		out = append(out, api.AdminPlacementActionOpenDispute)
	}
	if p.Status == "disputed" && p.Dispute != nil && p.Dispute.Resolution == "" {
		out = append(out, api.AdminPlacementActionResolveDispute)
	}
	if last := tx.LastPayout(p.ID); sr && last != nil && last.Status == "failed" && p.Status == "accepted" {
		out = append(out, api.AdminPlacementActionRetryPayout)
	}
	return out
}

func adminPlacement(tx *store.Tx, u *store.User, p *store.Placement) api.AdminPlacement {
	out := recast[api.AdminPlacement](adminPlacementItem(tx, p))
	ap := advertiserPlacement(tx, p)
	out.BloggerAmount, out.FeeAmount, out.PayoutAmount = p.BloggerAmount, p.FeeAmount, p.PayoutAmount
	out.PerformanceShare, out.PublicationUrl, out.Views = p.PerformanceShare, ap.PublicationUrl, p.Views
	if p.RemovalReason != "" {
		out.RemovalReason = ptr(api.AdminPlacementRemovalReason(p.RemovalReason))
	}
	out.Claim, out.History = ap.Claim, ap.History
	out.Dispute = adminDisputeBrief(tx, p)
	out.Violations = []api.PlacementViolation{}
	for _, v := range p.Violations {
		out.Violations = append(out.Violations, api.PlacementViolation{Kind: api.PlacementViolationKind(v.Kind),
			OccurredAt: v.OccurredAt, Source: api.PlacementViolationSource(v.Source)})
	}
	out.Payouts = []api.AdminPayout{}
	var pos []*store.Payout
	for _, po := range tx.D.Payouts {
		if po.PlacementID == p.ID {
			pos = append(pos, po)
		}
	}
	slices.SortFunc(pos, func(a, b *store.Payout) int { return b.Attempt - a.Attempt })
	for _, po := range pos {
		out.Payouts = append(out.Payouts, adminPayout(tx, po))
	}
	out.AvailableActions = adminActions(tx, u, p)
	return out
}

func (h *Handlers) ListAdminPlacements(ctx context.Context, req api.ListAdminPlacementsRequestObject) (api.ListAdminPlacementsResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	if _, err := admin(ctx, tx); err != nil {
		return nil, err
	}
	q := req.Params
	var found []*store.Placement
	for _, p := range tx.D.Placements {
		switch {
		case q.Status != nil && string(*q.Status) != p.Status,
			q.ChannelId != nil && *q.ChannelId != p.ChannelID,
			q.OrderId != nil && *q.OrderId != p.OrderID,
			!inPeriod(p.CreatedAt, q.From, q.To),
			q.RemovalOverdue != nil && *q.RemovalOverdue != removalOverdue(tx, p):
			continue
		}
		found = append(found, p)
	}
	found = newestFirst(found, func(p *store.Placement) time.Time { return p.CreatedAt })
	pg, next, total := page(found, q.Limit, q.Cursor)
	items := []api.AdminPlacementListItem{}
	for _, p := range pg {
		items = append(items, adminPlacementItem(tx, p))
	}
	return api.ListAdminPlacements200JSONResponse{Items: items, NextCursor: next, Total: total}, nil
}

func (h *Handlers) GetAdminPlacement(ctx context.Context, req api.GetAdminPlacementRequestObject) (api.GetAdminPlacementResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := admin(ctx, tx)
	if err != nil {
		return nil, err
	}
	p, err := tx.Placement(req.Id)
	if err != nil {
		return nil, err
	}
	return api.GetAdminPlacement200JSONResponse(adminPlacement(tx, u, p)), nil
}

// placementOp выполняет ручную операцию над размещением и возвращает карточку.
func (h *Handlers) placementOp(ctx context.Context, id uuid.UUID, seniorOnly bool, op string,
	fn func(tx *store.Tx, u *store.User, p *store.Placement) error) (api.AdminPlacement, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := adminWith(ctx, tx, seniorOnly)
	if err != nil {
		return api.AdminPlacement{}, err
	}
	p, err := tx.Placement(id)
	if err != nil {
		return api.AdminPlacement{}, err
	}
	if err := fn(tx, u, p); err != nil {
		return api.AdminPlacement{}, err
	}
	h.logger(ctx).Warn("admin placement operation", "op", op, "placement_id", p.ID, "admin_user_id", u.ID)
	return adminPlacement(tx, u, p), nil
}

func (h *Handlers) AdminCancelPlacement(ctx context.Context, req api.AdminCancelPlacementRequestObject) (api.AdminCancelPlacementResponseObject, error) {
	out, err := h.placementOp(ctx, req.Id, true, "Admin.CancelPlacement", func(tx *store.Tx, u *store.User, p *store.Placement) error {
		return tx.AdminCancel(u.ID, p, req.Body.Reason)
	})
	if err != nil {
		return nil, err
	}
	return api.AdminCancelPlacement200JSONResponse(out), nil
}

func (h *Handlers) AdminRemovePost(ctx context.Context, req api.AdminRemovePostRequestObject) (api.AdminRemovePostResponseObject, error) {
	out, err := h.placementOp(ctx, req.Id, false, "Admin.RemovePost", func(tx *store.Tx, u *store.User, p *store.Placement) error {
		return tx.AdminRemovePost(u.ID, p, req.Body.Reason)
	})
	if err != nil {
		return nil, err
	}
	return api.AdminRemovePost200JSONResponse(out), nil
}

func (h *Handlers) AdminMarkRemoved(ctx context.Context, req api.AdminMarkRemovedRequestObject) (api.AdminMarkRemovedResponseObject, error) {
	out, err := h.placementOp(ctx, req.Id, true, "Admin.MarkRemoved", func(tx *store.Tx, u *store.User, p *store.Placement) error {
		return tx.AdminMarkRemoved(u.ID, p, req.Body.RemovedAt.UTC(), req.Body.Reason)
	})
	if err != nil {
		return nil, err
	}
	return api.AdminMarkRemoved200JSONResponse(out), nil
}

func (h *Handlers) AdminOpenDispute(ctx context.Context, req api.AdminOpenDisputeRequestObject) (api.AdminOpenDisputeResponseObject, error) {
	out, err := h.placementOp(ctx, req.Id, false, "Admin.OpenDispute", func(tx *store.Tx, u *store.User, p *store.Placement) error {
		return tx.AdminOpenDispute(u.ID, p, req.Body.Reason)
	})
	if err != nil {
		return nil, err
	}
	return api.AdminOpenDispute200JSONResponse(out), nil
}

// ---------------------------------------------------------------- споры

func adminDisputeItem(tx *store.Tx, p *store.Placement) api.AdminDisputeListItem {
	out := recast[api.AdminDisputeListItem](adminDisputeBrief(tx, p))
	item := adminPlacementItem(tx, p)
	out.PlacementId, out.OrderNumber, out.Channel = p.ID, item.OrderNumber, item.Channel
	out.Advertiser, out.Blogger = item.Advertiser, item.Blogger
	out.PlacementStatus, out.TotalAmount = api.PlacementStatus(p.Status), p.TotalAmount
	return out
}

func disputeMessage(m *store.DisputeMessage) api.DisputeMessage {
	out := api.DisputeMessage{Id: m.ID, AuthorType: api.DisputeMessageAuthorType(m.AuthorType), Text: m.Text, CreatedAt: m.CreatedAt}
	if m.Recipient != "" {
		out.Recipient = ptr(api.DisputeMessageRecipient(m.Recipient))
	}
	return out
}

// disputeMessages — переписка; side пусто — все сообщения (администратор).
func disputeMessages(tx *store.Tx, p *store.Placement, side string) []api.DisputeMessage {
	out := []api.DisputeMessage{}
	for _, m := range tx.D.DisputeMessages {
		if m.PlacementID == p.ID && (side == "" || m.VisibleTo(side)) {
			out = append(out, disputeMessage(m))
		}
	}
	return out
}

func disputePreview(p *store.Placement, resolution string, share int) api.DisputePreview {
	d := store.Distribute(p, share)
	return api.DisputePreview{Resolution: api.DisputeResolution(resolution), Share: share, TotalAmount: d.TotalAmount,
		BloggerAmount: d.Payout, FeeAmount: d.Fee, RefundAmount: d.Refund}
}

func adminDispute(tx *store.Tx, u *store.User, p *store.Placement) api.AdminDispute {
	out := recast[api.AdminDispute](adminDisputeItem(tx, p))
	out.Placement = adminPlacement(tx, u, p)
	out.Messages = disputeMessages(tx, p, "")
	if d := p.Dispute; d.Resolution == "" && d.SuggestedShare != nil {
		pv := disputePreview(p, "proportional", *d.SuggestedShare)
		out.Preview = &pv
	}
	return out
}

func (h *Handlers) ListAdminDisputes(ctx context.Context, req api.ListAdminDisputesRequestObject) (api.ListAdminDisputesResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	if _, err := admin(ctx, tx); err != nil {
		return nil, err
	}
	q := req.Params
	state := "open"
	if q.State != nil {
		state = string(*q.State)
	}
	var found []*store.Placement
	for _, p := range tx.D.Placements {
		d := p.Dispute
		switch {
		case d == nil,
			state == "open" && d.Resolution != "",
			state == "resolved" && d.Resolution == "",
			q.Overdue != nil && *q.Overdue != (d.Resolution == "" && tx.Now.After(d.DeadlineAt)):
			continue
		}
		found = append(found, p)
	}
	if state == "open" {
		found = slices.Clone(found)
		slices.SortStableFunc(found, func(a, b *store.Placement) int { return a.Dispute.DeadlineAt.Compare(b.Dispute.DeadlineAt) })
	} else {
		found = newestFirst(found, func(p *store.Placement) time.Time { return p.Dispute.OpenedAt })
	}
	pg, next, total := page(found, q.Limit, q.Cursor)
	items := []api.AdminDisputeListItem{}
	for _, p := range pg {
		items = append(items, adminDisputeItem(tx, p))
	}
	return api.ListAdminDisputes200JSONResponse{Items: items, NextCursor: next, Total: total}, nil
}

func (h *Handlers) GetAdminDispute(ctx context.Context, req api.GetAdminDisputeRequestObject) (api.GetAdminDisputeResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := admin(ctx, tx)
	if err != nil {
		return nil, err
	}
	p, err := tx.DisputeByID(req.Id)
	if err != nil {
		return nil, err
	}
	return api.GetAdminDispute200JSONResponse(adminDispute(tx, u, p)), nil
}

func (h *Handlers) PreviewDisputeResolution(ctx context.Context, req api.PreviewDisputeResolutionRequestObject) (api.PreviewDisputeResolutionResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	if _, err := admin(ctx, tx); err != nil {
		return nil, err
	}
	p, err := tx.DisputeByID(req.Id)
	if err != nil {
		return nil, err
	}
	res := string(req.Params.Resolution)
	share, err := store.ResolutionShare(p.Dispute, res, req.Params.Share)
	if err != nil {
		return nil, err
	}
	return api.PreviewDisputeResolution200JSONResponse(disputePreview(p, res, share)), nil
}

func (h *Handlers) ResolveDispute(ctx context.Context, req api.ResolveDisputeRequestObject) (api.ResolveDisputeResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := admin(ctx, tx)
	if err != nil {
		return nil, err
	}
	p, err := tx.DisputeByID(req.Id)
	if err != nil {
		return nil, err
	}
	b := req.Body
	if err := tx.ResolveDispute(u.ID, p, string(b.Resolution), b.Share, b.Comment); err != nil {
		return nil, err
	}
	h.logger(ctx).Warn("dispute resolved", "op", "Admin.ResolveDispute", "dispute_id", p.Dispute.ID,
		"resolution", b.Resolution, "admin_user_id", u.ID)
	return api.ResolveDispute200JSONResponse(adminDispute(tx, u, p)), nil
}

func (h *Handlers) ListAdminDisputeMessages(ctx context.Context, req api.ListAdminDisputeMessagesRequestObject) (api.ListAdminDisputeMessagesResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	if _, err := admin(ctx, tx); err != nil {
		return nil, err
	}
	p, err := tx.DisputeByID(req.Id)
	if err != nil {
		return nil, err
	}
	return api.ListAdminDisputeMessages200JSONResponse{Items: disputeMessages(tx, p, "")}, nil
}

func (h *Handlers) CreateAdminDisputeMessage(ctx context.Context, req api.CreateAdminDisputeMessageRequestObject) (api.CreateAdminDisputeMessageResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := admin(ctx, tx)
	if err != nil {
		return nil, err
	}
	p, err := tx.DisputeByID(req.Id)
	if err != nil {
		return nil, err
	}
	m, err := tx.PostDisputeMessage(p, "admin", u.ID, string(req.Body.To), req.Body.Text)
	if err != nil {
		return nil, err
	}
	return api.CreateAdminDisputeMessage201JSONResponse(disputeMessage(m)), nil
}

// partyDispute — размещение стороны со спором; спора нет — 404.
func partyDispute(p *store.Placement) error {
	if p.Dispute == nil {
		return store.NotFound("dispute", p.ID)
	}
	return nil
}

func (h *Handlers) ListPlacementDisputeMessages(ctx context.Context, req api.ListPlacementDisputeMessagesRequestObject) (api.ListPlacementDisputeMessagesResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	p, err := advPlacement(tx, o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	if err := partyDispute(p); err != nil {
		return nil, err
	}
	return api.ListPlacementDisputeMessages200JSONResponse{Items: disputeMessages(tx, p, "advertiser")}, nil
}

func (h *Handlers) CreatePlacementDisputeMessage(ctx context.Context, req api.CreatePlacementDisputeMessageRequestObject) (api.CreatePlacementDisputeMessageResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, o, err := advertiser(ctx, tx)
	if err != nil {
		return nil, err
	}
	p, err := advPlacement(tx, o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	m, err := tx.PostDisputeMessage(p, "advertiser", u.ID, "", req.Body.Text)
	if err != nil {
		return nil, err
	}
	return api.CreatePlacementDisputeMessage201JSONResponse(disputeMessage(m)), nil
}

func (h *Handlers) ListBloggerDisputeMessages(ctx context.Context, req api.ListBloggerDisputeMessagesRequestObject) (api.ListBloggerDisputeMessagesResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	_, o, err := blogger(ctx, tx)
	if err != nil {
		return nil, err
	}
	p, err := blgPlacement(tx, o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	if err := partyDispute(p); err != nil {
		return nil, err
	}
	return api.ListBloggerDisputeMessages200JSONResponse{Items: disputeMessages(tx, p, "blogger")}, nil
}

func (h *Handlers) CreateBloggerDisputeMessage(ctx context.Context, req api.CreateBloggerDisputeMessageRequestObject) (api.CreateBloggerDisputeMessageResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, o, err := blogger(ctx, tx)
	if err != nil {
		return nil, err
	}
	p, err := blgPlacement(tx, o.ID, req.Id)
	if err != nil {
		return nil, err
	}
	m, err := tx.PostDisputeMessage(p, "blogger", u.ID, "", req.Body.Text)
	if err != nil {
		return nil, err
	}
	return api.CreateBloggerDisputeMessage201JSONResponse(disputeMessage(m)), nil
}

// ---------------------------------------------------------------- каналы и категории

func (h *Handlers) BlockChannel(ctx context.Context, req api.BlockChannelRequestObject) (api.BlockChannelResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := senior(ctx, tx)
	if err != nil {
		return nil, err
	}
	c, err := adminTarget(tx, req.Id)
	if err != nil {
		return nil, err
	}
	if err := tx.BlockChannel(u.ID, c, req.Body.Reason); err != nil {
		return nil, err
	}
	h.logger(ctx).Warn("admin channel blocked", "op", "Admin.BlockChannel", "channel_id", c.ID, "admin_user_id", u.ID)
	return api.BlockChannel200JSONResponse(adminChannel(tx, c)), nil
}

func (h *Handlers) UnblockChannel(ctx context.Context, req api.UnblockChannelRequestObject) (api.UnblockChannelResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := senior(ctx, tx)
	if err != nil {
		return nil, err
	}
	c, err := adminTarget(tx, req.Id)
	if err != nil {
		return nil, err
	}
	if err := tx.UnblockChannel(u.ID, c, req.Body.Reason); err != nil {
		return nil, err
	}
	h.logger(ctx).Warn("admin channel unblocked", "op", "Admin.UnblockChannel", "channel_id", c.ID, "admin_user_id", u.ID)
	return api.UnblockChannel200JSONResponse(adminChannel(tx, c)), nil
}

func adminCategory(c *store.Category) api.AdminChannelCategory {
	return api.AdminChannelCategory{Id: c.ID, Slug: c.Slug, Name: c.Name, Position: c.Position, Archived: c.Archived}
}

func (h *Handlers) CreateChannelCategory(ctx context.Context, req api.CreateChannelCategoryRequestObject) (api.CreateChannelCategoryResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := senior(ctx, tx)
	if err != nil {
		return nil, err
	}
	b := req.Body
	pos := 0
	for _, c := range tx.D.Categories {
		if c.Slug == b.Slug {
			return nil, store.ConflictReason("slug_taken", "category slug is taken", nil)
		}
		pos = max(pos, c.Position+1)
	}
	if b.Position != nil {
		pos = *b.Position
	}
	c := &store.Category{ID: store.NewID(), Slug: b.Slug, Name: b.Name, Position: pos}
	tx.D.Categories = append(tx.D.Categories, c)
	slices.SortStableFunc(tx.D.Categories, func(a, b *store.Category) int { return a.Position - b.Position })
	tx.Audit(u.ID, nil, "channel_category.created", "channel_category", c.ID.String(), "", map[string]any{"slug": c.Slug})
	return api.CreateChannelCategory201JSONResponse(adminCategory(c)), nil
}

func (h *Handlers) UpdateChannelCategory(ctx context.Context, req api.UpdateChannelCategoryRequestObject) (api.UpdateChannelCategoryResponseObject, error) {
	tx, done := h.st.Begin(ctx)
	defer done()
	u, err := senior(ctx, tx)
	if err != nil {
		return nil, err
	}
	c := tx.Category(req.Id)
	if c == nil {
		return nil, store.NotFound("channel_category", req.Id)
	}
	b := req.Body
	if b.Name != nil {
		c.Name = *b.Name
	}
	if b.Position != nil {
		c.Position = *b.Position
	}
	if b.Archived != nil {
		c.Archived = *b.Archived
	}
	slices.SortStableFunc(tx.D.Categories, func(a, b *store.Category) int { return a.Position - b.Position })
	tx.Audit(u.ID, nil, "channel_category.updated", "channel_category", c.ID.String(), "", nil)
	return api.UpdateChannelCategory200JSONResponse(adminCategory(c)), nil
}
