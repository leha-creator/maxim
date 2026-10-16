package outbound_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/leha-creator/maxim/internal/platform/clock"
	"github.com/leha-creator/maxim/internal/platform/dbtest"
	"github.com/leha-creator/maxim/internal/platform/job"
	"github.com/leha-creator/maxim/internal/platform/logx"
	"github.com/leha-creator/maxim/internal/platform/outbound"
)

func TestMain(m *testing.M) { os.Exit(dbtest.Main(m)) }

var start = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

type alerts struct {
	mu      sync.Mutex
	reasons []string
}

func (a *alerts) RaiseError(_ context.Context, reason, _, _ string, _ map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.reasons = append(a.reasons, reason)
}

type env struct {
	d      *dbtest.DB
	clk    *clock.Manual
	client *outbound.Client
	sleeps []time.Duration
	alerts *alerts
	reg    *prometheus.Registry
}

// newEnv поднимает сервер, отвечающий по очереди ответами responses (последний повторяется).
func newEnv(t *testing.T, handler http.HandlerFunc) (*env, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	e := &env{d: dbtest.New(t), clk: clock.NewManual(start), alerts: &alerts{}, reg: prometheus.NewRegistry()}
	e.client = outbound.New(e.d.Pool, e.clk, logx.Discard(), outbound.Options{
		System:  outbound.SystemMAX,
		Alerter: e.alerts,
		Metrics: outbound.NewMetrics(e.reg),
		Sleep: func(_ context.Context, d time.Duration) error {
			e.sleeps = append(e.sleeps, d)
			return nil
		},
	})
	return e, srv
}

func sequence(statuses ...int) (http.HandlerFunc, *atomic.Int32) {
	var n atomic.Int32
	return func(w http.ResponseWriter, r *http.Request) {
		i := int(n.Add(1)) - 1
		status := statuses[min(i, len(statuses)-1)]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"ok":true,"token":"server-secret"}`))
	}, &n
}

type stored struct {
	Attempt int
	Status  *int
	Request map[string]any
	Body    map[string]any
}

func (e *env) requests(t *testing.T) []stored {
	t.Helper()
	rows, err := e.d.Pool.Query(context.Background(),
		`SELECT attempt, response_status, request, response_body FROM outbound_request ORDER BY attempt`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []stored
	for rows.Next() {
		var s stored
		var req, body []byte
		if err := rows.Scan(&s.Attempt, &s.Status, &req, &body); err != nil {
			t.Fatal(err)
		}
		_ = json.Unmarshal(req, &s.Request)
		_ = json.Unmarshal(body, &s.Body)
		out = append(out, s)
	}
	return out
}

func get(url string) outbound.Call {
	return outbound.Call{Operation: "GetMe", Method: http.MethodGet, URL: url + "/me?access_token=abc&x=1",
		Header: http.Header{"Authorization": {"bot-token"}}}
}

func TestRetriesThenSuccess(t *testing.T) {
	h, _ := sequence(503, 503, 200)
	e, srv := newEnv(t, h)
	resp, err := e.client.Do(context.Background(), get(srv.URL))
	if err != nil || resp.Status != 200 {
		t.Fatalf("resp %d err %v", resp.Status, err)
	}
	reqs := e.requests(t)
	if len(reqs) != 3 {
		t.Fatalf("%d outbound_request rows, want 3", len(reqs))
	}
	if len(e.sleeps) != 2 || e.sleeps[0] != time.Second || e.sleeps[1] != 2*time.Second {
		t.Fatalf("sleeps %v", e.sleeps)
	}
	// Секреты не сохраняются (05 §1).
	headers, _ := reqs[0].Request["headers"].(map[string]any)
	if _, ok := headers["Authorization"]; ok {
		t.Fatal("Authorization header stored")
	}
	if url, _ := reqs[0].Request["url"].(string); strings.Contains(url, "abc") {
		t.Fatalf("access_token stored in url %s", url)
	}
	if reqs[2].Body["token"] != "[REDACTED]" {
		t.Fatalf("response token not redacted: %v", reqs[2].Body)
	}
	if v := counter(t, e.reg, "ok"); v != 1 {
		t.Fatalf("ok counter %v", v)
	}
}

// counter — значение maxim_outbound_requests_total с исходом outcome (сумма по операциям).
func counter(t *testing.T, reg *prometheus.Registry, outcome string) float64 {
	t.Helper()
	mfs, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	total := 0.0
	for _, mf := range mfs {
		if mf.GetName() != "maxim_outbound_requests_total" {
			continue
		}
		for _, m := range mf.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "outcome" && l.GetValue() == outcome {
					total += m.GetCounter().GetValue()
				}
			}
		}
	}
	return total
}

func TestRejectedIsPermanent(t *testing.T) {
	h, calls := sequence(400)
	e, srv := newEnv(t, h)
	_, err := e.client.Do(context.Background(), get(srv.URL))
	var rejected *outbound.RejectedError
	if !job.IsPermanent(err) || !errors.As(err, &rejected) || rejected.Status != 400 {
		t.Fatalf("err %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("%d calls for 400", calls.Load())
	}
	if len(e.alerts.reasons) != 1 || e.alerts.reasons[0] != "outbound_failed:max:GetMe" {
		t.Fatalf("alerts %v", e.alerts.reasons)
	}
}

func TestRetryAfter(t *testing.T) {
	var n atomic.Int32
	e, srv := newEnv(t, func(w http.ResponseWriter, r *http.Request) {
		if n.Add(1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(429)
			return
		}
		w.WriteHeader(200)
	})
	if _, err := e.client.Do(context.Background(), get(srv.URL)); err != nil {
		t.Fatal(err)
	}
	if len(e.sleeps) != 1 || e.sleeps[0] != 2*time.Second {
		t.Fatalf("sleeps %v", e.sleeps)
	}
}

func TestLongRetryAfterDefersJob(t *testing.T) {
	e, srv := newEnv(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "600")
		w.WriteHeader(429)
	})
	_, err := e.client.Do(context.Background(), get(srv.URL))
	if after, ok := job.RetryAfter(err); !ok || after != 600*time.Second {
		t.Fatalf("err %v after %v", err, after)
	}
}

func TestBodyRejection(t *testing.T) {
	e, srv := newEnv(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"message":"not found"}`))
	})
	call := get(srv.URL)
	call.ValidateBody = func(r outbound.Response) error {
		var b struct{ Success bool }
		_ = json.Unmarshal(r.Body, &b)
		if !b.Success {
			return errors.New("success=false")
		}
		return nil
	}
	if _, err := e.client.Do(context.Background(), call); !job.IsPermanent(err) {
		t.Fatalf("err %v", err)
	}
}

