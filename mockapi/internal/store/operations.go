package store

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"time"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------- корзина

// CartLine — позиция корзины с текущей ценой.
type CartLine struct {
	Item         *CartItem
	Channel      *Channel
	CurrentTotal *int64
	Available    bool
}

// Cart возвращает позиции корзины организации с проверкой доступности.
func (tx *Tx) Cart(orgID uuid.UUID) []CartLine {
	var out []CartLine
	for _, it := range tx.D.CartItems {
		if it.OrgID != orgID {
			continue
		}
		line := CartLine{Item: it}
		if c, err := tx.Channel(it.ChannelID); err == nil {
			line.Channel = c
			if pr := c.Prices[it.FormatID]; pr != nil && pr.IsEnabled {
				_, total := TotalFor(pr.Amount)
				line.CurrentTotal = &total
				line.Available = c.Status == "active"
			}
		}
		out = append(out, line)
	}
	return out
}

// ---------------------------------------------------------------- оформление

// Checkout оформляет заказ кампании из корзины (S-06, P-01).
func (tx *Tx) Checkout(org *Organization, userID uuid.UUID, c *Campaign) (*Order, error) {
	log := tx.log.With("op", "Order.Checkout", "organization_id", org.ID, "campaign_id", c.ID)
	if c.Status != "draft" {
		return nil, Conflict("conflict", "campaign is already ordered", map[string]any{"status": c.Status})
	}
	var missing []string
	if c.DateFrom == nil || c.DateTo == nil {
		missing = append(missing, "date_from", "date_to")
	}
	if c.TimeFrom == nil || c.TimeTo == nil {
		missing = append(missing, "time_from", "time_to")
	}
	body, problems := CheckDraft(c.Post)
	if len(missing) > 0 || len(problems) > 0 {
		log.Warn("checkout rejected: campaign incomplete", "missing", missing, "problems", len(problems))
		return nil, Conflict("campaign_incomplete", "campaign is not ready for payment",
			map[string]any{"missing_fields": missing, "post_problems": problems})
	}
	windowStart, windowEnd := WindowFor(*c.DateFrom, *c.DateTo)
	if !windowEnd.After(tx.Now.Add(time.Hour)) {
		return nil, Conflict("campaign_incomplete", "campaign date range is in the past",
			map[string]any{"missing_fields": []string{"date_to"}})
	}
	lines := tx.Cart(org.ID)
	if len(lines) == 0 {
		return nil, Conflict("cart_changed", "cart is empty", map[string]any{"items": []string{}})
	}
	var bad []string
	var total int64
	for _, l := range lines {
		if !l.Available || *l.CurrentTotal != l.Item.ShownTotal {
			bad = append(bad, l.Item.ID.String())
			continue
		}
		total += *l.CurrentTotal
	}
	if len(bad) > 0 {
		log.Warn("checkout rejected: cart changed", "items", bad)
		return nil, Conflict("cart_changed", "some cart items are unavailable or changed price", map[string]any{"items": bad})
	}
	available, _ := tx.Balance(org.ID)
	if available < total {
		log.Warn("checkout rejected: insufficient balance", "available", available, "total_amount", total)
		return nil, Conflict("insufficient_balance", "balance is less than order total",
			map[string]any{"total_amount": total, "available_amount": available, "missing_amount": total - available})
	}

	v := &PostVersion{
		ID: NewID(), Number: 1, Status: "pending_moderation", Doc: c.Post.Doc, Body: body,
		MediaIDs: slices.Clone(c.Post.MediaIDs), TokenCarrier: c.Post.TokenCarrier, TokenPosition: c.Post.TokenPosition,
		AdvertiserName: org.Name, AdvertiserINN: org.INN, CreatedAt: tx.Now,
	}
	o := &Order{ID: NewID(), Number: tx.D.NextOrderNo, OrgID: org.ID, CampaignID: c.ID,
		Versions: []*PostVersion{v}, Status: "moderation", TotalAmount: total, CreatedAt: tx.Now}
	tx.D.NextOrderNo++
	tx.D.Orders = append(tx.D.Orders, o)
	for _, l := range lines {
		f := tx.Format(l.Item.FormatID)
		price := l.Channel.Prices[l.Item.FormatID].Amount
		fee, ptotal := TotalFor(price)
		p := &Placement{
			ID: NewID(), OrderID: o.ID, ChannelID: l.Channel.ID, BloggerOrgID: *l.Channel.OrgID,
			FormatID: f.ID, TopHours: f.TopHours, DurationHours: f.DurationHours,
			WindowStart: windowStart, WindowEnd: windowEnd, TimeFrom: *c.TimeFrom, TimeTo: *c.TimeTo,
			PostVersionID: v.ID, FeeRateBP: FeeRateBP, BloggerAmount: price, FeeAmount: fee, TotalAmount: ptotal,
			CreatedAt: tx.Now,
		}
		tx.D.Placements = append(tx.D.Placements, p)
		if err := tx.apply(p, "P-01", ""); err != nil {
			return nil, err
		}
		tx.hold(o, p, l.Channel.Title)
	}
	tx.D.CartItems = slices.DeleteFunc(tx.D.CartItems, func(it *CartItem) bool { return it.OrgID == org.ID })
	c.Status = "ordered"
	c.OrderID = &o.ID
	c.UpdatedAt = tx.Now
	tx.recalcOrder(o)
	log.Info("order created", "order_id", o.ID, "order_number", o.Number, "total_amount", total, "placements", len(lines), "user_id", userID)
	return o, nil
}

