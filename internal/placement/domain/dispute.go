package domain

import (
	"time"

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

// Dispute — спор по размещению (04 §5). Поля решения пишет этап «Приёмка, споры и возвраты».
type Dispute struct {
	ID             uuid.UUID
	PlacementID    uuid.UUID
	Reason         string
	OpenedBy       OpenedBy
	ViolationID    *uuid.UUID
	DeadlineAt     time.Time
	SuggestedShare *int
	Resolution     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
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
