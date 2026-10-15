package server_test

import (
	"net/http"
	"testing"
	"time"

	"maxim/mockapi/internal/seed"
	"maxim/mockapi/internal/store"
)

type placementView struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Channel struct {
		ID string `json:"id"`
	} `json:"channel"`
	AvailableActions []string `json:"available_actions"`
}

type orderView struct {
	ID         string          `json:"id"`
	Status     string          `json:"status"`
	Placements []placementView `json:"placements"`
}

func (o orderView) placementOn(t *testing.T, channelID string) placementView {
	t.Helper()
	for _, p := range o.Placements {
		if p.Channel.ID == channelID {
			return p
		}
	}
	t.Fatalf("order has no placement on channel %s", channelID)
	return placementView{}
}

func expect(t *testing.T, r response, status int, what string) {
	t.Helper()
	if r.status != status {
		t.Fatalf("%s: status %d, want %d\nrequest_id=%s body=%s", what, r.status, status, r.header.Get("X-Request-Id"), truncate(r.body))
	}
}

type balanceView struct {
	Available int64 `json:"available_amount"`
	Held      int64 `json:"held_amount"`
}

// TestScenarioFullCycle — корзина → оформление → модерация → блогер принимает и
// планирует → публикация → снятие → приёмка → выплата; плюс таймер T-01.
func TestScenarioFullCycle(t *testing.T) {
	e := newEnv(t)
	a, b, anon := e.client(adv), e.client(blg), e.client("")
	campaign := "/campaigns/" + seed.CampaignDraft.String()

	var before balanceView
	e.do(a, http.MethodGet, "/balance", nil, "").json(t, &before)

	// Оформление (S-06, P-01): в корзине два канала из начальных данных.
	r := e.do(a, http.MethodPost, campaign+"/checkout", nil, "")
	expect(t, r, http.StatusCreated, "checkout")
	var o orderView
	r.json(t, &o)
	if o.Status != "moderation" || len(o.Placements) != 2 {
		t.Fatalf("order after checkout: status %s, placements %d", o.Status, len(o.Placements))
	}
	var afterCheckout balanceView
	e.do(a, http.MethodGet, "/balance", nil, "").json(t, &afterCheckout)
	if afterCheckout.Held <= before.Held || afterCheckout.Available >= before.Available {
		t.Fatalf("money is not held: before %+v, after %+v", before, afterCheckout)
	}
	expect(t, e.do(a, http.MethodPost, campaign+"/checkout", nil, ""), http.StatusConflict, "second checkout")

	// Модерация и erid (S-17, P-02).
	r = e.do(anon, http.MethodPost, "/_mock/orders/"+o.ID+"/approve-post", nil, "")
	expect(t, r, http.StatusOK, "approve post")
	r.json(t, &o)
	mine := o.placementOn(t, seed.ChannelBlogger1.String())
	other := o.placementOn(t, seed.ID(107).String())
	if o.Status != "in_progress" || mine.Status != "awaiting_blogger" {
		t.Fatalf("after approve: order %s, placement %s", o.Status, mine.Status)
	}

	// Блогер принимает (P-03) и планирует (P-17) на завтра 12:00 МСК.
	pl := "/blogger/placements/" + mine.ID
	expect(t, e.do(b, http.MethodPost, pl+"/accept", nil, ""), http.StatusOK, "blogger accept")
	tomorrow := time.Now().In(store.MSK).AddDate(0, 0, 1)
	publishAt := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 12, 0, 0, 0, store.MSK).UTC()
	r = e.do(b, http.MethodPost, pl+"/schedule", map[string]any{"mode": "at", "publish_at": publishAt.Format(time.RFC3339)}, "")
	expect(t, r, http.StatusOK, "schedule")
	var bp placementView
	r.json(t, &bp)
	if bp.Status != "scheduled" {
		t.Fatalf("after schedule: %s", bp.Status)
	}
	// Время вне периода кампании отклоняется.
	expect(t, e.do(b, http.MethodPost, pl+"/reschedule", map[string]any{"publish_at": time.Now().AddDate(0, 0, 30).Format(time.RFC3339)}, ""),
		http.StatusUnprocessableEntity, "reschedule outside window")

	// Публикация и снятие (P-08, P-10) — вместо планировщика.
	adv := "/_mock/placements/" + mine.ID + "/advance"
	expect(t, e.do(anon, http.MethodPost, adv, map[string]any{"transition": "P-08"}, ""), http.StatusOK, "publish")
	expect(t, e.do(anon, http.MethodPost, adv, map[string]any{"transition": "P-10"}, ""), http.StatusOK, "remove")

	// Приёмка рекламодателем (P-12); повтор — недопустимый переход.
	r = e.do(a, http.MethodPost, "/placements/"+mine.ID+"/accept", nil, "")
	expect(t, r, http.StatusOK, "advertiser accept")
	var ap placementView
	r.json(t, &ap)
	if ap.Status != "accepted" {
		t.Fatalf("after accept: %s", ap.Status)
	}
	r = e.do(a, http.MethodPost, "/placements/"+mine.ID+"/accept", nil, "")
	expect(t, r, http.StatusConflict, "repeated accept")
	var errBody struct {
		Error struct{ Code string } `json:"error"`
	}
	r.json(t, &errBody)
	if errBody.Error.Code != "invalid_transition" {
		t.Fatalf("repeated accept error code: %s", errBody.Error.Code)
	}

	// Выплата прошла (P-16).
	r = e.do(anon, http.MethodPost, adv, map[string]any{"transition": "P-16"}, "")
	expect(t, r, http.StatusOK, "payout succeeded")
	r.json(t, &ap)
	if ap.Status != "paid" {
		t.Fatalf("after payout: %s", ap.Status)
	}
	var payouts struct {
		Items []struct {
			PlacementID string `json:"placement_id"`
			Status      string `json:"status"`
		} `json:"items"`
	}
	e.do(b, http.MethodGet, "/blogger/payouts", nil, "").json(t, &payouts)
	found := false
	for _, p := range payouts.Items {
		found = found || (p.PlacementID == mine.ID && p.Status == "succeeded")
	}
	if !found {
		t.Fatal("blogger does not see succeeded payout")
	}

	// Второе размещение никто не принял: часы ушли вперёд при публикации и снятии,
	// таймер T-01 (P-05) уже сработал, удержание снято, заказ завершён.
	r = e.do(a, http.MethodGet, "/orders/"+o.ID, nil, "")
	r.json(t, &o)
	if st := o.placementOn(t, seed.ID(107).String()).Status; st != "expired" {
		t.Fatalf("unaccepted placement %s: status %s, want expired", other.ID, st)
	}
	if o.Status != "completed" {
		t.Fatalf("order status after all placements finished: %s", o.Status)
	}
}