// NewPostVersion отправляет изменённый пост в заказ (S-29, P-11).
func (tx *Tx) NewPostVersion(org *Organization, o *Order, c *Campaign) error {
	log := tx.log.With("op", "Order.NewPostVersion", "order_id", o.ID)
	if o.Status == "completed" || o.Status == "cancelled" {
		return Conflict("conflict", "order is finished", map[string]any{"status": o.Status})
	}
	if cur := o.CurrentVersion(); cur.Status == "pending_moderation" || cur.Status == "approved" {
		return Conflict("conflict", "current post version is still being processed", map[string]any{"post_version_status": cur.Status})
	}
	body, problems := CheckDraft(c.Post)
	if len(problems) > 0 {
		return Conflict("campaign_incomplete", "post does not pass automatic checks", map[string]any{"post_problems": problems})
	}
	ps := tx.OrderPlacements(o.ID)
	var need int64
	for _, p := range ps {
		if (p.Status == "created" || Allowed("P-11", p.Status)) && !p.HoldActive {
			need += p.TotalAmount
		}
	}
	if available, _ := tx.Balance(org.ID); available < need {
		return Conflict("insufficient_balance", "balance is less than amount to hold again",
			map[string]any{"missing_amount": need - available})
	}
	v := &PostVersion{
		ID: NewID(), Number: len(o.Versions) + 1, Status: "pending_moderation", Doc: c.Post.Doc, Body: body,
		MediaIDs: slices.Clone(c.Post.MediaIDs), TokenCarrier: c.Post.TokenCarrier, TokenPosition: c.Post.TokenPosition,
		AdvertiserName: org.Name, AdvertiserINN: org.INN, CreatedAt: tx.Now,
	}
	o.Versions = append(o.Versions, v)
	for _, p := range ps {
		if Allowed("P-11", p.Status) {
			if err := tx.apply(p, "P-11", "post_changed"); err != nil {
				return err
			}
			p.AcceptDeadlineAt, p.ScheduleDeadlineAt, p.PublishAt, p.RemoveAt = nil, nil, nil, nil
		}
		if p.Status == "created" {
			p.PostVersionID = v.ID
			if !p.HoldActive {
				ch, _ := tx.Channel(p.ChannelID)
				tx.hold(o, p, ch.Title)
			}
		}
	}
	tx.recalcOrder(o)
	log.Info("post version created", "post_version_id", v.ID, "number", v.Number)
	return nil
}

// ApprovePost заменяет модерацию и ОРД: версия становится ready с выдуманным
// erid, размещения из created переходят в awaiting_blogger (P-02).
func (tx *Tx) ApprovePost(o *Order) error {
	v := o.CurrentVersion()
	if v.Status != "pending_moderation" {
		return Conflict("conflict", "post version is not pending moderation", map[string]any{"post_version_status": v.Status})
	}
	v.Status = "approved"
	v.Erid = fmt.Sprintf("Mock%08X", rand.Uint32())
	v.Status = "ready"
	v.FinalText = FinalText(v.Body, v.TokenCarrier, v.TokenPosition, v.AdvertiserName, v.AdvertiserINN, v.Erid)
	tx.log.Info("post version ready", "op", "Post.Approve", "order_id", o.ID, "post_version_id", v.ID, "erid", v.Erid)
	for _, p := range tx.OrderPlacements(o.ID) {
		if p.Status != "created" {
			continue
		}
		ch, err := tx.Channel(p.ChannelID)
		if err != nil || ch.Status != "active" || !p.HoldActive || !p.WindowEnd.After(tx.Now.Add(time.Hour)) {
			tx.log.Warn("placement skipped on approve", "op", "Post.Approve", "placement_id", p.ID)
			continue
		}
		dl := minTime(tx.Now.Add(24*time.Hour), p.WindowEnd.Add(-time.Hour)) // T-01
		p.AcceptDeadlineAt = &dl
		p.PostVersionID = v.ID
		if err := tx.apply(p, "P-02", ""); err != nil {
			return err
		}
	}
	tx.recalcOrder(o)
	return nil
}

