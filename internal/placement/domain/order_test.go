package domain

import (
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestDeriveOrderStatus(t *testing.T) {
	tests := []struct {
		name     string
		statuses []Status
		version  VersionStatus
		want     OrderStatus
	}{
		{"all cancelled", []Status{StatusCancelled, StatusCancelled}, VersionReady, OrderCancelled},
		{"all terminal", []Status{StatusCancelled, StatusDeclined, StatusExpired}, VersionReady, OrderCompleted},
		{"all terminal with paid", []Status{StatusPaid, StatusRefunded}, VersionReady, OrderCompleted},
		{"pending moderation", []Status{StatusCreated, StatusCancelled}, VersionPendingModeration, OrderModeration},
		{"rejected", []Status{StatusCreated}, VersionRejected, OrderRejected},
		{"approved", []Status{StatusCreated}, VersionApproved, OrderMarking},
		{"ready", []Status{StatusAwaitingBlogger, StatusDeclined}, VersionReady, OrderInProgress},
		{"no placements", nil, VersionPendingModeration, OrderModeration},
	}
	for _, tc := range tests {
		if got := DeriveOrderStatus(tc.statuses, tc.version); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestActions(t *testing.T) {
	now := mskTime(2026, 10, 20, 10, 0)
	for _, s := range Statuses {
		p := testPlacement(s)
		p.PublishAt = ptrTime(now.Add(2 * time.Hour))
		p.ClaimDeadlineAt = ptrTime(now.Add(time.Hour))
		adv := AdvertiserActions(p, false, now)
		wantAdv := []string{}
		switch s {
		case StatusCreated, StatusAwaitingBlogger:
			wantAdv = []string{ActionCancel}
		case StatusPublished:
			wantAdv = []string{ActionClaim}
		case StatusRemoved:
			wantAdv = []string{ActionAccept, ActionClaim}
		}
		if s == StatusRemoved {
			if got := AdvertiserActions(p, true, now); !slices.Equal(got, []string{ActionAccept}) {
				t.Errorf("removed with claim: %v", got)
			}
			if got := AdvertiserActions(p, false, now.Add(time.Hour)); !slices.Equal(got, []string{ActionAccept}) {
				t.Errorf("removed after claim window: %v", got)
			}
		}
		if !slices.Equal(adv, wantAdv) {
			t.Errorf("%s: advertiser %v", s, adv)
		}
		var wantBlg []string
		switch s {
		case StatusAwaitingBlogger:
			wantBlg = []string{ActionAccept, ActionDecline}
		case StatusAwaitingSchedule:
			wantBlg = []string{ActionSchedule}
		case StatusScheduled:
			wantBlg = []string{ActionReschedule}
		default:
			wantBlg = []string{}
		}
		if got := BloggerActions(p, now, DefaultTiming); !slices.Equal(got, wantBlg) {
			t.Errorf("%s: blogger %v", s, got)
		}
	}
	p := testPlacement(StatusScheduled)
	p.PublishAt = ptrTime(now.Add(50 * time.Minute))
	if got := BloggerActions(p, now, DefaultTiming); len(got) != 0 {
		t.Errorf("reschedule 50 min before: %v", got)
	}
}

func TestPageCursor(t *testing.T) {
	c := PageCursor{CreatedAt: time.Date(2026, 10, 20, 10, 0, 0, 123000, time.UTC), ID: uuid.New()}
	got, err := DecodePageCursor(c.Encode())
	if err != nil || !got.CreatedAt.Equal(c.CreatedAt) || got.ID != c.ID {
		t.Fatalf("round trip: %v %v", got, err)
	}
	if got, err := DecodePageCursor(""); got != nil || err != nil {
		t.Fatal("empty cursor is the first page")
	}
	if _, err := DecodePageCursor("!!"); err == nil {
		t.Fatal("broken cursor accepted")
	}
}
