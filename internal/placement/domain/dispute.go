package domain

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ViolationKind — вид нарушения размещения (04 §5, 03 §2).
type ViolationKind string

const (
	ViolationRemovedEarly    ViolationKind = "removed_early"
	ViolationContentModified ViolationKind = "content_modified"
	ViolationTopViolated     ViolationKind = "top_violated"
	ViolationBotAccessLost   ViolationKind = "bot_access_lost"
)

// ViolationKinds — все виды в порядке 04 §5.
var ViolationKinds = []ViolationKind{ViolationRemovedEarly, ViolationContentModified, ViolationTopViolated, ViolationBotAccessLost}

// ViolationSource — кто нашёл нарушение.
type ViolationSource string

const (
	SourceEvent          ViolationSource = "event"
	SourceReconciliation ViolationSource = "reconciliation"
)

// Violation — нарушение, найденное контролем (04 §5). Неизменяемо (R-20).
type Violation struct {
	ID             uuid.UUID
	PlacementID    uuid.UUID
	Kind           ViolationKind
	OccurredAt     time.Time
	Source         ViolationSource
	InboundEventID *uuid.UUID
	// Details — например, идентификатор постороннего сообщения. Текста сообщений нет (07 §5.4).
	Details   map[string]any
	CreatedAt time.Time
}

// OpenedBy — кто открыл спор (04 §5, dispute.opened_by_type).
type OpenedBy string

const (
	OpenedByAdvertiser OpenedBy = "advertiser"
	OpenedBySystem     OpenedBy = "system"
	OpenedByAdmin      OpenedBy = "admin"
)

// DisputeReasonAdmin — причина спора, открытого администратором (03 §6, admin_open_dispute).
const DisputeReasonAdmin = "admin"

// Dispute — спор по размещению (04 §5). Решение (dispute_resolution) — поля этой же записи.
type Dispute struct {
	ID             uuid.UUID
	PlacementID    uuid.UUID
	Reason         string
	OpenedBy       OpenedBy
	ClaimID        *uuid.UUID
	ViolationID    *uuid.UUID
	DeadlineAt     time.Time
	SuggestedShare *int
	Resolution     Resolution
	// PerformanceShare — итоговая доля решения (03 §7).
	PerformanceShare *int
	// ResolutionComment — основание решения администратора; сторонам и в журнал не попадает.
	ResolutionComment string
	ResolvedBy        *uuid.UUID
	ResolvedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Resolved — решение по спору принято.
func (d Dispute) Resolved() bool { return d.Resolution != "" }

// Resolution — решение по спору (03 §4 «Исходы P-15», A-18).
type Resolution string

const (
	ResolutionFullPayout   Resolution = "full_payout"
	ResolutionProportional Resolution = "proportional"
	ResolutionFullRefund   Resolution = "full_refund"
)

// Resolutions — все решения в порядке 03 §4.
var Resolutions = []Resolution{ResolutionFullPayout, ResolutionProportional, ResolutionFullRefund}

// ResolutionCommentMax — предел основания решения в символах.
const ResolutionCommentMax = 2000

// Status — состояние размещения после решения: accepted или refunded.
func (r Resolution) Status() Status {
	if r == ResolutionFullRefund {
		return StatusRefunded
	}
	return StatusAccepted
}

// ResolutionShare — итоговая доля решения (R-8 этапа приёмки): full_payout — 10000, full_refund — 0,
// proportional — share (1…9999), без share — предложение системы; предложения нет или оно вне
// 1…9999 — ErrShareRequired. Доля при full_payout и full_refund, если задана, должна совпадать.
func ResolutionShare(res Resolution, share, suggested *int) (int, error) {
	switch res {
	case ResolutionFullPayout, ResolutionFullRefund:
		want := 10000
		if res == ResolutionFullRefund {
			want = 0
		}
		if share != nil && *share != want {
			return 0, ErrShareOutOfRange
		}
		return want, nil
	case ResolutionProportional:
		v := share
		if v == nil {
			v = suggested
			if v == nil || *v < 1 || *v > 9999 {
				return 0, ErrShareRequired
			}
		}
		if *v < 1 || *v > 9999 {
			return 0, ErrShareOutOfRange
		}
		return *v, nil
	}
	return 0, ErrUnknownResolution
}

// Resolve записывает решение (S-18): окончательное, повтор — ErrDisputeResolved; основание
// обязательно (до 2000 символов).
func (d *Dispute) Resolve(res Resolution, share int, comment string, adminID uuid.UUID, now time.Time) error {
	if d.Resolved() {
		return ErrDisputeResolved
	}
	comment = strings.TrimSpace(comment)
	if n := utf8.RuneCountInString(comment); n == 0 || n > ResolutionCommentMax {
		return ErrResolutionComment
	}
	d.Resolution, d.PerformanceShare, d.ResolutionComment = res, &share, comment
	d.ResolvedBy, d.ResolvedAt, d.UpdatedAt = &adminID, &now, now
	return nil
}

// NewClaimDispute — спор по претензии рекламодателя (S-08, R-1 этапа приёмки).
func NewClaimDispute(id uuid.UUID, c Claim, share *int, now time.Time, r ControlRules) Dispute {
	cid := c.ID
	return Dispute{
		ID: id, PlacementID: c.PlacementID, Reason: string(c.Reason), OpenedBy: OpenedByAdvertiser, ClaimID: &cid,
		DeadlineAt: now.Add(r.DisputeDeadline), SuggestedShare: share, CreatedAt: now, UpdatedAt: now,
	}
}

// NewAdminDispute — спор, открытый администратором (admin_open_dispute, R-12): без предложения доли.
func NewAdminDispute(id, placementID uuid.UUID, now time.Time, r ControlRules) Dispute {
	return Dispute{
		ID: id, PlacementID: placementID, Reason: DisputeReasonAdmin, OpenedBy: OpenedByAdmin,
		DeadlineAt: now.Add(r.DisputeDeadline), CreatedAt: now, UpdatedAt: now,
	}
}

// RatingPenalty — на сколько снижается рейтинг канала по решению (A-08, R-13 этапа приёмки):
// full_refund и proportional — параметры, full_payout — 0.
func RatingPenalty(res Resolution, r ControlRules) int {
	switch res {
	case ResolutionFullRefund:
		return r.RatingPenaltyRefund
	case ResolutionProportional:
		return r.RatingPenaltyProportional
	}
	return 0
}

// NewSystemDispute — спор, который система открывает по нарушению (R-11).
func NewSystemDispute(id uuid.UUID, v Violation, share *int, now time.Time, r ControlRules) Dispute {
	vid := v.ID
	return Dispute{
		ID: id, PlacementID: v.PlacementID, Reason: string(v.Kind), OpenedBy: OpenedBySystem, ViolationID: &vid,
		DeadlineAt: now.Add(r.DisputeDeadline), SuggestedShare: share, CreatedAt: now, UpdatedAt: now,
	}
}

// SuggestedShare — доля, которую система предлагает администратору (03 §7): removed_early —
// доля прошедшего срока, top_violated — параметр; остальные — без предложения.
func SuggestedShare(kind ViolationKind, publishedAt, removedAt, removeAt time.Time, r ControlRules) *int {
	switch kind {
	case ViolationRemovedEarly:
		total := removeAt.Sub(publishedAt)
		if total <= 0 {
			return nil
		}
		done := max(removedAt.Sub(publishedAt), 0)
		share := int(min(int64(10000)*int64(done/time.Second)/int64(total/time.Second), 10000))
		return &share
	case ViolationTopViolated:
		share := r.TopViolationShare
		return &share
	}
	return nil
}