// RejectPost заменяет отклонение поста администратором (S-17): размещения
// остаются в created, их удержания снимаются (03 §4, особые правила).
func (tx *Tx) RejectPost(o *Order, reason string) error {
	v := o.CurrentVersion()
	if v.Status != "pending_moderation" {
		return Conflict("conflict", "post version is not pending moderation", map[string]any{"post_version_status": v.Status})
	}
	v.Status = "rejected"
	v.RejectionReason = reason
	for _, p := range tx.OrderPlacements(o.ID) {
		if p.Status == "created" {
			tx.releaseHold(p)
		}
	}
	tx.log.Info("post version rejected", "op", "Post.Reject", "order_id", o.ID, "post_version_id", v.ID)
	tx.Notify(o.OrgID, "post.rejected", "Пост отклонён", fmt.Sprintf("Заказ %d: %s", o.Number, reason), "order", o.ID)
	tx.recalcOrder(o)
	return nil
}

// ---------------------------------------------------------------- действия рекламодателя

func (tx *Tx) CancelPlacement(p *Placement) error { return tx.apply(p, "P-06", "advertiser_cancel") }

func (tx *Tx) AcceptPlacement(p *Placement) error {
	if !Allowed("P-12", p.Status) {
		return tx.apply(p, "P-12", "")
	}
	tx.distributeFull(p)
	return tx.apply(p, "P-12", "")
}

func (tx *Tx) ClaimPlacement(p *Placement, reason, text string) error {
	if p.Claim != nil {
		return Conflict("conflict", "claim already exists for this placement", nil)
	}
	if p.Status == "removed" && p.ClaimDeadlineAt != nil && !tx.Now.Before(*p.ClaimDeadlineAt) {
		return Conflict("invalid_transition", "claim window is over", map[string]any{"transition": "P-14", "status": p.Status})
	}
	if err := tx.apply(p, "P-14", "claim_"+reason); err != nil {
		return err
	}
	p.Claim = &Claim{Reason: reason, Text: text, CreatedAt: tx.Now}
	return nil
}

// ---------------------------------------------------------------- действия блогера

func (tx *Tx) BloggerAccept(org *Organization, p *Placement) error {
	if !Allowed("P-03", p.Status) {
		return tx.apply(p, "P-03", "")
	}
	if org.LegalType != "legal_entity" {
		if rw := tx.D.Rocketwork[org.ID]; rw == nil || rw.Status != "active" {
			return Conflict("conflict", "rocketwork contractor is not active", map[string]any{"hint": "POST /blogger/rocketwork/invite"})
		}
	}
	dl := minTime(tx.Now.Add(24*time.Hour), p.WindowEnd) // T-13
	p.ScheduleDeadlineAt = &dl
	return tx.apply(p, "P-03", "")
}

func (tx *Tx) BloggerDecline(p *Placement) error { return tx.apply(p, "P-04", "declined_by_blogger") }

// checkPublishAt проверяет время публикации: I-06, I-07, A-11, D-17.
func (tx *Tx) checkPublishAt(p *Placement, at time.Time, now bool) error {
	if at.Before(p.WindowStart) || at.After(p.WindowEnd) {
		return Validation("publish_at", "publish_at is outside campaign date range")
	}
	if !InTimeRange(at, p.TimeFrom, p.TimeTo) {
		return Validation("publish_at", "publish_at is outside campaign time of day range")
	}
	if !now && at.Before(tx.Now.Add(time.Hour)) {
		return Validation("publish_at", "publish_at must be at least 1 hour from now")
	}
	if ch, err := tx.Channel(p.ChannelID); err == nil {
		for _, u := range ch.Unavailability {
			if !at.Before(u.StartsAt) && at.Before(u.EndsAt) {
				return Validation("publish_at", "publish_at falls into channel unavailability")
			}
		}
	}
	return nil
}

