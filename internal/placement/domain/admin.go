package domain

import (
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// Действия администратора по размещению (этап «API администратора», R-5, R-6, R-10).

var (
	// ErrRemovedAtInvalid — время снятия раньше публикации или в будущем: 422 removed_at.
	ErrRemovedAtInvalid = &RuleError{Field: "removed_at", Reason: "out_of_range"}
	// ErrMessageText — текст сообщения по спору пуст или длиннее 2000 символов.
	ErrMessageText = &RuleError{Field: "text", Reason: "length"}
	// ErrMessageRecipient — адресат сообщения администратора неизвестен.
	ErrMessageRecipient = &RuleError{Field: "to", Reason: "unknown"}
)

// MaxMessageText — предел сообщения по спору (dispute_message.text).
const MaxMessageText = 2000

// CancelByAdmin — P-06 администратором (admin_cancel, R-5): до публикации, причина admin_cancel,
// основание — в reason_text истории; удержание снимается, задания и таймеры отменяются.
func (p *Placement) CancelByAdmin(adminID uuid.UUID, text string, now time.Time) (StateChange, error) {
	if text == "" {
		return StateChange{}, ErrReasonRequired
	}
	change, err := p.Cancel(ActorAdmin, ReasonAdminCancel, now)
	if err != nil {
		return StateChange{}, err
	}
	change.UserID, change.ReasonText = &adminID, text
	return change, nil
}

// MarkRemovedByAdmin — P-10 администратором (admin_mark_removed, R-6): MAX не подтвердил снятие,
// администратор проверил канал вручную. Время снятия — не раньше публикации и не позже now;
// окно претензий и T-05 — от него. Нарушение не фиксируется.
func (p *Placement) MarkRemovedByAdmin(adminID uuid.UUID, text string, views *int, publishedAt, removedAt, now time.Time,
	r ControlRules) (StateChange, error) {
	if text == "" {
		return StateChange{}, ErrReasonRequired
	}
	if removedAt.Before(publishedAt) || removedAt.After(now) {
		return StateChange{}, ErrRemovedAtInvalid
	}
	change, err := p.Apply(P10Remove, ActorAdmin, string(RemovalAdmin), now)
	if err != nil {
		return StateChange{}, err
	}
	claim := removedAt.Add(r.ClaimWindow)
	p.RemovedAt, p.ClaimDeadlineAt = &removedAt, &claim
	if views != nil {
		v := *views
		p.FinalViews = &v
	}
	change.UserID, change.ReasonText = &adminID, text
	change.CancelJobs = []string{RemoveKey(p.ID)}
	change.Jobs = []Job{TimerJob(p.ID, TimerClaim, claim, StatusRemoved)}
	change.Notices = p.notifyBoth(NoticeRemoved)
	return change, nil
}

// MarkRemovedByAdmin — публикация снята в указанное время по решению администратора (R-6).
func (pub *Publication) MarkRemovedByAdmin(removedAt, now time.Time) {
	pub.RemovedAt, pub.RemovalReason, pub.RemovalOverdue, pub.UpdatedAt = &removedAt, RemovalAdmin, false, now
}

// AdminAction — действие администратора, доступное в карточке размещения.
type AdminAction string

const (
	AdminActionCancel         AdminAction = "cancel"
	AdminActionMarkRemoved    AdminAction = "mark_removed"
	AdminActionOpenDispute    AdminAction = "open_dispute"
	AdminActionRemovePost     AdminAction = "remove_post"
	AdminActionResolveDispute AdminAction = "resolve_dispute"
	AdminActionRetryPayout    AdminAction = "retry_payout"
)

// AdminActions — действия по состоянию размещения, публикации и роли (матрица R-1): отмена,
// отметка снятия и повтор выплаты — только senior; спор и удаление поста — обе роли.
func AdminActions(p *Placement, pub *Publication, d *Dispute, payoutRetryable, senior bool) []AdminAction {
	out := []AdminAction{}
	if senior && Allowed(P06Cancel, ActorAdmin, p.Status) {
		out = append(out, AdminActionCancel)
	}
	live := pub != nil && pub.Live()
	if senior && p.Status == StatusPublished && pub != nil && (live || pub.RemovalOverdue) {
		out = append(out, AdminActionMarkRemoved)
	}
	if Allowed(P14OpenDispute, ActorAdmin, p.Status) && d == nil {
		out = append(out, AdminActionOpenDispute)
	}
	if live && (p.Status == StatusPublished || p.Status == StatusDisputed) {
		out = append(out, AdminActionRemovePost)
	}
	if p.Status == StatusDisputed && d != nil && !d.Resolved() {
		out = append(out, AdminActionResolveDispute)
	}
	if senior && payoutRetryable {
		out = append(out, AdminActionRetryPayout)
	}
	return out
}

// MessageAuthor — автор сообщения по спору (dispute_message.author_type).
type MessageAuthor string

const (
	AuthorAdmin      MessageAuthor = "admin"
	AuthorAdvertiser MessageAuthor = "advertiser"
	AuthorBlogger    MessageAuthor = "blogger"
)

// MessageRecipient — адресат сообщения администратора.
type MessageRecipient string

const (
	RecipientAdvertiser MessageRecipient = "advertiser"
	RecipientBlogger    MessageRecipient = "blogger"
	RecipientBoth       MessageRecipient = "both"
)

// DisputeMessage — сообщение по спору (R-9): администратор пишет стороне или обеим, сторона
// отвечает администратору. Только добавление.
type DisputeMessage struct {
	ID           uuid.UUID
	DisputeID    uuid.UUID
	Author       MessageAuthor
	AuthorUserID uuid.UUID
	// Recipient — только у сообщения администратора.
	Recipient MessageRecipient
	Text      string
	CreatedAt time.Time
}

func checkText(text string) error {
	if n := utf8.RuneCountInString(text); n == 0 || n > MaxMessageText {
		return ErrMessageText
	}
	return nil
}

// NewAdminMessage — сообщение администратора адресату to; по решённому спору писать нельзя.
func NewAdminMessage(id uuid.UUID, d Dispute, adminID uuid.UUID, to MessageRecipient, text string, now time.Time) (DisputeMessage, error) {
	switch to {
	case RecipientAdvertiser, RecipientBlogger, RecipientBoth:
	default:
		return DisputeMessage{}, ErrMessageRecipient
	}
	if err := checkText(text); err != nil {
		return DisputeMessage{}, err
	}
	if d.Resolved() {
		return DisputeMessage{}, ErrDisputeResolved
	}
	return DisputeMessage{ID: id, DisputeID: d.ID, Author: AuthorAdmin, AuthorUserID: adminID, Recipient: to, Text: text,
		CreatedAt: now}, nil
}

// NewPartyMessage — ответ стороны администратору; по решённому спору — ErrDisputeResolved.
func NewPartyMessage(id uuid.UUID, d Dispute, author MessageAuthor, userID uuid.UUID, text string, now time.Time) (DisputeMessage, error) {
	if author != AuthorAdvertiser && author != AuthorBlogger {
		return DisputeMessage{}, ErrMessageRecipient
	}
	if err := checkText(text); err != nil {
		return DisputeMessage{}, err
	}
	if d.Resolved() {
		return DisputeMessage{}, ErrDisputeResolved
	}
	return DisputeMessage{ID: id, DisputeID: d.ID, Author: author, AuthorUserID: userID, Text: text, CreatedAt: now}, nil
}

// VisibleTo — сторона видит свои сообщения и сообщения администратора ей или обеим.
func (m DisputeMessage) VisibleTo(side MessageAuthor) bool {
	if m.Author == side {
		return true
	}
	return m.Author == AuthorAdmin && (m.Recipient == RecipientBoth || string(m.Recipient) == string(side))
}