// TestTimerFiresOnClockShift — сдвиг часов выполняет T-01 и сообщает о переходе.
func TestTimerFiresOnClockShift(t *testing.T) {
	e := newEnv(t)
	anon := e.client("")
	var clock struct {
		Transitions []struct {
			PlacementID string `json:"placement_id"`
			Transition  string `json:"transition"`
			ToStatus    string `json:"to_status"`
		} `json:"transitions"`
	}
	// У размещения 801 срок принятия — через 20 часов от начальных данных.
	e.do(anon, http.MethodPost, "/_mock/clock", map[string]any{"hours": 21}, "").json(t, &clock)
	fired := false
	for _, tr := range clock.Transitions {
		fired = fired || (tr.PlacementID == seed.PlacementAwaitingBlogger.String() && tr.Transition == "P-05" && tr.ToStatus == "expired")
	}
	if !fired {
		t.Fatalf("T-01 did not fire: %+v", clock.Transitions)
	}
	var bp placementView
	e.do(e.client(blg), http.MethodGet, "/blogger/placements/"+seed.PlacementAwaitingBlogger.String(), nil, "").json(t, &bp)
	if bp.Status != "expired" || len(bp.AvailableActions) != 0 {
		t.Fatalf("after T-01: status %s, actions %v", bp.Status, bp.AvailableActions)
	}
}

// TestAccessIsolation — блогер не видит заказы рекламодателя, без входа — 401.
func TestAccessIsolation(t *testing.T) {
	e := newEnv(t)
	expect(t, e.do(e.client(""), http.MethodGet, "/orders", nil, ""), http.StatusUnauthorized, "orders without session")
	expect(t, e.do(e.client(blg), http.MethodGet, "/orders", nil, ""), http.StatusForbidden, "orders as blogger")
	expect(t, e.do(e.client(adv), http.MethodGet, "/blogger/channels", nil, ""), http.StatusForbidden, "blogger channels as advertiser")
	// Канал другого блогера — как будто его нет.
	expect(t, e.do(e.client(blg), http.MethodGet, "/blogger/channels/"+seed.ChannelFree.String(), nil, ""), http.StatusNotFound, "foreign channel")
}
