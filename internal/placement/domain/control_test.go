package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

const testBot int64 = 777

// livePlacement — опубликованное размещение формата 1/24 с топом [published, published + 1 ч).
func livePlacement(published time.Time, msgID string) LivePlacement {
	p := testPlacement(StatusPublished)
	rem := published.Add(24 * time.Hour)
	p.PublishAt, p.RemoveAt, p.PublishedAt = &published, &rem, &published
	return LivePlacement{Placement: *p, Publication: Publication{
		ID: uuid.New(), PlacementID: p.ID, ChannelID: p.ChannelID, MaxMessageID: msgID, PublishedAt: published, ContentHash: "h1",
	}}
}

func TestClassifyChannelPostTop(t *testing.T) {
	t0 := mskTime(2026, 10, 20, 10, 0)
	first := livePlacement(t0, "mid.ad1")
	second := livePlacement(t0.Add(time.Hour), "mid.ad2") // стык топов допустим (I-06)
	disputed := livePlacement(t0, "mid.ad3")
	disputed.Placement.Status = StatusDisputed
	removed := livePlacement(t0, "mid.ad4")
	removed.Publication.MarkRemoving(t0)
	live := []LivePlacement{first, second}
	tests := []struct {
		name   string
		live   []LivePlacement
		sender int64
		at     time.Time
		want   []uuid.UUID
	}{
		{"human in first top", live, 1, t0.Add(30 * time.Minute), []uuid.UUID{first.Placement.ID}},
		{"last second of first top", live, 1, t0.Add(time.Hour - time.Second), []uuid.UUID{first.Placement.ID}},
		{"end of first top belongs to second", live, 1, t0.Add(time.Hour), []uuid.UUID{second.Placement.ID}},
		{"after all tops", live, 1, t0.Add(2 * time.Hour), nil},
		{"before publication", live, 1, t0.Add(-time.Second), nil},
		{"our bot: other ad", live, testBot, t0.Add(10 * time.Minute), nil},
		{"post as channel (no sender)", live, 0, t0.Add(10 * time.Minute), []uuid.UUID{first.Placement.ID}},
		{"disputed still controlled", []LivePlacement{disputed}, 1, t0.Add(time.Minute), []uuid.UUID{disputed.Placement.ID}},
		{"removal started", []LivePlacement{removed}, 1, t0.Add(time.Minute), nil},
	}
	for _, tc := range tests {
		ev := uuid.New()
		got := ClassifyChannelPost(tc.live, ChannelPost{MessageID: "mid.x", SenderID: tc.sender, At: tc.at, InboundEventID: &ev}, testBot)
		if len(got) != len(tc.want) {
			t.Errorf("%s: %d violations, want %d", tc.name, len(got), len(tc.want))
			continue
		}
		for i, v := range got {
			if v.PlacementID != tc.want[i] || v.Kind != ViolationTopViolated || !v.OccurredAt.Equal(tc.at) ||
				v.Source != SourceEvent || *v.InboundEventID != ev || v.Details["message_id"] != "mid.x" {
				t.Errorf("%s: violation %+v", tc.name, v)
			}
		}
	}
}

func TestClassifyRemovedAndEdited(t *testing.T) {
	t0 := mskTime(2026, 10, 20, 10, 0)
	lp := livePlacement(t0, "mid.1")
	at := t0.Add(5 * time.Hour)
	if v, ok := ClassifyRemoved(lp.Publication, at); !ok || v.Kind != ViolationRemovedEarly || !v.OccurredAt.Equal(at) {
		t.Fatalf("foreign removal: %+v %v", v, ok)
	}
	own := lp.Publication
	own.MarkRemoving(at)
	if _, ok := ClassifyRemoved(own, at); ok {
		t.Fatal("own removal reported as violation")
	}
	if v, ok := ClassifyEdited(lp.Publication, "h2", at); !ok || v.Kind != ViolationContentModified {
		t.Fatalf("edit with other hash: %+v %v", v, ok)
	}
	for _, h := range []string{"h1", ""} {
		if _, ok := ClassifyEdited(lp.Publication, h, at); ok {
			t.Fatalf("edit with hash %q reported", h)
		}
	}
}

