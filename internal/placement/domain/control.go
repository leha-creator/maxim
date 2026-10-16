package domain

import (
	"time"

	"github.com/google/uuid"
)

// Контроль размещения (03 §8): публикация на месте, содержание не изменено, «часы в топе».
// Функции только решают; нарушения сохраняет и переводит в спор app (R-10).

// ChannelPost — сообщение, появившееся в канале (событие message_created, R-9).
type ChannelPost struct {
	ChannelID uuid.UUID
	MaxChatID int64
	MessageID string
	// SenderID — автор; 0 — пост от имени канала, считается чужим (R-8).
	SenderID       int64
	At             time.Time
	InboundEventID *uuid.UUID
}

// PublishedTop — интервал «часов в топе» опубликованного размещения [published_at, + top_hours) (T-08).
func (p *Placement) PublishedTop() (Interval, bool) {
	if p.PublishedAt == nil {
		return Interval{}, false
	}
	return TopInterval(*p.PublishedAt, p.TopHours), true
}

// InTop — момент at внутри «часов в топе» опубликованного размещения.
func (p *Placement) InTop(at time.Time) bool {
	top, ok := p.PublishedTop()
	return ok && top.Contains(at)
}

// controlled — размещение под контролем: публикация в канале, состояние published или disputed (03 §8).
func controlled(lp LivePlacement) bool {
	return lp.Publication.Live() && (lp.Placement.Status == StatusPublished || lp.Placement.Status == StatusDisputed)
}

// foreign — сообщение создано не ботом платформы (03 §8): другие рекламные посты бота топ не нарушают.
func foreign(senderID, botUserID int64) bool { return senderID == 0 || senderID != botUserID }

// ClassifyChannelPost — чужое сообщение во время топа размещений канала → top_violated (S-27).
// Время нарушения — время сообщения по MAX (R-10).
func ClassifyChannelPost(live []LivePlacement, post ChannelPost, botUserID int64) []Violation {
	if !foreign(post.SenderID, botUserID) {
		return nil
	}
	var out []Violation
	for _, lp := range live {
		if !controlled(lp) || lp.Publication.MaxMessageID == post.MessageID || !lp.Placement.InTop(post.At) {
			continue
		}
		out = append(out, Violation{
			PlacementID: lp.Placement.ID, Kind: ViolationTopViolated, OccurredAt: post.At, Source: SourceEvent,
			InboundEventID: post.InboundEventID, Details: map[string]any{"message_id": post.MessageID},
		})
	}
	return out
}

// ClassifyRemoved — сообщение публикации удалено (message_removed, S-26). Своё удаление
// записано намерением заранее (05 §2.2) — это не нарушение.
func ClassifyRemoved(pub Publication, at time.Time) (Violation, bool) {
	if !pub.Live() {
		return Violation{}, false
	}
	return Violation{PlacementID: pub.PlacementID, Kind: ViolationRemovedEarly, OccurredAt: at, Source: SourceEvent}, true
}

// ClassifyEdited — сообщение публикации изменено (message_edited, S-26): хеш не совпал (R-6).
func ClassifyEdited(pub Publication, hash string, at time.Time) (Violation, bool) {
	if !pub.Live() || hash == "" || hash == pub.ContentHash {
		return Violation{}, false
	}
	return Violation{PlacementID: pub.PlacementID, Kind: ViolationContentModified, OccurredAt: at, Source: SourceEvent}, true
}

// TopCheckWindow — за какой интервал сверка читает сообщения канала: от публикации до
// текущего момента, но не дальше конца топа; false — топ закончился больше интервала сверки
// назад и уже просмотрен последней проверкой (05 §2.8 шаг 3).
func TopCheckWindow(p *Placement, now time.Time, r ControlRules) (from, to time.Time, ok bool) {
	top, has := p.PublishedTop()
	if !has || !now.After(top.Start) || now.After(top.End.Add(r.ReconcileInterval)) {
		return time.Time{}, time.Time{}, false
	}
	to = now
	if !to.Before(top.End) {
		// Последняя проверка после конца топа: сообщения до его конца ещё не просмотрены.
		to = top.End.Add(-time.Millisecond)
	}
	return top.Start, to, true
}

// ReconcileResult — что нашла сверка по одной публикации.
type ReconcileResult struct {
	Violations []Violation
	// Views — прочитанные просмотры, если их пора сохранить.
	Views       *int
	UpdateViews bool
}

// Reconcile — решение сверки по публикации (05 §2.8): msg — сообщение из GET /messages/{id}
// (nil — не найдено), recent — сообщения канала за TopCheckWindow. topChecked — recent читались.
func Reconcile(lp LivePlacement, msg *ChannelMessage, recent []ChannelMessage, topChecked bool, botUserID int64,
	now time.Time, r ControlRules) ReconcileResult {
	var res ReconcileResult
	if !controlled(lp) {
		return res
	}
	pub := lp.Publication
	if msg == nil {
		res.Violations = append(res.Violations, Violation{PlacementID: pub.PlacementID, Kind: ViolationRemovedEarly,
			OccurredAt: now, Source: SourceReconciliation})
		return res
	}
	if msg.ContentHash != "" && msg.ContentHash != pub.ContentHash {
		res.Violations = append(res.Violations, Violation{PlacementID: pub.PlacementID, Kind: ViolationContentModified,
			OccurredAt: now, Source: SourceReconciliation})
	}
	if topChecked {
		// Самое раннее чужое сообщение в топе: recent — новые первыми.
		var first *ChannelMessage
		for i := range recent {
			m := recent[i]
			if m.ID == pub.MaxMessageID || !foreign(m.SenderID, botUserID) || !lp.Placement.InTop(m.At) {
				continue
			}
			if first == nil || m.At.Before(first.At) {
				first = &recent[i]
			}
		}
		if first != nil {
			res.Violations = append(res.Violations, Violation{PlacementID: pub.PlacementID, Kind: ViolationTopViolated,
				OccurredAt: first.At, Source: SourceReconciliation, Details: map[string]any{"message_id": first.ID}})
		}
	}
	if pub.ViewsDue(now, r.ViewsRefresh) {
		res.Views, res.UpdateViews = msg.Views, true
	}
	return res
}
