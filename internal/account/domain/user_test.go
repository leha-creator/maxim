package domain

import (
	"testing"

	"github.com/google/uuid"

	"github.com/leha-creator/maxim/internal/platform/apperr"
)

func TestUserBlock(t *testing.T) {
	u := NewUser(uuid.New(), "a@example.ru", now)
	if changed, err := u.Block("fraud", now); err != nil || !changed || !u.Blocked() || u.BlockedAt == nil || *u.BlockedReason != "fraud" {
		t.Fatalf("block: %v %v %+v", changed, err, u)
	}
	if changed, err := u.Block("again", now); err != nil || changed || *u.BlockedReason != "fraud" {
		t.Fatalf("repeat block: %v %v", changed, err)
	}
	if !u.Unblock(now) || u.Blocked() || u.BlockedAt != nil || u.BlockedReason != nil {
		t.Fatalf("unblock: %+v", u)
	}
	if u.Unblock(now) {
		t.Fatal("repeat unblock changed")
	}
	role := AdminModerator
	u.AdminRole = &role
	_, err := u.Block("x", now)
	if e, ok := apperr.As(err); !ok || e.Code != apperr.CodeConflict || e.Details["reason"] != ReasonAdminUser {
		t.Fatalf("block admin: %v", err)
	}
}
