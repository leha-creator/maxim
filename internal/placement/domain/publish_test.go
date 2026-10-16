package domain

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

// scheduledPlacement — размещение в scheduled с публикацией в 10:00 МСК 20.10 и снятием через сутки.
func scheduledPlacement() *Placement {
	p := testPlacement(StatusScheduled)
	pub := mskTime(2026, 10, 20, 10, 0)
	rem := pub.Add(24 * time.Hour)
	p.PublishAt, p.RemoveAt = &pub, &rem
	sent := pub.Add(-48 * time.Hour)
	p.SentToBloggerAt = &sent
	return p
}

func readyContent(p *Placement, images int) Content {
	c := Content{VersionID: p.PostVersionID, Status: VersionReady, Erid: "erid-1", FinalText: "text erid: erid-1", MarkingOK: true}
	for range images {
		c.Media = append(c.Media, MediaRef{DerivativeID: uuid.New(), ContentType: "image/jpeg", Size: 10})
	}
	return c
}

func TestCheckPublish(t *testing.T) {
	r := DefaultControlRules
	active := ChannelOffer{Active: true}
	tests := []struct {
		name    string
		mutate  func(p *Placement, ch *ChannelOffer, c *Content)
		wantErr string
	}{
		{"ok", func(*Placement, *ChannelOffer, *Content) {}, ""},
		{"not scheduled", func(p *Placement, _ *ChannelOffer, _ *Content) { p.Status = StatusCancelled }, "cancelled"},
		{"channel suspended", func(_ *Placement, ch *ChannelOffer, _ *Content) { ch.Active = false }, PublishChannelNotActive},
		{"version approved", func(_ *Placement, _ *ChannelOffer, c *Content) { c.Status = VersionApproved }, PublishVersionNotReady},
		{"other version", func(_ *Placement, _ *ChannelOffer, c *Content) { c.VersionID = uuid.New() }, PublishVersionNotReady},
		{"no erid", func(_ *Placement, _ *ChannelOffer, c *Content) { c.Erid = "" }, PublishVersionNotReady},
		{"empty final text", func(_ *Placement, _ *ChannelOffer, c *Content) { c.FinalText = "" }, PublishVersionNotReady},
		{"marking mismatch", func(_ *Placement, _ *ChannelOffer, c *Content) { c.MarkingOK = false }, PublishMarkingMismatch},
		{"11 images", func(p *Placement, _ *ChannelOffer, c *Content) { *c = readyContent(p, 11) }, PublishTooManyImages},
	}
	for _, tc := range tests {
		p, ch := scheduledPlacement(), active
		c := readyContent(p, 10)
		tc.mutate(p, &ch, &c)
		err := CheckPublish(p, ch, c, r)
		var pe *PreconditionError
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: unexpected %v", tc.name, err)
		case tc.wantErr != "" && (!errors.As(err, &pe) || pe.Reason != tc.wantErr):
			t.Errorf("%s: err %v, want reason %s", tc.name, err, tc.wantErr)
		}
	}
}

func TestPublishRetryWindow(t *testing.T) {
	r := DefaultControlRules
	p := scheduledPlacement()
	at := *p.PublishAt
	tests := []struct {
		now      time.Time
		retry    time.Duration
		fail     bool
		expired  bool
		describe string
	}{
		{at, 30 * time.Second, false, false, "first failure"},
		{at.Add(29*time.Minute + 50*time.Second), 10 * time.Second, false, false, "last 10 s of T-03"},
		{at.Add(30*time.Minute - time.Second), time.Second, false, false, "one second before T-03"},
		{at.Add(30 * time.Minute), 0, true, false, "T-03 reached"},
		{at.Add(31 * time.Minute), 0, true, true, "after downtime"},
	}
	for _, tc := range tests {
		retry, fail := p.PublishRetry(tc.now, r)
		if retry != tc.retry || fail != tc.fail {
			t.Errorf("%s: retry %s fail %v, want %s %v", tc.describe, retry, fail, tc.retry, tc.fail)
		}
		if got := p.PublishExpired(tc.now, r); got != tc.expired {
			t.Errorf("%s: expired %v, want %v", tc.describe, got, tc.expired)
		}
	}
}

