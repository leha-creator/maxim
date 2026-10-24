package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewClaim(t *testing.T) {
	now := mskTime(2026, 10, 20, 10, 0)
	tests := []struct {
		name   string
		reason ClaimReason
		text   string
		want   error
	}{
		{"ok", ClaimContentMismatch, "  не тот текст  ", nil},
		{"2000 runes", ClaimOther, strings.Repeat("я", 2000), nil},
		{"2001 runes", ClaimOther, strings.Repeat("я", 2001), ErrClaimText},
		{"blank", ClaimOther, "   ", ErrClaimText},
		{"system-only reason", ClaimReason("content_modified"), "x", ErrClaimReason},
	}
	for _, tt := range tests {
		c, err := NewClaim(uuid.New(), uuid.New(), uuid.New(), tt.reason, tt.text, now)
		if !errors.Is(err, tt.want) {
			t.Errorf("%s: err %v, want %v", tt.name, err, tt.want)
		}
		if err == nil && c.Text != strings.TrimSpace(tt.text) {
			t.Errorf("%s: text not trimmed", tt.name)
		}
	}
}

func TestClaimWindow(t *testing.T) {
	now := mskTime(2026, 10, 20, 10, 0)
	p := testPlacement(StatusRemoved)
	p.ClaimDeadlineAt = ptrTime(now.Add(time.Second))
	if !ClaimWindowOpen(p, now) {
		t.Error("open 1 s before deadline")
	}
	if ClaimWindowOpen(p, now.Add(time.Second)) {
		t.Error("open at deadline")
	}
	change, err := p.FileClaim(uuid.New(), ClaimOther, now.Add(time.Second), DefaultControlRules)
	var te *TransitionError
	if !errors.As(err, &te) || te.Reason != ReasonClaimWindowClosed || p.Status != StatusRemoved {
		t.Fatalf("late claim: %v %+v", err, change)
	}
	pub := testPlacement(StatusPublished)
	if !ClaimWindowOpen(pub, now) {
		t.Error("published without deadline is open")
	}
	change, err = pub.FileClaim(uuid.New(), ClaimTopViolated, now, DefaultControlRules)
	if err != nil || pub.Status != StatusDisputed || change.Reason != "claim_top_violated" {
		t.Fatalf("claim from published: %v %+v", err, change)
	}
	if len(change.Jobs) != 1 || change.Jobs[0].DedupeKey != TimerKey(pub.ID, TimerDispute) ||
		!change.Jobs[0].RunAt.Equal(now.Add(72*time.Hour)) {
		t.Errorf("T-06: %+v", change.Jobs)
	}
	if !slices.Contains(change.CancelJobs, TimerKey(pub.ID, TimerClaim)) {
		t.Errorf("T-05 not cancelled: %v", change.CancelJobs)
	}
	if len(change.Notices) != 2 || change.Notices[1].Role != ActorAdmin {
		// SentToBloggerAt пуст в testPlacement: блогеру не пишется, рекламодателю и администратору — да.
		t.Errorf("notices: %+v", change.Notices)
	}
	if _, err := testPlacement(StatusAccepted).FileClaim(uuid.New(), ClaimOther, now, DefaultControlRules); err == nil {
		t.Error("claim from accepted")
	}
}

func TestResolutionShare(t *testing.T) {
	tests := []struct {
		name             string
		res              Resolution
		share, suggested *int
		want             int
		err              error
	}{
		{"full payout", ResolutionFullPayout, nil, intPtr(7500), 10000, nil},
		{"full payout explicit", ResolutionFullPayout, intPtr(10000), nil, 10000, nil},
		{"full payout wrong share", ResolutionFullPayout, intPtr(5000), nil, 0, ErrShareOutOfRange},
		{"full refund", ResolutionFullRefund, nil, nil, 0, nil},
		{"proportional explicit", ResolutionProportional, intPtr(1), intPtr(7500), 1, nil},
		{"proportional suggested", ResolutionProportional, nil, intPtr(7500), 7500, nil},
		{"proportional no suggestion", ResolutionProportional, nil, nil, 0, ErrShareRequired},
		{"proportional suggestion 10000", ResolutionProportional, nil, intPtr(10000), 0, ErrShareRequired},
		{"proportional 10000", ResolutionProportional, intPtr(10000), nil, 0, ErrShareOutOfRange},
		{"proportional 0", ResolutionProportional, intPtr(0), nil, 0, ErrShareOutOfRange},
		{"unknown", Resolution("half"), nil, nil, 0, ErrUnknownResolution},
	}
	for _, tt := range tests {
		got, err := ResolutionShare(tt.res, tt.share, tt.suggested)
		if !errors.Is(err, tt.err) || got != tt.want {
			t.Errorf("%s: %d %v, want %d %v", tt.name, got, err, tt.want, tt.err)
		}
	}
}

func TestDisputeResolve(t *testing.T) {
	now := mskTime(2026, 10, 20, 10, 0)
	d := NewAdminDispute(uuid.New(), uuid.New(), now, DefaultControlRules)
	if d.SuggestedShare != nil || d.Reason != DisputeReasonAdmin || !d.DeadlineAt.Equal(now.Add(72*time.Hour)) {
		t.Fatalf("admin dispute %+v", d)
	}
	if err := d.Resolve(ResolutionFullRefund, 0, " ", uuid.New(), now); !errors.Is(err, ErrResolutionComment) {
		t.Fatalf("blank comment: %v", err)
	}
	if err := d.Resolve(ResolutionFullRefund, 0, "пост не выходил", uuid.New(), now); err != nil || !d.Resolved() {
		t.Fatalf("resolve: %v", err)
	}
	if err := d.Resolve(ResolutionFullPayout, 10000, "передумали", uuid.New(), now); !errors.Is(err, ErrDisputeResolved) {
		t.Fatalf("second resolve: %v", err)
	}
}

