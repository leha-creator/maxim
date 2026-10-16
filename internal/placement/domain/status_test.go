package domain

import (
	"slices"
	"strings"
	"testing"
)

// Перечисления 04 §9, переписанные из документа.
const (
	docPlacementStatus = "created, awaiting_blogger, awaiting_schedule, scheduled, published, removed, disputed, accepted, " +
		"paid, declined, expired, cancelled, failed, refunded"
	docOrderStatus = "moderation, rejected, marking, in_progress, completed, cancelled"
	docActorType   = "advertiser, blogger, admin, system"
	docReasons     = "advertiser_cancel, admin_cancel, channel_unavailable, window_elapsed, blogger_declined, accept_timeout, " +
		"schedule_timeout, publish_error, precondition_failed, new_post_version, erir_rejected, auto_acceptance"
)

func list[T ~string](items []T) string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		out = append(out, string(s))
	}
	return strings.Join(out, ", ")
}

func TestEnumsMatchDomainModel(t *testing.T) {
	if got := list(Statuses); got != docPlacementStatus {
		t.Errorf("placement.status: %s", got)
	}
	if got := list(OrderStatuses); got != docOrderStatus {
		t.Errorf("order.status: %s", got)
	}
	if got := list(ActorTypes); got != docActorType {
		t.Errorf("actor_type: %s", got)
	}
	if got := strings.Join(Reasons, ", "); got != docReasons {
		t.Errorf("reasons: %s", got)
	}
}

func TestStatusGroups(t *testing.T) {
	before := []Status{StatusCreated, StatusAwaitingBlogger, StatusAwaitingSchedule, StatusScheduled}
	for _, s := range Statuses {
		if s.BeforePublication() != slices.Contains(before, s) {
			t.Errorf("%s: before publication %t", s, s.BeforePublication())
		}
	}
	if !OrderCompleted.Closed() || !OrderCancelled.Closed() || OrderInProgress.Closed() {
		t.Error("closed order statuses: completed and cancelled")
	}
}