func TestTimeoutOnCreateRecovered(t *testing.T) {
	var calls atomic.Int32
	e, srv := newEnv(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		time.Sleep(300 * time.Millisecond)
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	call := outbound.Call{Operation: "SendMessage", Method: http.MethodPost, URL: srv.URL, Body: map[string]string{"text": "x"},
		Create: true, CheckExisting: func(context.Context) (bool, error) { return true, nil }}
	// Таймаут попытки короче ответа сервера: клиент с собственным таймаутом.
	e.client = outbound.New(e.d.Pool, e.clk, logx.Discard(), outbound.Options{
		System: outbound.SystemMAX, HTTP: &http.Client{Timeout: 50 * time.Millisecond},
		Sleep: func(context.Context, time.Duration) error { return nil },
	})
	resp, err := e.client.Do(ctx, call)
	if err != nil || !resp.Recovered {
		t.Fatalf("resp %+v err %v", resp, err)
	}
	if calls.Load() != 1 {
		t.Fatalf("%d calls, want 1", calls.Load())
	}
}

func TestCircuitBreaker(t *testing.T) {
	h, calls := sequence(500)
	e, srv := newEnv(t, h)
	ctx := context.Background()
	// 3 вызова по 4 попытки: на 10-й ошибке предохранитель открывается.
	var err error
	for range 3 {
		_, err = e.client.Do(ctx, get(srv.URL))
	}
	if after, ok := job.RetryAfter(err); !ok || after != 60*time.Second || !errors.Is(err, outbound.ErrCircuitOpen) {
		t.Fatalf("err %v", err)
	}
	if calls.Load() != 10 {
		t.Fatalf("%d calls before opening, want 10", calls.Load())
	}
	if len(e.alerts.reasons) == 0 || e.alerts.reasons[len(e.alerts.reasons)-1] != "circuit_open:max" {
		t.Fatalf("alerts %v", e.alerts.reasons)
	}
	// Пока открыт — без запроса.
	_, err = e.client.Do(ctx, get(srv.URL))
	if !errors.Is(err, outbound.ErrCircuitOpen) || calls.Load() != 10 {
		t.Fatalf("call while open: err %v calls %d", err, calls.Load())
	}
	// Через 60 с запрос проходит.
	e.clk.Advance(61 * time.Second)
	_, _ = e.client.Do(ctx, get(srv.URL))
	if calls.Load() != 11 {
		t.Fatalf("calls after open period %d, want 11", calls.Load())
	}
}
