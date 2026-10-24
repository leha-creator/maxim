package domain

import (
	"time"

	"github.com/google/uuid"
)

// RemovalReason — почему снята публикация (04 §5, publication.removal_reason).
type RemovalReason string

const (
	// RemovalSchedule — бот платформы удалил пост по сроку (S-22).
	RemovalSchedule RemovalReason = "schedule"
	// RemovalExternal — пост удалил не бот платформы (S-26).
	RemovalExternal RemovalReason = "external"
	RemovalAdmin    RemovalReason = "admin"
	RemovalRefund   RemovalReason = "refund"
)

// SentMessage — сообщение, которое MAX создал по нашему запросу (S-21 шаг 4).
type SentMessage struct {
	ID  string
	URL string
	// At — время сообщения по MAX; нулевое — MAX его не вернул.
	At time.Time
	// ContentHash — хеш сообщения в том виде, в каком его хранит MAX (R-6).
	ContentHash string
}

// ChannelMessage — сообщение канала, прочитанное из MAX. Текста нет: он не хранится и не
// пишется в журнал (07 §5.4, R-13).
type ChannelMessage struct {
	ID string
	At time.Time
	// SenderID — автор; 0 — пост от имени канала (R-8).
	SenderID    int64
	ContentHash string
	// Views — stat.views; nil, если MAX поле не вернул (Q-08).
	Views *int
}

// Publication — сообщение в канале по размещению (04 §5). У размещения не больше одной (I-11).
type Publication struct {
	ID           uuid.UUID
	PlacementID  uuid.UUID
	ChannelID    uuid.UUID
	MaxMessageID string
	URL          string
	PublishedAt  time.Time
	// RemovedAt — снята; при своём удалении записывается до вызова MAX (05 §2.6 шаг 2, R-7).
	RemovedAt      *time.Time
	RemovalReason  RemovalReason
	RemovalOverdue bool
	Views          *int
	ViewsCheckedAt *time.Time
	ContentHash    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// NewPublication — публикация по ответу MAX; время — из ответа или publishedAt (P-08).
func NewPublication(id uuid.UUID, p *Placement, msg SentMessage, publishedAt, now time.Time) Publication {
	return Publication{
		ID: id, PlacementID: p.ID, ChannelID: p.ChannelID, MaxMessageID: msg.ID, URL: msg.URL,
		PublishedAt: publishedAt, ContentHash: msg.ContentHash, CreatedAt: now, UpdatedAt: now,
	}
}

// Live — публикация в канале: не снята и снятие не начато.
func (pub Publication) Live() bool { return pub.RemovedAt == nil }

// MarkRemoving записывает намерение удалить по сроку до вызова MAX: так своё удаление
// отличается от чужого в событии message_removed (05 §2.2).
func (pub *Publication) MarkRemoving(now time.Time) {
	pub.RemovedAt, pub.RemovalReason, pub.UpdatedAt = &now, RemovalSchedule, now
}

// MarkRemovingFor — то же с причиной reason (refund, admin); пустая — schedule.
func (pub *Publication) MarkRemovingFor(reason RemovalReason, now time.Time) {
	if reason == "" {
		reason = RemovalSchedule
	}
	pub.RemovedAt, pub.RemovalReason, pub.UpdatedAt = &now, reason, now
}

// RollbackRemoving — удаление не удалось: намерение снято, публикация removal_overdue (05 §2.6 шаг 4).
func (pub *Publication) RollbackRemoving(now time.Time) {
	pub.RemovedAt, pub.RemovalReason, pub.RemovalOverdue, pub.UpdatedAt = nil, "", true, now
}

// MarkRemoved — удаление по сроку подтверждено MAX.
func (pub *Publication) MarkRemoved(now time.Time) {
	pub.RemovalOverdue, pub.UpdatedAt = false, now
}

// MarkRemovedExternally — пост удалил не бот платформы (S-26): время — когда это произошло.
func (pub *Publication) MarkRemovedExternally(at, now time.Time) {
	pub.RemovedAt, pub.RemovalReason, pub.RemovalOverdue, pub.UpdatedAt = &at, RemovalExternal, false, now
}

// UpdateViews сохраняет прочитанные просмотры; nil — MAX поле не вернул, прежнее значение остаётся.
func (pub *Publication) UpdateViews(views *int, now time.Time) {
	if views != nil {
		v := *views
		pub.Views = &v
	}
	pub.ViewsCheckedAt, pub.UpdatedAt = &now, now
}

// ViewsDue — пора перечитать просмотры: не чаще раза в every на публикацию (03 §8).
func (pub Publication) ViewsDue(now time.Time, every time.Duration) bool {
	return pub.ViewsCheckedAt == nil || now.Sub(*pub.ViewsCheckedAt) >= every
}
