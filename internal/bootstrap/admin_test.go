package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	accountdomain "github.com/leha-creator/maxim/internal/account/domain"
)

func TestAdminGrant(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	role := accountdomain.AdminModerator
	res, err := a.mods.account.SetAdminRole(ctx, " New.Admin@Example.ru ", &role)
	if err != nil || !res.Created || !res.Changed {
		t.Fatalf("grant: %+v %v", res, err)
	}
	again, err := a.mods.account.SetAdminRole(ctx, "new.admin@example.ru", &role)
	if err != nil || again.Created || again.Changed || again.UserID != res.UserID {
		t.Fatalf("repeat grant: %+v %v", again, err)
	}
	var got *string
	if err := a.db.Pool.QueryRow(ctx, `SELECT admin_role FROM "user" WHERE id = $1`, res.UserID).Scan(&got); err != nil || got == nil || *got != "moderator" {
		t.Fatalf("role %v %v", got, err)
	}
	if res, err := a.mods.account.SetAdminRole(ctx, "new.admin@example.ru", nil); err != nil || !res.Changed {
		t.Fatalf("revoke: %+v %v", res, err)
	}
	if err := a.db.Pool.QueryRow(ctx, `SELECT admin_role FROM "user" WHERE id = $1`, res.UserID).Scan(&got); err != nil || got != nil {
		t.Fatalf("role after revoke %v %v", got, err)
	}
	var n int
	if err := a.db.Pool.QueryRow(ctx, `SELECT count(*) FROM audit_log WHERE action = 'user.admin_role_changed' AND actor_type = 'system'
		AND data::text NOT LIKE '%new.admin@example.ru%'`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("audit entries %d %v", n, err)
	}
	for _, args := range [][]string{{}, {"grant", "--email", "x@example.ru"}, {"grant", "--email", "x@example.ru", "--role", "owner"}, {"revoke"}, {"bogus"}} {
		if err := runAdmin(ctx, testConfig(), a.mods.in.log, args, &bytes.Buffer{}); !errors.Is(err, errUsage) {
			t.Errorf("%v: %v", args, err)
		}
	}
}

// TestCatalogOperationsImplemented — операции этапа «Каталог» больше не отвечают 501.
func TestCatalogOperationsImplemented(t *testing.T) {
	a := newTestApp(t)
	c := a.client()
	c.login("anyone@example.ru")
	id := "0190a000-0000-7000-8000-000000000999"
	for _, op := range []struct{ method, path string }{
		{http.MethodGet, "/placement-formats"},
		{http.MethodGet, "/catalog/channels"},
		{http.MethodGet, "/catalog/channels/" + id},
		{http.MethodGet, "/admin/channels"},
		{http.MethodGet, "/admin/channels/" + id},
		{http.MethodPost, "/admin/channels/" + id + "/approve"},
	} {
		if r := c.do(op.method, op.path, nil); r.status == http.StatusNotImplemented {
			t.Errorf("%s %s: 501", op.method, op.path)
		}
	}
}
