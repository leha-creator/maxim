package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"maxim/mockapi/internal/api"
	"maxim/mockapi/internal/clock"
	"maxim/mockapi/internal/handlers"
	"maxim/mockapi/internal/logx"
	"maxim/mockapi/internal/seed"
	"maxim/mockapi/internal/server"
	"maxim/mockapi/internal/store"
)

func init() {
	openapi3filter.RegisterBodyDecoder("application/pdf", openapi3filter.FileBodyDecoder)
}

// env — тестовый сервер, его хранилище и маршрутизатор спецификации для проверки ответов.
type env struct {
	t      *testing.T
	srv    *httptest.Server
	st     *store.Store
	spec   *openapi3.T
	router routers.Router
}

func newEnv(t *testing.T) *env {
	t.Helper()
	log := logx.New(os.Stderr, "warn")
	st := store.New(clock.New(), log, seed.Load)
	h, err := server.New(log, st, handlers.Options{})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	spec, err := api.GetSwagger()
	if err != nil {
		t.Fatal(err)
	}
	spec.Servers = openapi3.Servers{{URL: srv.URL + server.BaseURL}}
	router, err := gorillamux.NewRouter(spec)
	if err != nil {
		t.Fatal(err)
	}
	return &env{t: t, srv: srv, st: st, spec: spec, router: router}
}

// client входит под пользователем и возвращает клиент с cookie сессии.
// Пустой email — клиент без входа.
func (e *env) client(email string) *http.Client {
	e.t.Helper()
	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}
	if email == "" {
		return c
	}
	resp := e.do(c, http.MethodPost, "/auth/login", map[string]string{"email": email, "code": "000000"}, "")
	if resp.status != http.StatusOK {
		e.t.Fatalf("login %s: status %d: %s", email, resp.status, resp.body)
	}
	return c
}

type response struct {
	status int
	body   []byte
	header http.Header
}

func (r response) json(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		t.Fatalf("decode %s: %v", r.body, err)
	}
}

// do выполняет запрос к API и проверяет ответ по спецификации.
func (e *env) do(c *http.Client, method, path string, body any, contentType string) response {
	e.t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case []byte:
		rd = bytes.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
		if contentType == "" {
			contentType = "application/json"
		}
	}
	req, err := http.NewRequest(method, e.srv.URL+server.BaseURL+path, rd)
	if err != nil {
		e.t.Fatal(err)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	out := response{status: resp.StatusCode, body: raw, header: resp.Header}
	if err := e.validate(req, out); err != nil {
		e.t.Errorf("%s %s: response %d does not match spec: %v\nrequest_id=%s body=%s",
			req.Method, req.URL.Path, out.status, err, out.header.Get("X-Request-Id"), truncate(out.body))
	}
	return out
}

// validate проверяет, что статус объявлен в спецификации и тело ответа совпадает со схемой.
func (e *env) validate(req *http.Request, r response) error {
	route, params, err := e.router.FindRoute(req)
	if err != nil {
		return fmt.Errorf("route not in spec: %w", err)
	}
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route},
		Status:                 r.status,
		Header:                 r.header,
		Body:                   io.NopCloser(bytes.NewReader(r.body)),
		Options:                &openapi3filter.Options{IncludeResponseStatus: true, MultiError: true},
	}
	return openapi3filter.ValidateResponse(context.Background(), in)
}

// TestValidatorRejectsMismatch проверяет саму проверку: ответ не по схеме и
// необъявленный статус должны отклоняться, иначе остальные тесты ничего не доказывают.
func TestValidatorRejectsMismatch(t *testing.T) {
	e := newEnv(t)
	req, _ := http.NewRequest(http.MethodGet, e.srv.URL+server.BaseURL+"/me", nil)
	h := http.Header{"Content-Type": {"application/json"}}
	if err := e.validate(req, response{status: 200, body: []byte(`{"user":{}}`), header: h}); err == nil {
		t.Error("body without required fields passed validation")
	}
	if err := e.validate(req, response{status: 418, body: []byte(`{}`), header: h}); err == nil {
		t.Error("undeclared status passed validation")
	}
}

func truncate(b []byte) string {
	if len(b) > 600 {
		return string(b[:600]) + "…"
	}
	return string(b)
}
