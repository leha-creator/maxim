package server_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"maxim/mockapi/internal/seed"
)

// Регрессии по замечаниям фронтенда (docs/frontend-api-gaps.md): ответы тестового
// сервера должны совпадать с контрактом и с настоящим бэкендом.

type errorView struct {
	Error struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

func errorOf(t *testing.T, r response, status int, code, reason string) errorView {
	t.Helper()
	expect(t, r, status, code)
	var e errorView
	r.json(t, &e)
	if e.Error.Code != code {
		t.Fatalf("code %q, want %q: %s", e.Error.Code, code, r.body)
	}
	if reason != "" && e.Error.Details["reason"] != reason {
		t.Fatalf("details.reason %v, want %q: %s", e.Error.Details["reason"], reason, r.body)
	}
	return e
}

func TestCheckoutIncompleteDetails(t *testing.T) {
	e := newEnv(t)
	c := e.client(adv)
	var camp struct {
		ID string `json:"id"`
	}
	r := e.do(c, http.MethodPost, "/campaigns", map[string]any{"name": "Без дат"}, "")
	expect(t, r, http.StatusCreated, "create campaign")
	r.json(t, &camp)

	er := errorOf(t, e.do(c, http.MethodPost, "/campaigns/"+camp.ID+"/checkout", nil, ""), http.StatusConflict, "campaign_incomplete", "")
	missing, ok := er.Error.Details["missing_fields"].([]any)
	if !ok || len(missing) == 0 {
		t.Fatalf("missing_fields = %v, want non-empty array", er.Error.Details["missing_fields"])
	}
	problems, ok := er.Error.Details["post_problems"].([]any)
	if !ok || len(problems) == 0 {
		t.Fatalf("post_problems = %v, want non-empty array", er.Error.Details["post_problems"])
	}
	p := problems[0].(map[string]any)
	if p["code"] != "empty_text" || p["message"] == nil {
		t.Fatalf("post problem keys: %v, want code/message", p)
	}
}

func TestCampaignPeriodReasons(t *testing.T) {
	e := newEnv(t)
	c := e.client(adv)
	path := "/campaigns/" + seed.CampaignDraft.String()
	var before struct {
		DateTo string `json:"date_to"`
	}
	e.do(c, http.MethodGet, path, nil, "").json(t, &before)

	day := func(n int) string { return time.Now().AddDate(0, 0, n).Format("2006-01-02") }
	cases := []struct {
		body   map[string]any
		reason string
	}{
		{map[string]any{"date_from": day(1), "date_to": day(20)}, "period_too_long"},
		{map[string]any{"date_from": day(5), "date_to": day(2)}, "period_order"},
		{map[string]any{"date_from": day(-3), "date_to": day(2)}, "period_in_past"},
		{map[string]any{"time_from": "10:00", "time_to": "10:30"}, "time_window"},
		{map[string]any{"name": "   "}, "required"},
	}
	for _, tc := range cases {
		errorOf(t, e.do(c, http.MethodPatch, path, tc.body, ""), http.StatusUnprocessableEntity, "validation_failed", tc.reason)
	}
	var after struct {
		DateTo string `json:"date_to"`
	}
	e.do(c, http.MethodGet, path, nil, "").json(t, &after)
	if after.DateTo != before.DateTo {
		t.Fatalf("rejected update changed campaign: date_to %s → %s", before.DateTo, after.DateTo)
	}
}

func TestDuplicateInnIsInnTaken(t *testing.T) {
	e := newEnv(t)
	body := map[string]any{"cabinet": "advertiser", "legal_type": "legal_entity", "inn": "7700000001",
		"name": "ООО «Двойник»", "contact_name": "Тест", "phone": "+79990000009", "address": "г. Москва"}
	errorOf(t, e.do(e.client("new@example.test"), http.MethodPost, "/organizations", body, ""), http.StatusConflict, "inn_taken", "")
}

func TestUnavailabilityRules(t *testing.T) {
	e := newEnv(t)
	c := e.client(blg)
	path := "/blogger/channels/" + seed.ID(103).String() + "/unavailability"
	// Сид: у канала 103 интервал day(10)…day(12).
	overlap := map[string]any{"starts_at": at(11 * 24 * time.Hour), "ends_at": at(13 * 24 * time.Hour)}
	errorOf(t, e.do(c, http.MethodPost, path, overlap, ""), http.StatusUnprocessableEntity, "validation_failed", "overlap")
	tooLong := map[string]any{"starts_at": at(20 * 24 * time.Hour), "ends_at": at(120 * 24 * time.Hour)}
	errorOf(t, e.do(c, http.MethodPost, path, tooLong, ""), http.StatusUnprocessableEntity, "validation_failed", "too_long")
	touching := map[string]any{"starts_at": at(13 * 24 * time.Hour), "ends_at": at(14 * 24 * time.Hour)}
	expect(t, e.do(c, http.MethodPost, path, touching, ""), http.StatusCreated, "non-overlapping interval")
}

func TestSameRknNumberKeepsCheck(t *testing.T) {
	e := newEnv(t)
	// Канал 104: номер подтверждён 100 суток назад — пора проверить снова.
	recheckDue := func() bool {
		var v struct {
			Due bool `json:"rkn_recheck_due"`
		}
		e.do(e.client(adm), http.MethodGet, "/admin/channels/"+seed.ID(104).String(), nil, "").json(t, &v)
		return v.Due
	}
	if !recheckDue() {
		t.Fatal("seed: channel 104 must be due for rkn recheck")
	}
	r := e.do(e.client(blg), http.MethodPatch, "/blogger/channels/"+seed.ID(104).String(),
		map[string]any{"rkn_number": "4900000001", "description": "Новое описание"}, "")
	expect(t, r, http.StatusOK, "update channel")
	if !recheckDue() {
		t.Fatal("same rkn number in update must not refresh the rkn check")
	}
}

func TestPostVersionInProgressReason(t *testing.T) {
	e := newEnv(t)
	c := e.client(adv)
	path := "/orders/" + seed.OrderActive.String() + "/post-versions"
	expect(t, e.do(c, http.MethodPost, path, nil, ""), http.StatusCreated, "first post version")
	errorOf(t, e.do(c, http.MethodPost, path, nil, ""), http.StatusConflict, "conflict", "post_version_in_progress")
}

func TestContractorInactiveReason(t *testing.T) {
	e := newEnv(t)
	tx, done := e.st.Begin(context.Background())
	tx.D.Rocketwork[seed.OrgBlogger].Status = "none"
	done()
	path := "/blogger/placements/" + seed.PlacementAwaitingBlogger.String() + "/accept"
	errorOf(t, e.do(e.client(blg), http.MethodPost, path, nil, ""), http.StatusConflict, "conflict", "contractor_inactive")
}

func TestReadAllNotifications(t *testing.T) {
	e := newEnv(t)
	c := e.client(adv)
	expect(t, e.do(c, http.MethodPost, "/notifications/read-all", nil, ""), http.StatusNoContent, "read-all")
	var page struct {
		Items []any `json:"items"`
		Total *int  `json:"total"`
	}
	e.do(c, http.MethodGet, "/notifications?unread=true", nil, "").json(t, &page)
	if len(page.Items) != 0 || page.Total == nil || *page.Total != 0 {
		t.Fatalf("unread after read-all: items=%d total=%v", len(page.Items), page.Total)
	}
}

func TestListTotal(t *testing.T) {
	e := newEnv(t)
	var page struct {
		Items      []any   `json:"items"`
		NextCursor *string `json:"next_cursor"`
		Total      *int    `json:"total"`
	}
	e.do(e.client(adv), http.MethodGet, "/catalog/channels?limit=2", nil, "").json(t, &page)
	if page.Total == nil || *page.Total <= len(page.Items) || page.NextCursor == nil {
		t.Fatalf("total=%v items=%d next=%v: want total over all pages", page.Total, len(page.Items), page.NextCursor)
	}
}

func TestOrderReportRowAmounts(t *testing.T) {
	e := newEnv(t)
	var rep struct {
		Rows []struct {
			Status   string `json:"status"`
			Spent    int64  `json:"spent_amount"`
			Refunded int64  `json:"refunded_amount"`
			Held     int64  `json:"held_amount"`
		} `json:"rows"`
		Totals struct {
			Spent    int64 `json:"spent_amount"`
			Refunded int64 `json:"refunded_amount"`
		} `json:"totals"`
	}
	r := e.do(e.client(adv), http.MethodGet, "/orders/"+seed.OrderPast.String()+"/report", nil, "")
	expect(t, r, http.StatusOK, "report")
	r.json(t, &rep)
	var spent, refunded int64
	for _, row := range rep.Rows {
		spent += row.Spent
		refunded += row.Refunded
		if row.Status == "disputed" && (row.Held == 0 || row.Spent != 0) {
			t.Fatalf("disputed row: spent=%d held=%d, want money held", row.Spent, row.Held)
		}
	}
	if spent != rep.Totals.Spent || refunded != rep.Totals.Refunded {
		t.Fatalf("rows sum spent=%d refunded=%d, totals %d/%d", spent, refunded, rep.Totals.Spent, rep.Totals.Refunded)
	}
}

func TestDisputeSummaryFilled(t *testing.T) {
	e := newEnv(t)
	var o struct {
		Placements []struct {
			ID      string `json:"id"`
			Dispute *struct {
				Reason     string    `json:"reason"`
				DeadlineAt time.Time `json:"deadline_at"`
				OpenedAt   time.Time `json:"opened_at"`
			} `json:"dispute"`
		} `json:"placements"`
	}
	e.do(e.client(adv), http.MethodGet, "/orders/"+seed.OrderPast.String(), nil, "").json(t, &o)
	for _, p := range o.Placements {
		if p.ID != seed.PlacementDisputed.String() {
			continue
		}
		if p.Dispute == nil || p.Dispute.Reason != "removed_early" || p.Dispute.DeadlineAt.Sub(p.Dispute.OpenedAt) != 72*time.Hour {
			t.Fatalf("dispute = %+v", p.Dispute)
		}
		return
	}
	t.Fatal("order has no disputed placement")
}

func TestAdminChannels(t *testing.T) {
	e := newEnv(t)
	var me struct {
		AdminRole *string `json:"admin_role"`
	}
	e.do(e.client(adm), http.MethodGet, "/me", nil, "").json(t, &me)
	if me.AdminRole == nil || *me.AdminRole != "senior" {
		t.Fatalf("admin_role = %v, want senior", me.AdminRole)
	}
	me.AdminRole = nil
	e.do(e.client(adv), http.MethodGet, "/me", nil, "").json(t, &me)
	if me.AdminRole != nil {
		t.Fatalf("advertiser admin_role = %v, want none", *me.AdminRole)
	}

	errorOf(t, e.do(e.client(adv), http.MethodGet, "/admin/channels", nil, ""), http.StatusForbidden, "forbidden", "admin_required")

	c := e.client(adm)
	path := "/admin/channels/" + seed.ChannelQueue.String()
	er := errorOf(t, e.do(c, http.MethodPost, path+"/approve", nil, ""), http.StatusConflict, "conflict", "")
	if m, _ := er.Error.Details["missing"].([]any); len(m) != 1 || m[0] != "rkn" {
		t.Fatalf("missing = %v, want [rkn]", er.Error.Details["missing"])
	}
	expect(t, e.do(c, http.MethodPost, path+"/rkn-check", map[string]any{"result": "confirmed"}, ""), http.StatusOK, "rkn-check")
	var ch struct {
		Status  string `json:"status"`
		History []struct {
			Action string `json:"action"`
		} `json:"moderation_history"`
	}
	r := e.do(c, http.MethodPost, path+"/approve", nil, "")
	expect(t, r, http.StatusOK, "approve")
	r.json(t, &ch)
	if ch.Status != "active" || len(ch.History) != 2 || ch.History[1].Action != "approved" {
		t.Fatalf("after approve: %+v", ch)
	}
	errorOf(t, e.do(c, http.MethodPost, path+"/approve", nil, ""), http.StatusConflict, "conflict", "")
}

// Этап выплат: юрлицу Рокет Ворк не нужен — 409 contractor_not_required, выплата переводом.
func TestRocketworkNotRequiredForLegal(t *testing.T) {
	e := newEnv(t)
	tx, done := e.st.Begin(context.Background())
	tx.D.Orgs[seed.OrgBlogger].LegalType = "legal_entity"
	done()
	c := e.client(blg)
	errorOf(t, e.do(c, http.MethodPost, "/blogger/rocketwork/invite", nil, ""), http.StatusConflict, "conflict", "contractor_not_required")
	errorOf(t, e.do(c, http.MethodPost, "/blogger/rocketwork/refresh", nil, ""), http.StatusConflict, "conflict", "contractor_not_required")
	var page struct {
		Items []struct {
			Method string `json:"method"`
		} `json:"items"`
	}
	e.do(c, http.MethodGet, "/blogger/payouts", nil, "").json(t, &page)
	if len(page.Items) == 0 || page.Items[0].Method != "bank_transfer" {
		t.Fatalf("payouts %+v", page.Items)
	}
}