func TestRemoveRetryGrowsUntilT10(t *testing.T) {
	r := DefaultControlRules
	p := scheduledPlacement()
	at := *p.RemoveAt
	tests := []struct {
		attempts  int
		now       time.Time
		retry     time.Duration
		exhausted bool
	}{
		{0, at, 30 * time.Second, false},
		{1, at, time.Minute, false},
		{3, at, 4 * time.Minute, false},
		{7, at, time.Hour, false},
		{20, at, time.Hour, false},
		{20, at.Add(24*time.Hour - 10*time.Minute), 10 * time.Minute, false},
		{20, at.Add(24 * time.Hour), 0, true},
	}
	for _, tc := range tests {
		retry, exhausted := p.RemoveRetry(tc.attempts, tc.now, r)
		if retry != tc.retry || exhausted != tc.exhausted {
			t.Errorf("attempts %d at +%s: retry %s exhausted %v, want %s %v", tc.attempts, tc.now.Sub(at), retry,
				exhausted, tc.retry, tc.exhausted)
		}
	}
}

func TestPublishRecomputesRemoveAt(t *testing.T) {
	p := scheduledPlacement()
	sent := p.PublishAt.Add(7 * time.Second)
	c, err := p.Publish(SentMessage{ID: "mid.1", At: sent}, sent.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != StatusPublished || !p.PublishedAt.Equal(sent) || !p.RemoveAt.Equal(sent.Add(24*time.Hour)) {
		t.Fatalf("published %v remove %v status %s", p.PublishedAt, p.RemoveAt, p.Status)
	}
	if len(c.MoveJobs) != 1 || c.MoveJobs[0].DedupeKey != RemoveKey(p.ID) || !c.MoveJobs[0].RunAt.Equal(*p.RemoveAt) {
		t.Fatalf("remove job move %+v", c.MoveJobs)
	}
	if len(c.Notices) != 2 || c.Notices[0].Template != NoticePublished {
		t.Fatalf("notices %+v", c.Notices)
	}
	// Время MAX не пришло — берётся текущий момент.
	p2 := scheduledPlacement()
	now := p2.PublishAt.Add(3 * time.Second)
	if _, err := p2.Publish(SentMessage{ID: "mid.2"}, now); err != nil || !p2.PublishedAt.Equal(now) {
		t.Fatalf("publish without time: %v %v", err, p2.PublishedAt)
	}
}

func TestFailPublishCancelsRemoval(t *testing.T) {
	p := scheduledPlacement()
	c, err := p.FailPublish(ReasonPublishError, "access.denied", *p.PublishAt)
	if err != nil {
		t.Fatal(err)
	}
	if p.Status != StatusFailed || !c.ReleaseHold || c.ReasonText != "access.denied" || !slices.Contains(c.CancelJobs, RemoveKey(p.ID)) {
		t.Fatalf("P-09: status %s change %+v", p.Status, c)
	}
	if _, err := p.FailPublish(ReasonPublishError, "", *p.PublishAt); err == nil {
		t.Fatal("P-09 from failed accepted")
	}
}

func TestRemoveStartsClaimWindow(t *testing.T) {
	p := scheduledPlacement()
	if _, err := p.Publish(SentMessage{ID: "mid.1", At: *p.PublishAt}, *p.PublishAt); err != nil {
		t.Fatal(err)
	}
	removedAt := p.RemoveAt.Add(4 * time.Second)
	views := 1200
	c, err := p.Remove(&views, removedAt, DefaultControlRules)
	if err != nil {
		t.Fatal(err)
	}
	claim := removedAt.Add(24 * time.Hour)
	if p.Status != StatusRemoved || *p.FinalViews != 1200 || !p.ClaimDeadlineAt.Equal(claim) || !p.RemovedAt.Equal(removedAt) {
		t.Fatalf("P-10: %+v", p)
	}
	if len(c.Jobs) != 1 || c.Jobs[0].DedupeKey != TimerKey(p.ID, TimerClaim) || !c.Jobs[0].RunAt.Equal(claim) {
		t.Fatalf("T-05: %+v", c.Jobs)
	}
	if c.ReleaseHold {
		t.Fatal("P-10 released hold")
	}
}

func TestOpenDispute(t *testing.T) {
	now := mskTime(2026, 10, 20, 10, 30)
	for _, from := range []Status{StatusPublished, StatusRemoved} {
		p := testPlacement(from)
		c, err := p.OpenDispute(ViolationTopViolated, now, DefaultControlRules)
		if err != nil {
			t.Fatalf("P-14 from %s: %v", from, err)
		}
		if p.Status != StatusDisputed || p.StatusReason != "top_violated" || c.CancelJobs[0] != TimerKey(p.ID, TimerClaim) {
			t.Fatalf("P-14 from %s: %+v", from, c)
		}
		if len(c.Jobs) != 1 || c.Jobs[0].DedupeKey != TimerKey(p.ID, TimerDispute) || !c.Jobs[0].RunAt.Equal(now.Add(72*time.Hour)) {
			t.Fatalf("T-06: %+v", c.Jobs)
		}
	}
	p := testPlacement(StatusDisputed)
	if _, err := p.OpenDispute(ViolationTopViolated, now, DefaultControlRules); err == nil {
		t.Fatal("second dispute accepted")
	}
}

func TestSuggestedShare(t *testing.T) {
	r := DefaultControlRules
	pub := mskTime(2026, 10, 20, 10, 0)
	rem := pub.Add(24 * time.Hour)
	// 03 §7: удалён через 18 ч из 24 — 7500.
	if s := SuggestedShare(ViolationRemovedEarly, pub, pub.Add(18*time.Hour), rem, r); s == nil || *s != 7500 {
		t.Fatalf("removed after 18 h: %v", s)
	}
	if s := SuggestedShare(ViolationRemovedEarly, pub, pub.Add(-time.Minute), rem, r); s == nil || *s != 0 {
		t.Fatalf("removed before publish: %v", s)
	}
	// Округление вниз: 1 ч из 24 — 416,66 → 416.
	if s := SuggestedShare(ViolationRemovedEarly, pub, pub.Add(time.Hour), rem, r); s == nil || *s != 416 {
		t.Fatalf("removed after 1 h: %v", s)
	}
	if s := SuggestedShare(ViolationTopViolated, pub, pub, rem, r); s == nil || *s != 5000 {
		t.Fatalf("top violated: %v", s)
	}
	for _, k := range []ViolationKind{ViolationContentModified, ViolationBotAccessLost} {
		if s := SuggestedShare(k, pub, pub, rem, r); s != nil {
			t.Fatalf("%s: suggestion %d", k, *s)
		}
	}
}

func TestPublicationRemovalIntent(t *testing.T) {
	now := mskTime(2026, 10, 21, 10, 0)
	pub := Publication{ID: uuid.New(), PublishedAt: now.Add(-24 * time.Hour)}
	pub.MarkRemoving(now)
	if pub.Live() || pub.RemovalReason != RemovalSchedule {
		t.Fatalf("intent: %+v", pub)
	}
	pub.RollbackRemoving(now.Add(time.Second))
	if !pub.Live() || !pub.RemovalOverdue || pub.RemovalReason != "" {
		t.Fatalf("rollback: %+v", pub)
	}
	pub.MarkRemovedExternally(now.Add(-time.Hour), now)
	if pub.Live() || pub.RemovalReason != RemovalExternal || pub.RemovalOverdue || !pub.RemovedAt.Equal(now.Add(-time.Hour)) {
		t.Fatalf("external: %+v", pub)
	}
	if !pub.ViewsDue(now, time.Hour) {
		t.Fatal("views never read: due")
	}
	v := 5
	pub.UpdateViews(&v, now)
	if pub.ViewsDue(now.Add(59*time.Minute), time.Hour) || !pub.ViewsDue(now.Add(61*time.Minute), time.Hour) {
		t.Fatal("views refresh interval")
	}
	pub.UpdateViews(nil, now.Add(2*time.Hour))
	if *pub.Views != 5 {
		t.Fatal("views lost when MAX returned none")
	}
}