func TestResolveDisputeTransition(t *testing.T) {
	now := mskTime(2026, 10, 20, 10, 0)
	for _, res := range Resolutions {
		p := testPlacement(StatusDisputed)
		p.SentToBloggerAt = ptrTime(now.Add(-48 * time.Hour))
		p.BloggerAmount, p.FeeAmount, p.TotalAmount = 1000000, 150000, 1150000
		share, _ := ResolutionShare(res, intPtr(map[Resolution]int{ResolutionFullPayout: 10000, ResolutionProportional: 7500}[res]), nil)
		d, err := Distribute(p.BloggerAmount, p.FeeAmount, p.TotalAmount, share)
		if err != nil {
			t.Fatal(err)
		}
		change, err := p.ResolveDispute(res, uuid.New(), d, now)
		if err != nil {
			t.Fatalf("%s: %v", res, err)
		}
		if p.Status != res.Status() || change.ReleaseHold != (res == ResolutionFullRefund) {
			t.Errorf("%s: status %s release %t", res, p.Status, change.ReleaseHold)
		}
		if p.RefundAmount == nil || *p.RefundAmount != d.RefundAmount || *p.PerformanceShare != share {
			t.Errorf("%s: distribution not set", res)
		}
		if len(change.Notices) != 2 || change.Notices[0].Template != NoticeDisputeResolved ||
			change.Notices[0].Data["refund_amount"] != int64(d.RefundAmount) {
			t.Errorf("%s: notices %+v", res, change.Notices)
		}
		if !slices.Equal(change.CancelJobs, []string{TimerKey(p.ID, TimerDispute)}) {
			t.Errorf("%s: cancel %v", res, change.CancelJobs)
		}
	}
	// Пример 03 §7: доля 7500 — 7 500,00 блогеру, 1 125,00 комиссия, 2 875,00 возврат.
	d, _ := Distribute(1000000, 150000, 1150000, 7500)
	if d.PayoutAmount != 750000 || d.FeeCharged != 112500 || d.RefundAmount != 287500 {
		t.Errorf("03 §7 example: %+v", d)
	}
	if _, err := testPlacement(StatusRemoved).ResolveDispute(ResolutionFullPayout, uuid.New(), d, now); err == nil {
		t.Error("P-15 from removed")
	}
}

func TestAcceptWorkAndSettleable(t *testing.T) {
	now := mskTime(2026, 10, 20, 10, 0)
	p := testPlacement(StatusRemoved)
	change, err := p.AcceptWork(P13AutoAccept, ActorSystem, nil, now)
	if err != nil || p.Status != StatusAccepted || change.Reason != ReasonAutoAcceptance ||
		!slices.Equal(change.CancelJobs, []string{TimerKey(p.ID, TimerClaim)}) {
		t.Fatalf("P-13: %v %+v", err, change)
	}
	if _, err := testPlacement(StatusRemoved).AcceptWork(P12AcceptByAdvertiser, ActorSystem, nil, now); err == nil {
		t.Error("P-12 by system")
	}
	if _, err := testPlacement(StatusPublished).AcceptWork(P12AcceptByAdvertiser, ActorAdvertiser, nil, now); err == nil {
		t.Error("P-12 from published")
	}
	live := Publication{PublishedAt: now.Add(-time.Hour)}
	var te *TransitionError
	if err := p.CheckSettleable(P15ResolveDispute, &live); !errors.As(err, &te) || te.Reason != ReasonPublicationLive {
		t.Errorf("live publication: %v", err)
	}
	if err := p.CheckSettleable(P12AcceptByAdvertiser, nil); err == nil {
		t.Error("no publication")
	}
	live.RemovedAt = ptrTime(now)
	if err := p.CheckSettleable(P12AcceptByAdvertiser, &live); err != nil {
		t.Errorf("removed publication: %v", err)
	}
}

func TestClaimSuggestedShareAndPenalty(t *testing.T) {
	now := mskTime(2026, 10, 20, 10, 0)
	pub := Publication{PublishedAt: now}
	removeAt := now.Add(24 * time.Hour)
	if ClaimSuggestedShare(ClaimRemovedEarly, pub, removeAt, DefaultControlRules) != nil {
		t.Error("removed_early with live publication")
	}
	pub.RemovedAt = ptrTime(now.Add(18 * time.Hour))
	if s := ClaimSuggestedShare(ClaimRemovedEarly, pub, removeAt, DefaultControlRules); s == nil || *s != 7500 {
		t.Errorf("removed_early: %v", s)
	}
	if s := ClaimSuggestedShare(ClaimTopViolated, pub, removeAt, DefaultControlRules); s == nil || *s != 5000 {
		t.Errorf("top_violated: %v", s)
	}
	if ClaimSuggestedShare(ClaimOther, pub, removeAt, DefaultControlRules) != nil {
		t.Error("other")
	}
	r := ControlRules{RatingPenaltyRefund: 10, RatingPenaltyProportional: 5}
	if RatingPenalty(ResolutionFullRefund, r) != 10 || RatingPenalty(ResolutionProportional, r) != 5 ||
		RatingPenalty(ResolutionFullPayout, r) != 0 {
		t.Error("rating penalty")
	}
}
