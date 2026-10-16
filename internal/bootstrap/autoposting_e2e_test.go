package bootstrap

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Текст поста сквозного теста: его не должно быть в журнале (07 §5.4).
const autopostText = "Секретный текст зимней распродажи"

// chatOf — идентификатор чата MAX канала.
func (a *testApp) chatOf(channelID string) int64 {
	a.t.Helper()
	var id int64
	if err := a.db.Pool.QueryRow(context.Background(), `SELECT max_chat_id FROM channel WHERE id = $1`, channelID).Scan(&id); err != nil {
		a.t.Fatal(err)
	}
	return id
}

// placementState — состояние и причина размещения.
func (a *testApp) placementState(pid string) (status, reason string) {
	a.t.Helper()
	var r *string
	if err := a.db.Pool.QueryRow(context.Background(), `SELECT status, status_reason FROM placement WHERE id = $1`, pid).Scan(&status, &r); err != nil {
		a.t.Fatal(err)
	}
	if r != nil {
		reason = *r
	}
	return status, reason
}

func (a *testApp) count(query string, args ...any) int {
	a.t.Helper()
	var n int
	if err := a.db.Pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		a.t.Fatal(err)
	}
	return n
}

// at — момент 20 октября по Москве.
func at(hh, mm, ss int) time.Time {
	return time.Date(2026, 10, 20, hh-3, mm, ss, 0, time.UTC)
}

// autopostOrder — заказ рекламодателя из каналов блогера; версия поста готова, блогер принял все
// размещения. Возвращает размещения по каналам.
func (a *testApp) autopostOrder(adv, blg *client, titles ...string) (channels, placements []string) {
	a.t.Helper()
	campaign := a.campaignWithPost(adv, "Зима "+titles[0])
	a.savePost(adv, campaign, autopostText)
	for _, title := range titles {
		ch := a.activeChannel(blg, title, 1_000_000)
		if r := adv.do(http.MethodPost, "/cart/items", map[string]any{"channel_id": ch, "format_id": "f_1_24"}); r.status != http.StatusCreated {
			a.t.Fatalf("add to cart: %d %s", r.status, r.body)
		}
		channels = append(channels, ch)
	}
	var order orderView
	adv.do(http.MethodPost, "/campaigns/"+campaign+"/checkout", nil).json(a.t, &order)
	a.readyVersion(order.CurrentPostVersion.ID)
	for _, ch := range channels {
		pid := a.placementOf(order.ID, ch)
		if r := blg.do(http.MethodPost, "/blogger/placements/"+pid+"/accept", nil); r.status != http.StatusOK {
			a.t.Fatalf("accept: %d %s", r.status, r.body)
		}
		placements = append(placements, pid)
	}
	return channels, placements
}

func (a *testApp) schedule(blg *client, pid string, body map[string]any) {
	a.t.Helper()
	if r := blg.do(http.MethodPost, "/blogger/placements/"+pid+"/schedule", body); r.status != http.StatusOK {
		a.t.Fatalf("schedule: %d %s", r.status, r.body)
	}
}

func messageCreated(chatID, sender int64, mid string, ts time.Time) string {
	return fmt.Sprintf(`{"update_type":"message_created","timestamp":%d,"message":{"sender":{"user_id":%d},`+
		`"recipient":{"chat_id":%d,"chat_type":"channel"},"timestamp":%d,"body":{"mid":%q,"text":"Свой пост"}}}`,
		ts.UnixMilli(), sender, chatID, ts.UnixMilli(), mid)
}

func (a *testApp) send(body string) {
	a.t.Helper()
	if code := a.webhook(testWebhookPath, testWebhookSecret, body); code != http.StatusOK {
		a.t.Fatalf("webhook: %d", code)
	}
}

