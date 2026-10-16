package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestAcceptDeadline(t *testing.T) {
	w := testWindow()
	tm := DefaultTiming
	sent := mskTime(2026, 10, 20, 13, 0)
	if got := AcceptDeadline(sent, w, tm); !got.Equal(sent.Add(24 * time.Hour)) {
		t.Errorf("24 h: %s", got)
	}
	late := mskTime(2026, 10, 22, 9, 0)
	if got, want := AcceptDeadline(late, w, tm), mskTime(2026, 10, 22, 23, 0); !got.Equal(want) {
		t.Errorf("cut by window end - 1 h: %s, want %s", got, want)
	}
}

func TestScheduleDeadline(t *testing.T) {
	w := testWindow()
	tm := DefaultTiming
	accepted := mskTime(2026, 10, 20, 13, 0)
	if got := ScheduleDeadline(accepted, w, tm); !got.Equal(accepted.Add(24 * time.Hour)) {
		t.Errorf("24 h: %s", got)
	}
	late := mskTime(2026, 10, 22, 3, 0)
	if got, want := ScheduleDeadline(late, w, tm), mskTime(2026, 10, 22, 20, 55); !got.Equal(want) {
		t.Errorf("cut by last slot: %s, want %s", got, want)
	}
	if got, want := WindowDeadline(w, tm), mskTime(2026, 10, 22, 23, 0); !got.Equal(want) {
		t.Errorf("T-14: %s, want %s", got, want)
	}
}

func TestJobsAndKeys(t *testing.T) {
	id := uuid.MustParse("0190a000-0000-7000-8000-000000000001")
	at := mskTime(2026, 10, 20, 10, 0)
	j := TimerJob(id, TimerAccept, at, StatusAwaitingBlogger)
	if j.Kind != KindPlacementTimer || j.DedupeKey != "timer:"+id.String()+":T-01" || !j.RunAt.Equal(at) {
		t.Fatalf("timer job %+v", j)
	}
	if p, ok := j.Payload.(TimerPayload); !ok || p.PlacementID != id || p.Timer != TimerAccept || p.ExpectedStatus != StatusAwaitingBlogger {
		t.Fatalf("payload %+v", j.Payload)
	}
	if PublishJob(id, at).DedupeKey != "publish:"+id.String() || RemoveJob(id, at).DedupeKey != "remove:"+id.String() {
		t.Fatal("publish and remove keys (07 §2)")
	}
	for timer, want := range map[TimerCode]Status{TimerAccept: StatusAwaitingBlogger, TimerSchedule: StatusAwaitingSchedule, TimerWindow: StatusCreated} {
		if timer.ExpectedStatus() != want {
			t.Errorf("%s expects %s", timer, timer.ExpectedStatus())
		}
	}
	if (Timing{}).WithDefaults() != DefaultTiming {
		t.Fatal("zero timing gets defaults")
	}
}
