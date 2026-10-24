package domain

import (
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestApprovalRules(t *testing.T) {
	now := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	author, other := uuid.New(), uuid.New()
	req := Request{Kind: KindLedgerManual, Key: "k1", Payload: map[string]int{"a": 1}, Amount: 15_000_000,
		Summary: "bank → platform_revenue", Reason: "комиссия", AdminID: author}
	for name, r := range map[string]Request{
		"kind":   {Kind: "x", Key: "k", Amount: 1, Summary: "s", Reason: "r", AdminID: author},
		"amount": {Kind: KindDepositTopup, Key: "k", Summary: "s", Reason: "r", AdminID: author},
		"reason": {Kind: KindDepositTopup, Key: "k", Amount: 1, Summary: "s", Reason: " ", AdminID: author},
		"key":    {Kind: KindDepositTopup, Amount: 1, Summary: "s", Reason: "r", AdminID: author},
	} {
		if _, err := NewApproval(uuid.New(), r, time.Hour, now); !errors.Is(err, ErrInvalidApproval) {
			t.Errorf("%s: %v", name, err)
		}
	}
	a, err := NewApproval(uuid.New(), req, 72*time.Hour, now)
	if err != nil || a.Status != StatusPending || !a.ExpiresAt.Equal(now.Add(72*time.Hour)) || string(a.Payload) != `{"a":1}` {
		t.Fatalf("new: %+v %v", a, err)
	}
	if err := a.Approve(author, uuid.New(), now); !errors.Is(err, ErrSameAdmin) {
		t.Fatalf("same admin: %v", err)
	}
	late := a
	if err := late.Approve(other, uuid.New(), now.Add(72*time.Hour)); !errors.Is(err, ErrApprovalExpired) {
		t.Fatalf("expired: %v", err)
	}
	if !late.Expire(now.Add(72*time.Hour)) || late.Status != StatusExpired || late.Expire(now.Add(80*time.Hour)) {
		t.Fatalf("expire: %+v", late)
	}
	res := uuid.New()
	if err := a.Approve(other, res, now.Add(time.Hour)); err != nil || a.Status != StatusApproved || *a.ResultID != res || *a.DecidedBy != other {
		t.Fatalf("approve: %+v %v", a, err)
	}
	if err := a.Approve(uuid.New(), res, now.Add(time.Hour)); !errors.Is(err, ErrApprovalDecided) {
		t.Fatalf("decided: %v", err)
	}
	if err := a.Reject(other, "нет", now); !errors.Is(err, ErrApprovalDecided) {
		t.Fatalf("reject decided: %v", err)
	}
	b, _ := NewApproval(uuid.New(), req, time.Hour, now)
	if err := b.Reject(author, "", now); !errors.Is(err, ErrInvalidApproval) {
		t.Fatalf("reject without reason: %v", err)
	}
	if err := b.Reject(author, "передумал", now); err != nil || b.Status != StatusRejected || b.DecisionReason != "передумал" {
		t.Fatalf("withdraw: %+v %v", b, err)
	}
}