func TestAutopostingEndToEnd(t *testing.T) {
	a := newTestApp(t)
	adv := a.advertiserClient("adv.autopost@example.ru")
	blg := a.bloggerClient("blg.autopost@example.ru")
	var advOrg struct {
		Name string `json:"name"`
		INN  string `json:"inn"`
	}
	adv.do(http.MethodGet, "/organizations/current", nil).json(t, &advOrg)
	channels, pids := a.autopostOrder(adv, blg, "Кухня", "Сад", "Дом")

	// Первый — «Опубликовать сейчас» в 11:30, второй — на 13:30, третий — на 12:30.
	a.clk.Set(at(11, 30, 0))
	a.schedule(blg, pids[0], map[string]any{"mode": "now"})
	a.schedule(blg, pids[1], map[string]any{"mode": "at", "publish_at": at(13, 30, 0).Format(time.RFC3339)})
	a.schedule(blg, pids[2], map[string]any{"mode": "at", "publish_at": at(12, 30, 0).Format(time.RFC3339)})
	a.clk.Set(at(11, 30, 4))
	a.settle()
	if st, _ := a.placementState(pids[0]); st != "published" {
		t.Fatalf("now: %s", st)
	}
	a.clk.Set(at(12, 30, 5))
	a.settle()
	a.clk.Set(at(13, 30, 5))
	a.settle()
	for i, pid := range pids {
		if st, _ := a.placementState(pid); st != "published" {
			t.Fatalf("placement %d: %s", i, st)
		}
	}
	// Точность 07 §2: публикация не позже 10 с после плана.
	if n := a.count(`SELECT count(*) FROM placement WHERE id = ANY($1::uuid[]) AND published_at - publish_at > interval '10 seconds'`,
		pids); n != 0 {
		t.Fatalf("%d placements published late", n)
	}
	posts := a.maxFake().Posts()
	if len(posts) != 3 || !strings.Contains(posts[0].Text, autopostText) || !strings.Contains(posts[0].Text, "erid: ") {
		t.Fatalf("posts %+v", posts)
	}

	// S-27: сообщение человека в первом канале после его топа (до 12:30) и во втором — во время топа.
	chat0, chat1 := a.chatOf(channels[0]), a.chatOf(channels[1])
	a.send(messageCreated(chat0, 42, "mid.h0", at(13, 0, 0))) // после топа первого
	a.send(messageCreated(chat1, 42, "mid.h1", at(14, 0, 0))) // во время топа второго
	a.settle()
	if st, _ := a.placementState(pids[0]); st != "published" {
		t.Fatalf("message after top: %s", st)
	}
	if st, reason := a.placementState(pids[1]); st != "disputed" || reason != "top_violated" {
		t.Fatalf("top violation: %s %s", st, reason)
	}

	// S-26: правка поста третьего канала через Webhook.
	edited := fmt.Sprintf(`{"update_type":"message_edited","timestamp":%d,"message":{"sender":{"user_id":42},`+
		`"recipient":{"chat_id":%d,"chat_type":"channel"},"timestamp":%d,"body":{"mid":%q,"text":"Другой текст"}}}`,
		at(15, 0, 0).UnixMilli(), a.chatOf(channels[2]), posts[1].At.UnixMilli(), posts[1].ID)
	a.send(edited)
	a.settle()
	if st, reason := a.placementState(pids[2]); st != "disputed" || reason != "content_modified" {
		t.Fatalf("content modified: %s %s", st, reason)
	}

	// Сверка без событий (Q-03): пост первого канала удалён, Webhook не пришёл.
	a.maxFake().RemoveExternally(posts[0].ID)
	a.clk.Set(at(16, 0, 0))
	if err := a.mods.placement.ReconcilePublications(context.Background()); err != nil {
		t.Fatal(err)
	}
	a.clk.Set(at(16, 5, 0))
	a.settle()
	if st, reason := a.placementState(pids[0]); st != "disputed" || reason != "removed_early" {
		t.Fatalf("reconciliation: %s %s", st, reason)
	}
	if a.count(`SELECT count(*) FROM violation WHERE placement_id = $1 AND source = 'reconciliation'`, pids[0]) != 1 {
		t.Fatal("reconciliation violation")
	}

	// Снятие по сроку: второй и третий в споре — посты удаляются, состояние не меняется.
	a.clk.Set(at(13, 30, 5).Add(24*time.Hour + 10*time.Second))
	a.settle()
	for _, pid := range pids[1:] {
		if st, _ := a.placementState(pid); st != "disputed" {
			t.Fatalf("removal in dispute: %s", st)
		}
	}
	if a.count(`SELECT count(*) FROM publication WHERE removed_at IS NULL`) != 0 {
		t.Fatal("live publications after removal")
	}

	// История: P-08 и P-14 с источником.
	if a.count(`SELECT count(*) FROM placement_state_history WHERE transition = 'P-08' AND actor_type = 'system' AND job_id IS NOT NULL`) != 3 {
		t.Fatal("P-08 history")
	}
	if a.count(`SELECT count(*) FROM placement_state_history WHERE transition = 'P-14' AND inbound_event_id IS NOT NULL`) != 2 {
		t.Fatal("P-14 history with events")
	}
	a.checkAutopostLog(t, []string{autopostText, advOrg.INN, advOrg.Name, "Свой пост", "Другой текст"})
}

