package bootstrap

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/leha-creator/maxim/internal/bootstrap/bridge"
)

func TestPlacementConfig(t *testing.T) {
	cfg, err := LoadConfig(validAPIEnv(), ModeAPI)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Placement
	if p.AcceptTimeout != 24*time.Hour || p.ScheduleTimeout != 24*time.Hour || p.WindowGuard != time.Hour ||
		p.MinLead != time.Hour || p.SlotStep != 5*time.Minute || p.ReeditGuard != 24*time.Hour || p.CartMaxItems != 50 ||
		p.StuckScheduledAge != 35*time.Minute {
		t.Fatalf("placement defaults: %+v", p)
	}
	for _, tc := range []struct {
		key, value, field string
		ok                bool
	}{
		{"PLACEMENT_SLOT_STEP", "7m", "PLACEMENT_SLOT_STEP", false},
		{"PLACEMENT_SLOT_STEP", "90s", "PLACEMENT_SLOT_STEP", false},
		{"PLACEMENT_SLOT_STEP", "1m", "", true},
		{"PLACEMENT_SLOT_STEP", "60m", "", true},
		{"PLACEMENT_SLOT_STEP", "0s", "PLACEMENT_SLOT_STEP", false},
		{"PLACEMENT_WINDOW_GUARD", "24h", "", true},
		{"PLACEMENT_WINDOW_GUARD", "25h", "PLACEMENT_WINDOW_GUARD", false},
		{"PLACEMENT_ACCEPT_TIMEOUT", "0s", "PLACEMENT_ACCEPT_TIMEOUT", false},
		{"PLACEMENT_REEDIT_GUARD", "-1h", "PLACEMENT_REEDIT_GUARD", false},
		{"CART_MAX_ITEMS", "0", "CART_MAX_ITEMS", false},
		{"CART_MAX_ITEMS", "200", "", true},
		{"CART_MAX_ITEMS", "201", "CART_MAX_ITEMS", false},
	} {
		env := validAPIEnv()
		env[tc.key] = tc.value
		_, err := LoadConfig(env, ModeAPI)
		switch {
		case tc.ok && err != nil:
			t.Errorf("%s=%s: %v", tc.key, tc.value, err)
		case !tc.ok && (err == nil || !slices.ContainsFunc(configFields(t, err), func(f string) bool { return strings.HasPrefix(f, tc.field) })):
			t.Errorf("%s=%s: %v", tc.key, tc.value, err)
		}
	}
	if _, err := LoadConfig(map[string]string{"DATABASE_URL": "postgres://x"}, ModeDev); err != nil {
		t.Errorf("dev mode needs only the database: %v", err)
	}
	if !strings.Contains(cfg.LogValue().String(), "placement_slot_step") {
		t.Error("placement settings are not logged")
	}
}

func TestPlacementWiring(t *testing.T) {
	a := newTestApp(t)
	if !slices.Contains(a.worker.Kinds(), "placement_timer") {
		t.Fatalf("worker kinds %v have no placement_timer", a.worker.Kinds())
	}
	if _, ok := a.mods.post.Listener.(bridge.VersionListener); !ok {
		t.Fatalf("post version listener %T", a.mods.post.Listener)
	}
	if _, ok := a.mods.channel.Availability.(bridge.ChannelPlacements); !ok {
		t.Fatalf("channel availability listener %T", a.mods.channel.Availability)
	}
	if _, ok := a.mods.channel.Placements.(bridge.ChannelPlacements); !ok {
		t.Fatalf("channel placement schedule %T", a.mods.channel.Placements)
	}
	if _, ok := a.mods.channel.Cart.(bridge.ChannelPlacements); !ok {
		t.Fatalf("channel cart lookup %T", a.mods.channel.Cart)
	}
	if _, ok := a.mods.placement.Holds.(bridge.UnlimitedHolds); !ok {
		t.Fatalf("holds %T", a.mods.placement.Holds)
	}

	// Операции этапа отвечают не 501; приёмка и претензия — 501 до своего этапа.
	c := a.client()
	c.login("anyone-orders@example.ru")
	id := "0190a000-0000-7000-8000-000000000999"
	for _, op := range []struct{ method, path string }{
		{http.MethodGet, "/cart"}, {http.MethodGet, "/orders"}, {http.MethodGet, "/orders/" + id},
		{http.MethodPost, "/orders/" + id + "/cancel"}, {http.MethodPost, "/orders/" + id + "/post-versions"},
		{http.MethodPost, "/placements/" + id + "/cancel"}, {http.MethodGet, "/blogger/placements"},
		{http.MethodGet, "/blogger/placements/" + id}, {http.MethodPost, "/blogger/placements/" + id + "/accept"},
		{http.MethodPost, "/campaigns/" + id + "/checkout"},
	} {
		if r := c.do(op.method, op.path, nil); r.status == http.StatusNotImplemented {
			t.Errorf("%s %s: 501", op.method, op.path)
		}
	}
	if r := c.do(http.MethodPost, "/placements/"+id+"/accept", nil); r.status != http.StatusNotImplemented {
		t.Errorf("acceptPlacement: %d", r.status)
	}
}

func TestDevPostVersion(t *testing.T) {
	a := newTestApp(t)
	ctx := context.Background()
	prod := testConfig()
	prod.Env = EnvProduction
	if err := runDev(ctx, prod, a.mods.in.log, []string{"post-version", "--order", "1001", "approve"}, &bytes.Buffer{}); !errors.Is(err, errDevForbidden) {
		t.Fatalf("production: %v", err)
	}
	for _, args := range [][]string{{}, {"other"}, {"post-version", "approve"}, {"post-version", "--order", "1001", "publish"}} {
		if err := runDev(ctx, testConfig(), a.mods.in.log, args, &bytes.Buffer{}); !errors.Is(err, errUsage) {
			t.Errorf("%v: %v", args, err)
		}
	}

	adv := a.advertiserClient("adv-dev@example.ru")
	blg := a.bloggerClient("blg-dev@example.ru")
	ch := a.activeChannel(blg, "Канал", 1_000_000)
	campaign := a.campaignWithPost(adv, "Кампания")
	adv.do(http.MethodPost, "/cart/items", map[string]any{"channel_id": ch, "format_id": "f_1_24"})
	var order orderView
	adv.do(http.MethodPost, "/campaigns/"+campaign+"/checkout", nil).json(t, &order)

	cfg := testConfig()
	cfg.DatabaseURL = a.db.AppURL
	var out bytes.Buffer
	if err := runDev(ctx, cfg, a.mods.in.log, []string{"post-version", "--order", "1001", "approve"}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "order 1001 status marking") {
		t.Fatalf("output %q", out.String())
	}
	if !strings.Contains(a.logs.String(), `"msg":"dev command used"`) {
		t.Fatal("dev command not logged")
	}
}