func (tx *Tx) BloggerSchedule(p *Placement, mode string, at *time.Time) error {
	if !Allowed("P-17", p.Status) {
		return tx.apply(p, "P-17", "")
	}
	t := tx.Now
	if mode == "at" {
		if at == nil {
			return Validation("publish_at", "publish_at is required for mode=at")
		}
		t = at.UTC().Truncate(time.Second)
	}
	if err := tx.checkPublishAt(p, t, mode == "now"); err != nil {
		return err
	}
	rm := t.Add(time.Duration(p.DurationHours) * time.Hour)
	p.PublishAt, p.RemoveAt = &t, &rm
	return tx.apply(p, "P-17", "schedule_"+mode)
}

func (tx *Tx) BloggerReschedule(p *Placement, at time.Time) error {
	if !Allowed("P-07", p.Status) {
		return tx.apply(p, "P-07", "")
	}
	if p.PublishAt != nil && p.PublishAt.Sub(tx.Now) <= time.Hour {
		return Conflict("invalid_transition", "less than 1 hour left before publish_at",
			map[string]any{"transition": "P-07", "status": p.Status})
	}
	at = at.UTC().Truncate(time.Second)
	if err := tx.checkPublishAt(p, at, false); err != nil {
		return err
	}
	rm := at.Add(time.Duration(p.DurationHours) * time.Hour)
	p.PublishAt, p.RemoveAt = &at, &rm
	return tx.apply(p, "P-07", "rescheduled")
}

// ---------------------------------------------------------------- системные переходы

// Advance выполняет системный переход вместо планировщика и внешних систем.
// Для P-08 и P-10 часы сдвигаются на publish_at и remove_at.
func (tx *Tx) Advance(p *Placement, code string) error {
	if !Allowed(code, p.Status) {
		return tx.apply(p, code, "")
	}
	switch code {
	case "P-08":
		if p.PublishAt != nil && p.PublishAt.After(tx.Now) {
			tx.shiftTo(*p.PublishAt)
		}
		ch, _ := tx.Channel(p.ChannelID)
		now := tx.Now
		rm := now.Add(time.Duration(p.DurationHours) * time.Hour)
		p.PublishedAt, p.RemoveAt = &now, &rm
		p.Views = ptr(0)
		p.PublicationURL = fmt.Sprintf("https://max.ru/c/%d/%d", -ch.MaxChatID, rand.IntN(90000)+10000)
	case "P-10":
		if p.RemoveAt != nil && p.RemoveAt.After(tx.Now) {
			tx.shiftTo(*p.RemoveAt)
		}
		now := tx.Now
		dl := now.Add(24 * time.Hour) // T-05
		p.RemovedAt, p.ClaimDeadlineAt = &now, &dl
		if ch, err := tx.Channel(p.ChannelID); err == nil {
			p.Views = ptr(ch.AvgViews + rand.IntN(ch.AvgViews/5+1))
		}
	case "P-16":
		for _, po := range tx.D.Payouts {
			if po.PlacementID == p.ID && po.Status != "succeeded" {
				now := tx.Now
				po.Status, po.CompletedAt = "succeeded", &now
				po.ReceiptURL = fmt.Sprintf("https://rocketwork.example.test/receipts/%s", po.ID)
			}
		}
	}
	return tx.apply(p, code, "")
}

// shiftTo сдвигает часы вперёд в пределах транзакции.
func (tx *Tx) shiftTo(t time.Time) {
	tx.s.clock.ShiftTo(t)
	tx.Now = tx.s.clock.Now()
	tx.log.Debug("clock shifted", "op", "Clock.Shift", "now", tx.Now)
}

// ---------------------------------------------------------------- каналы

// RecheckChannel повторяет автоматическую модерацию канала после правки.
// Заглушка одобряет канал сразу, если выполнено ограничение 04 §3.
func (tx *Tx) RecheckChannel(c *Channel) {
	switch c.Status {
	case "moderation", "pending_rights", "rejected", "active":
	default:
		return
	}
	prev := c.Status
	hasPrice := false
	for _, p := range c.Prices {
		hasPrice = hasPrice || p.IsEnabled
	}
	rknOK := c.Subscribers < 10000 || c.RknStatus == "confirmed"
	rights := c.BotIsAdmin && slices.Contains(c.BotPermissions, "write") && slices.Contains(c.BotPermissions, "delete")
	switch {
	case !c.IsPublic || !rights:
		c.Status, c.StatusReason = "pending_rights", "missing_rights"
	case c.CategoryID == nil || !hasPrice:
		c.Status, c.StatusReason = "moderation", ""
	case !rknOK:
		c.Status, c.StatusReason = "moderation", "rkn_required"
	default:
		c.Status, c.StatusReason = "active", ""
	}
	if prev != c.Status {
		tx.log.Info("channel status changed", "op", "Channel.Recheck", "channel_id", c.ID, "from", prev, "to", c.Status)
	}
}
