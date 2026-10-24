package domain

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDisputeMessages(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	d := Dispute{ID: uuid.New()}
	admin := uuid.New()
	if _, err := NewAdminMessage(uuid.New(), d, admin, "nobody", "текст", now); !errors.Is(err, ErrMessageRecipient) {
		t.Fatalf("unknown recipient: %v", err)
	}
	if _, err := NewAdminMessage(uuid.New(), d, admin, RecipientBoth, "", now); !errors.Is(err, ErrMessageText) {
		t.Fatalf("empty text: %v", err)
	}
	if _, err := NewPartyMessage(uuid.New(), d, AuthorBlogger, admin, strings.Repeat("я", MaxMessageText+1), now); !errors.Is(err, ErrMessageText) {
		t.Fatalf("long text: %v", err)
	}
	toBlogger, _ := NewAdminMessage(uuid.New(), d, admin, RecipientBlogger, "блогеру", now)
	toBoth, _ := NewAdminMessage(uuid.New(), d, admin, RecipientBoth, "обоим", now)
	reply, _ := NewPartyMessage(uuid.New(), d, AuthorAdvertiser, uuid.New(), "ответ", now)
	cases := []struct {
		m    DisputeMessage
		side MessageAuthor
		want bool
	}{
		{toBlogger, AuthorBlogger, true}, {toBlogger, AuthorAdvertiser, false},
		{toBoth, AuthorAdvertiser, true}, {toBoth, AuthorBlogger, true},
		{reply, AuthorAdvertiser, true}, {reply, AuthorBlogger, false},
	}
	for i, c := range cases {
		if got := c.m.VisibleTo(c.side); got != c.want {
			t.Errorf("case %d: visible to %s = %t", i, c.side, got)
		}
	}
	res := now
	d.Resolution, d.ResolvedAt = ResolutionFullPayout, &res
	if _, err := NewPartyMessage(uuid.New(), d, AuthorBlogger, admin, "поздно", now); !errors.Is(err, ErrDisputeResolved) {
		t.Fatalf("after resolve: %v", err)
	}
}

func TestMarkRemovedByAdmin(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	published := now.Add(-26 * time.Hour)
	r := ControlRules{}.WithDefaults()
	mk := func() Placement {
		return Placement{ID: uuid.New(), Status: StatusPublished, PublishedAt: &published}
	}
	p := mk()
	if _, err := p.MarkRemovedByAdmin(uuid.New(), "", nil, published, now, now, r); !errors.Is(err, ErrReasonRequired) {
		t.Fatalf("no reason: %v", err)
	}
	for _, at := range []time.Time{published.Add(-time.Minute), now.Add(time.Minute)} {
		p := mk()
		if _, err := p.MarkRemovedByAdmin(uuid.New(), "проверено", nil, published, at, now, r); !errors.Is(err, ErrRemovedAtInvalid) {
			t.Fatalf("removed_at %s: %v", at, err)
		}
	}
	views := 1500
	removedAt := now.Add(-time.Hour)
	change, err := p.MarkRemovedByAdmin(uuid.New(), "проверено", &views, published, removedAt, now, r)
	if err != nil || p.Status != StatusRemoved || change.Actor != ActorAdmin || *p.RemovedAt != removedAt ||
		!p.ClaimDeadlineAt.Equal(removedAt.Add(r.ClaimWindow)) || *p.FinalViews != 1500 ||
		!slices.Contains(change.CancelJobs, RemoveKey(p.ID)) {
		t.Fatalf("marked: %+v %+v %v", p, change, err)
	}
	s := Placement{ID: uuid.New(), Status: StatusScheduled}
	if _, err := s.MarkRemovedByAdmin(uuid.New(), "проверено", nil, published, removedAt, now, r); err == nil {
		t.Fatal("scheduled placement marked removed")
	}
}

func TestCancelByAdmin(t *testing.T) {
	now := time.Now()
	for _, st := range []Status{StatusCreated, StatusAwaitingBlogger, StatusAwaitingSchedule, StatusScheduled} {
		p := Placement{ID: uuid.New(), Status: st}
		c, err := p.CancelByAdmin(uuid.New(), "нарушение", now)
		if err != nil || p.Status != StatusCancelled || c.Reason != ReasonAdminCancel || !c.ReleaseHold || c.ReasonText != "нарушение" {
			t.Fatalf("%s: %+v %v", st, c, err)
		}
	}
	p := Placement{ID: uuid.New(), Status: StatusPublished}
	if _, err := p.CancelByAdmin(uuid.New(), "нарушение", now); err == nil {
		t.Fatal("published cancelled")
	}
}

func TestAdminActions(t *testing.T) {
	pub := &Publication{}
	p := Placement{Status: StatusPublished}
	senior := AdminActions(&p, pub, nil, false, true)
	moderator := AdminActions(&p, pub, nil, true, false)
	if !slices.Contains(senior, AdminActionMarkRemoved) || slices.Contains(moderator, AdminActionMarkRemoved) ||
		slices.Contains(moderator, AdminActionRetryPayout) || !slices.Contains(moderator, AdminActionOpenDispute) ||
		!slices.Contains(moderator, AdminActionRemovePost) {
		t.Fatalf("senior %v moderator %v", senior, moderator)
	}
	d := &Dispute{}
	disputed := Placement{Status: StatusDisputed}
	if got := AdminActions(&disputed, nil, d, false, false); !slices.Equal(got, []AdminAction{AdminActionResolveDispute}) {
		t.Fatalf("disputed %v", got)
	}
}
