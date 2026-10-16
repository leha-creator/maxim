package logx

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestMask(t *testing.T) {
	cases := []struct {
		name string
		fn   func(string) string
		in   string
		want string
	}{
		{"email", MaskEmail, "ivan@example.ru", "i***@example.ru"},
		{"email cyrillic", MaskEmail, "иван@пример.рф", "и***@пример.рф"},
		{"email without at", MaskEmail, "broken", "***"},
		{"inn 10", MaskINN, "7712345678", "77****5678"},
		{"inn 12", MaskINN, "771234561234", "77******1234"},
		{"short", MaskINN, "123", "***"},
		{"phone", MaskPhone, "+79991234567", "+7******4567"},
		{"name", MaskName, "Иванов Иван Иванович", "И*** И*** И***"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.fn(c.in); got != c.want {
				t.Fatalf("got %q, want %q", got, c.want)
			}
		})
	}
}

func TestParseLevel(t *testing.T) {
	if l, ok := ParseLevel("", slog.LevelInfo); !ok || l != slog.LevelInfo {
		t.Fatalf("empty: %v %v", l, ok)
	}
	if l, ok := ParseLevel("WARN", slog.LevelInfo); !ok || l != slog.LevelWarn {
		t.Fatalf("WARN: %v %v", l, ok)
	}
	if _, ok := ParseLevel("verbose", slog.LevelInfo); ok {
		t.Fatal("unknown level accepted")
	}
}

func TestContextFields(t *testing.T) {
	var buf bytes.Buffer
	log := New(&buf, slog.LevelDebug, "api")
	ctx := WithRequestID(context.Background(), "req-1")
	ctx = With(ctx, KeyUserID, "u-1")
	ctx = With(ctx, KeyUserID, "u-2", KeyOrganizationID, "o-1")
	log.InfoContext(ctx, "user logged in", KeyOp, "Account.Login")

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("not json: %v: %s", err, buf.String())
	}
	want := map[string]any{
		"msg": "user logged in", "component": "api", "op": "Account.Login",
		"request_id": "req-1", "user_id": "u-2", "organization_id": "o-1",
	}
	for k, v := range want {
		if rec[k] != v {
			t.Errorf("%s = %v, want %v", k, rec[k], v)
		}
	}
	if bytes.Count(buf.Bytes(), []byte(`"user_id"`)) != 1 {
		t.Errorf("user_id duplicated: %s", buf.String())
	}
}
