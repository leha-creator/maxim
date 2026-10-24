package server_test

import (
	"net/http"
	"testing"

	"maxim/mockapi/internal/seed"
)

type errBody struct {
	Error struct {
		Code    string         `json:"code"`
		Details map[string]any `json:"details"`
	} `json:"error"`
}

// TestAdminRoles — moderator не выполняет операции senior, но видит общие разделы (R-1).
func TestAdminRoles(t *testing.T) {
	e := newEnv(t)
	m := e.client(mod)
	for _, path := range []string{"/admin/payouts", "/admin/ledger/summary", "/admin/audit-log", "/admin/approvals"} {
		r := e.do(m, http.MethodGet, path, nil, "")
		var b errBody
		r.json(t, &b)
		if r.status != http.StatusForbidden || b.Error.Details["reason"] != "admin_required" {
			t.Errorf("moderator GET %s: %d %v, want 403 admin_required", path, r.status, b.Error.Details)
		}
	}
	r := e.do(m, http.MethodPost, "/admin/placements/"+seed.PlacementScheduled.String()+"/cancel", map[string]any{"reason": "x"}, "")
	if r.status != http.StatusForbidden {
		t.Errorf("moderator cancel: %d, want 403", r.status)
	}
	if r := e.do(m, http.MethodGet, "/admin/disputes", nil, ""); r.status != http.StatusOK {
		t.Errorf("moderator disputes: %d, want 200", r.status)
	}
	// ИНН у moderator — с маской, у senior — полный (R-14).
	var org struct {
		Inn string `json:"inn"`
	}
	e.do(m, http.MethodGet, "/admin/organizations/"+seed.OrgAdvertiser.String(), nil, "").json(t, &org)
	if org.Inn != "77****0001" {
		t.Errorf("moderator inn = %q, want masked", org.Inn)
	}
	e.do(e.client(adm), http.MethodGet, "/admin/organizations/"+seed.OrgAdvertiser.String(), nil, "").json(t, &org)
	if org.Inn != "7700000001" {
		t.Errorf("senior inn = %q, want full", org.Inn)
	}
}

// TestSecondApproval — выше порога 202 и запрос; тот же администратор — 409 same_admin, другой — исполнено.
func TestSecondApproval(t *testing.T) {
	e := newEnv(t)
	p, _ := pendingApproval(e)
	path := "/admin/approvals/" + p["id"] + "/approve"
	r := e.do(e.client(adm), http.MethodPost, path, nil, "")
	var b errBody
	r.json(t, &b)
	if r.status != http.StatusConflict || b.Error.Details["reason"] != "same_admin" {
		t.Fatalf("same admin approve: %d %v", r.status, b.Error.Details)
	}
	var a struct {
		Status   string  `json:"status"`
		ResultID *string `json:"result_id"`
	}
	r = e.do(e.client(adm2), http.MethodPost, path, nil, "")
	r.json(t, &a)
	if r.status != http.StatusOK || a.Status != "approved" || a.ResultID == nil {
		t.Fatalf("second admin approve: %d %s", r.status, r.body)
	}
	if r := e.do(e.client(adm2), http.MethodPost, path, nil, ""); r.status != http.StatusConflict {
		t.Errorf("repeat approve: %d, want 409", r.status)
	}
	// Повтор исходного запроса с тем же ключом — тот же запрос, без новой проводки.
	var entries struct {
		Total *int `json:"total"`
	}
	e.do(e.client(adm), http.MethodGet, "/admin/ledger/entries?kind=manual", nil, "").json(t, &entries)
	if entries.Total == nil || *entries.Total != 1 {
		t.Errorf("manual entries = %v, want 1", entries.Total)
	}
}

// TestDisputeMessages — сообщение администратора блогеру не видно рекламодателю; после решения писать нельзя.
func TestDisputeMessages(t *testing.T) {
	e := newEnv(t)
	base := "/admin/disputes/" + seed.DisputeOpen.String()
	e.post(adm, base+"/messages", map[string]any{"to": "blogger", "text": "Вопрос блогеру"}, nil)
	e.post(adm, base+"/messages", map[string]any{"to": "advertiser", "text": "Вопрос рекламодателю"}, nil)
	var page struct {
		Items []struct {
			Text string `json:"text"`
		} `json:"items"`
	}
	e.do(e.client(adv), http.MethodGet, "/placements/"+seed.PlacementDisputed.String()+"/dispute/messages", nil, "").json(t, &page)
	if len(page.Items) != 1 || page.Items[0].Text != "Вопрос рекламодателю" {
		t.Fatalf("advertiser sees %v", page.Items)
	}
	e.post(adm, base+"/resolve", map[string]any{"resolution": "full_payout", "comment": "Нарушение не подтвердилось"}, nil)
	r := e.do(e.client(adv), http.MethodPost, "/placements/"+seed.PlacementDisputed.String()+"/dispute/messages", map[string]any{"text": "Ещё"}, "")
	var b errBody
	r.json(t, &b)
	if r.status != http.StatusConflict || b.Error.Details["reason"] != "dispute_resolved" {
		t.Fatalf("message after resolve: %d %v", r.status, b.Error.Details)
	}
}

// TestDisputePreview — доля 7500 делит удержание как 03 §7 и ничего не меняет.
func TestDisputePreview(t *testing.T) {
	e := newEnv(t)
	var pv struct {
		Total  int64 `json:"total_amount"`
		Blog   int64 `json:"blogger_amount"`
		Fee    int64 `json:"fee_amount"`
		Refund int64 `json:"refund_amount"`
	}
	e.do(e.client(adm), http.MethodGet, "/admin/disputes/"+seed.DisputeOpen.String()+"/preview?resolution=proportional&share=7500", nil, "").json(t, &pv)
	if pv.Blog+pv.Fee+pv.Refund != pv.Total || pv.Blog == 0 || pv.Refund == 0 {
		t.Fatalf("preview %+v", pv)
	}
	var d struct {
		Resolution *string `json:"resolution"`
	}
	e.do(e.client(adm), http.MethodGet, "/admin/disputes/"+seed.DisputeOpen.String(), nil, "").json(t, &d)
	if d.Resolution != nil {
		t.Fatal("preview resolved the dispute")
	}
}

// TestBlockedUser — заблокированный пользователь теряет сессию и не входит.
func TestBlockedUser(t *testing.T) {
	e := newEnv(t)
	c := e.client(blg)
	e.post(adm, "/admin/users/"+seed.UserBlogger.String()+"/block", map[string]any{"reason": "Проверка"}, nil)
	if r := e.do(c, http.MethodGet, "/me", nil, ""); r.status != http.StatusUnauthorized {
		t.Errorf("blocked session: %d, want 401", r.status)
	}
	r := e.do(&http.Client{}, http.MethodPost, "/auth/login", map[string]string{"email": blg, "code": "000000"}, "")
	if r.status != http.StatusForbidden {
		t.Errorf("blocked login: %d, want 403", r.status)
	}
	r = e.do(e.client(adm), http.MethodPost, "/admin/users/"+seed.UserAdmin2.String()+"/block", map[string]any{"reason": "x"}, "")
	if r.status != http.StatusConflict {
		t.Errorf("block admin: %d, want 409", r.status)
	}
}