func TestTopCheckWindow(t *testing.T) {
	t0 := mskTime(2026, 10, 20, 10, 0)
	lp := livePlacement(t0, "mid.1")
	r := DefaultControlRules
	from, to, ok := TopCheckWindow(&lp.Placement, t0.Add(20*time.Minute), r)
	if !ok || !from.Equal(t0) || !to.Equal(t0.Add(20*time.Minute)) {
		t.Fatalf("inside top: %s %s %v", from, to, ok)
	}
	if _, to, ok := TopCheckWindow(&lp.Placement, t0.Add(time.Hour+time.Minute), r); !ok || !to.Before(t0.Add(time.Hour)) {
		t.Fatalf("first check after top: %s %v", to, ok)
	}
	if _, _, ok := TopCheckWindow(&lp.Placement, t0.Add(time.Hour+6*time.Minute), r); ok {
		t.Fatal("top window read long after top end")
	}
}

func TestReconcile(t *testing.T) {
	t0 := mskTime(2026, 10, 20, 10, 0)
	r := DefaultControlRules
	now := t0.Add(30 * time.Minute)
	views := 300
	ok := &ChannelMessage{ID: "mid.1", At: t0, SenderID: testBot, ContentHash: "h1", Views: &views}
	kinds := func(res ReconcileResult) []ViolationKind {
		var out []ViolationKind
		for _, v := range res.Violations {
			out = append(out, v.Kind)
		}
		return out
	}

	lp := livePlacement(t0, "mid.1")
	res := Reconcile(lp, nil, nil, false, testBot, now, r)
	if k := kinds(res); len(k) != 1 || k[0] != ViolationRemovedEarly || res.Violations[0].Source != SourceReconciliation {
		t.Fatalf("missing message: %+v", res)
	}

	edited := *ok
	edited.ContentHash = "h2"
	if k := kinds(Reconcile(lp, &edited, nil, false, testBot, now, r)); len(k) != 1 || k[0] != ViolationContentModified {
		t.Fatalf("edited: %v", k)
	}

	recent := []ChannelMessage{
		{ID: "mid.h2", At: t0.Add(25 * time.Minute), SenderID: 5},
		{ID: "mid.bot", At: t0.Add(20 * time.Minute), SenderID: testBot},
		{ID: "mid.h1", At: t0.Add(15 * time.Minute), SenderID: 0},
		{ID: "mid.1", At: t0, SenderID: testBot},
	}
	res = Reconcile(lp, ok, recent, true, testBot, now, r)
	if k := kinds(res); len(k) != 1 || k[0] != ViolationTopViolated || res.Violations[0].Details["message_id"] != "mid.h1" ||
		!res.Violations[0].OccurredAt.Equal(t0.Add(15*time.Minute)) {
		t.Fatalf("top: %+v", res.Violations)
	}
	if !res.UpdateViews || *res.Views != 300 {
		t.Fatalf("views never read: %+v", res)
	}

	checked := t0.Add(-59 * time.Minute)
	lp.Publication.ViewsCheckedAt = &checked
	if res := Reconcile(lp, ok, nil, false, testBot, t0, r); res.UpdateViews || len(res.Violations) != 0 {
		t.Fatalf("views read 59 min ago: %+v", res)
	}
	if res := Reconcile(lp, ok, nil, false, testBot, t0.Add(2*time.Minute), r); !res.UpdateViews {
		t.Fatal("views read 61 min ago not refreshed")
	}

	removed := livePlacement(t0, "mid.9")
	removed.Publication.MarkRemoving(now)
	if res := Reconcile(removed, nil, nil, false, testBot, now, r); len(res.Violations) != 0 {
		t.Fatalf("removal in progress reported: %+v", res)
	}
}
