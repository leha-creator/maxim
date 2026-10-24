package domain

import (
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// ClaimReason — причина претензии рекламодателя (04 §9, S-08).
type ClaimReason string

const (
	ClaimRemovedEarly    ClaimReason = "removed_early"
	ClaimTopViolated     ClaimReason = "top_violated"
	ClaimContentMismatch ClaimReason = "content_mismatch"
	ClaimOther           ClaimReason = "other"
)

// ClaimReasons — все причины в порядке 04 §9.
var ClaimReasons = []ClaimReason{ClaimRemovedEarly, ClaimTopViolated, ClaimContentMismatch, ClaimOther}

// ClaimTextMax — предел описания претензии в символах (S-08).
const ClaimTextMax = 2000

// Claim — претензия рекламодателя (04 §5). Одна на размещение, неизменяема (R-1 этапа приёмки).
type Claim struct {
	ID          uuid.UUID
	PlacementID uuid.UUID
	Reason      ClaimReason
	// Text — описание рекламодателя; в журнал не пишется.
	Text      string
	CreatedBy uuid.UUID
	CreatedAt time.Time
}

// NewClaim проверяет причину и описание (1…2000 символов после обрезки пробелов).
func NewClaim(id, placementID, userID uuid.UUID, reason ClaimReason, text string, now time.Time) (Claim, error) {
	if !slices.Contains(ClaimReasons, reason) {
		return Claim{}, ErrClaimReason
	}
	text = strings.TrimSpace(text)
	if n := utf8.RuneCountInString(text); n == 0 || n > ClaimTextMax {
		return Claim{}, ErrClaimText
	}
	return Claim{ID: id, PlacementID: placementID, Reason: reason, Text: text, CreatedBy: userID, CreatedAt: now}, nil
}

// ClaimWindowOpen — претензию ещё можно подать (S-08, T-05): из published — всегда, из removed —
// до claim_deadline_at.
func ClaimWindowOpen(p *Placement, now time.Time) bool {
	switch p.Status {
	case StatusPublished:
		return true
	case StatusRemoved:
		return p.ClaimDeadlineAt != nil && now.Before(*p.ClaimDeadlineAt)
	}
	return false
}
