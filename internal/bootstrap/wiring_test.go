package bootstrap

import (
	"net/http"
	"slices"
	"testing"

	"github.com/leha-creator/maxim/internal/integration/max/maxfake"
	"github.com/leha-creator/maxim/internal/platform/inbound"
)

func TestAssembledApp(t *testing.T) {
	a := newTestApp(t)
	resp, err := http.Get(a.srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("readyz: %d", resp.StatusCode)
	}
	for _, kind := range []string{"send_login_code", "process_inbound_event", "send_alert", "check_stuck_objects", "cleanup",
		"refresh_channel_info", "delete_max_message", "leave_max_chat", "reply_bot_dialog", "release_unclaimed_channels",
		"schedule_channel_metrics", "refresh_channel_metrics",
		"build_media_derivative", "publish_media_derivative", "purge_media"} {
		if !slices.Contains(a.worker.Kinds(), kind) {
			t.Fatalf("worker kinds %v have no %s", a.worker.Kinds(), kind)
		}
	}
	resp, err = http.Get(a.srv.URL + "/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("openapi.json outside production: %d", resp.StatusCode)
	}
}

func TestMAXEventHandlersRegistered(t *testing.T) {
	a := newTestApp(t)
	for _, ev := range []string{"bot_added", "bot_removed", "bot_admin_permissions_changed", "message_created",
		"message_edited", "message_removed", "chat_title_changed"} {
		if !a.mods.platform.dispatcher.Handles(inbound.SourceMAX, ev) {
			t.Errorf("no handler for MAX event %s", ev)
		}
	}
}

func TestSubscribeMAX(t *testing.T) {
	fake := maxfake.New()
	ctx := t.Context()
	url := "https://api.maxim.example/api/v1/webhooks/max/secret"
	for range 2 {
		if err := subscribeMAX(ctx, fake, url, "s"); err != nil {
			t.Fatal(err)
		}
	}
	subs, _ := fake.ListSubscriptions(ctx)
	if len(subs) != 1 || subs[0].URL != url || !slices.Contains(subs[0].UpdateTypes, "bot_admin_permissions_changed") {
		t.Fatalf("subscriptions %+v", subs)
	}
	// Подписка без событий о публикациях обновляется (R-17); полная — нет.
	old := []string{"bot_added", "bot_removed", "bot_admin_permissions_changed", "message_created", "chat_title_changed"}
	if err := fake.Subscribe(ctx, url, old, "s"); err != nil {
		t.Fatal(err)
	}
	if err := subscribeMAX(ctx, fake, url, "s"); err != nil {
		t.Fatal(err)
	}
	subs, _ = fake.ListSubscriptions(ctx)
	if len(subs) != 1 || !slices.Contains(subs[0].UpdateTypes, "message_removed") || !slices.Contains(subs[0].UpdateTypes, "message_edited") {
		t.Fatalf("subscription not updated: %+v", subs)
	}
	cfg := Config{MAX: MAXConfig{WebhookURL: "https://api.maxim.example/api/v1/webhooks/max/", WebhookPathSecret: "secret"}}
	if got := maxWebhookURL(cfg); got != url {
		t.Fatalf("webhook url %q", got)
	}
}
