package server_test

import (
	"net/http"
	"time"

	"maxim/mockapi/internal/seed"
	"maxim/mockapi/internal/store"
)

// post выполняет POST от имени пользователя и требует 2xx.
func (e *env) post(email, path string, body any, header map[string]string) response {
	e.t.Helper()
	r := e.doH(e.client(email), http.MethodPost, path, body, "", header)
	if r.status < 200 || r.status > 299 {
		e.t.Fatalf("POST %s: %d %s", path, r.status, r.body)
	}
	return r
}

func idem(key string) map[string]string { return map[string]string{"Idempotency-Key": key} }

// prevPeriod — прошлый месяц по Москве: за него в начальных данных есть отчёт ОРД.
func prevPeriod() map[string]string {
	return map[string]string{"period": time.Now().In(store.MSK).AddDate(0, -1, 0).Format("2006-01")}
}

// bloggerDispute — администратор открывает спор по размещению блогера blg@example.test.
func bloggerDispute(e *env) (map[string]string, any) {
	e.post(adm, "/admin/placements/"+seed.PlacementRemoved.String()+"/open-dispute", map[string]any{"reason": "Проверка"}, nil)
	return id(seed.PlacementRemoved), noBodyMarker
}

// pendingApproval — ручная проводка выше порога создаёт запрос подтверждения от adm@example.test.
func pendingApproval(e *env) (map[string]string, any) {
	var a struct {
		ID string `json:"id"`
	}
	r := e.doH(e.client(adm), http.MethodPost, "/admin/ledger/manual", map[string]any{
		"debit": map[string]any{"kind": "bank"}, "credit": map[string]any{"kind": "platform_revenue"},
		"amount": 15_000_000, "reason": "Крупная корректировка",
	}, "", idem("approval-1"))
	if r.status != http.StatusAccepted {
		e.t.Fatalf("manual over threshold: %d %s", r.status, r.body)
	}
	r.json(e.t, &a)
	return map[string]string{"id": a.ID}, noBodyMarker
}