func TestAutopostingFailures(t *testing.T) {
	a := newTestApp(t)
	adv := a.advertiserClient("adv.fail@example.ru")
	blg := a.bloggerClient("blg.fail@example.ru")
	channels, pids := a.autopostOrder(adv, blg, "Сбой", "Простой", "Права")
	a.clk.Set(at(11, 0, 0))
	a.schedule(blg, pids[0], map[string]any{"mode": "at", "publish_at": at(12, 0, 0).Format(time.RFC3339)})
	a.schedule(blg, pids[1], map[string]any{"mode": "at", "publish_at": at(13, 0, 0).Format(time.RFC3339)})
	a.schedule(blg, pids[2], map[string]any{"mode": "at", "publish_at": at(14, 0, 0).Format(time.RFC3339)})

	// S-21: MAX отвечает ошибкой 31 мин — failed, оповещение.
	a.maxFake().FailSendWith(errors.New("max: status 502"))
	for m := 0; m <= 31; m++ {
		a.clk.Set(at(12, m, 0))
		a.settle()
	}
	a.maxFake().FailSendWith(nil)
	if st, reason := a.placementState(pids[0]); st != "failed" || reason != "publish_error" {
		t.Fatalf("publish error: %s %s", st, reason)
	}
	if a.count(`SELECT count(*) FROM alert WHERE reason = 'placement_publish_failed'`) != 1 {
		t.Fatal("publish failure alert")
	}

	// Простой worker 40 мин после publish_at второго — P-09 без вызова MAX.
	posts := len(a.maxFake().Posts())
	a.clk.Set(at(13, 40, 0))
	a.settle()
	if st, _ := a.placementState(pids[1]); st != "failed" || len(a.maxFake().Posts()) != posts {
		t.Fatalf("after downtime: %s", st)
	}

	// S-25: третий опубликован, бот удалён из канала — спор bot_access_lost; снятие не удаётся.
	a.clk.Set(at(14, 0, 2))
	a.settle()
	if st, _ := a.placementState(pids[2]); st != "published" {
		t.Fatalf("third: %s", st)
	}
	a.send(fmt.Sprintf(`{"update_type":"bot_removed","timestamp":%d,"chat_id":%d,"user":{"user_id":7},"is_channel":true}`,
		at(15, 0, 0).UnixMilli(), a.chatOf(channels[2])))
	a.settle()
	if st, reason := a.placementState(pids[2]); st != "disputed" || reason != "bot_access_lost" {
		t.Fatalf("bot removed: %s %s", st, reason)
	}
	a.maxFake().FailDeletesWith(errors.New("max: status 403"))
	removeAt := at(14, 0, 2).Add(24 * time.Hour)
	for h := 0; h <= 25; h++ {
		a.clk.Set(removeAt.Add(time.Duration(h) * time.Hour))
		a.settle()
	}
	if a.count(`SELECT count(*) FROM publication WHERE placement_id = $1 AND removal_overdue`, pids[2]) != 1 {
		t.Fatal("removal_overdue not set")
	}
	if a.count(`SELECT count(*) FROM alert WHERE reason = 'publication_removal_overdue'`) != 1 {
		t.Fatal("removal overdue alert")
	}
}

// checkAutopostLog — обязательные события этапа (07 §5.3) есть, текст поста и сообщений,
// название и ИНН рекламодателя — нет (07 §5.4).
func (a *testApp) checkAutopostLog(t *testing.T, secrets []string) {
	t.Helper()
	logs := a.logs.Bytes()
	messages := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(logs))
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	for sc.Scan() {
		var rec map[string]any
		if json.Unmarshal(sc.Bytes(), &rec) == nil {
			if m, ok := rec["msg"].(string); ok {
				messages[m] = true
			}
		}
	}
	for _, m := range []string{
		"placement transition applied", "placement published", "placement violation detected", "placement dispute opened",
		"inbound event processed", "job started", "job finished",
	} {
		if !messages[m] {
			t.Errorf("required log event %q is missing", m)
		}
	}
	for _, s := range secrets {
		if s != "" && strings.Contains(string(logs), s) {
			t.Errorf("log contains %q", s)
		}
	}
}
