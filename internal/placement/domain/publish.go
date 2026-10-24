package domain

import (
	"time"

	"github.com/google/uuid"
)

// ControlRules — сроки публикации, снятия и контроля (03 §5, 07 §8); значения — из конфигурации.
type ControlRules struct {
	// PublishRetryWindow — T-03: повторы публикации от publish_at (PUBLISH_RETRY_WINDOW, 30 мин).
	PublishRetryWindow time.Duration
	// PublishRetryInterval — пауза между попытками публикации (PUBLISH_RETRY_INTERVAL, 30 с).
	PublishRetryInterval time.Duration
	// PublishFindLookback — сколько назад искать своё сообщение после неизвестного исхода (R-5).
	PublishFindLookback time.Duration
	// MaxImages — предел изображений в посте (PUBLISH_MAX_IMAGES, A-23).
	MaxImages int
	// RemoveRetryWindow — T-10: повторы снятия от remove_at (REMOVE_RETRY_WINDOW, 24 ч).
	RemoveRetryWindow time.Duration
	// ClaimWindow — T-05: окно претензий после снятия (PLACEMENT_CLAIM_WINDOW, 24 ч).
	ClaimWindow time.Duration
	// DisputeDeadline — T-06: срок решения по спору (DISPUTE_DEADLINE, 72 ч).
	DisputeDeadline time.Duration
	// TopViolationShare — предложение доли при top_violated (DISPUTE_TOP_VIOLATION_SHARE, 5000).
	TopViolationShare int
	// RatingPenaltyRefund, RatingPenaltyProportional — снижение рейтинга канала по решению спора
	// full_refund и proportional (A-08, DISPUTE_RATING_PENALTY_*). Нуль — без санкции: значения
	// по умолчанию задаёт конфигурация, WithDefaults их не подставляет.
	RatingPenaltyRefund       int
	RatingPenaltyProportional int
	// ReconcileInterval — T-09 (RECONCILE_INTERVAL, 5 мин, A-27).
	ReconcileInterval time.Duration
	// ScheduledGrace — scheduled без публикации дольше этого после publish_at проверяется сверкой (5 мин).
	ScheduledGrace time.Duration
	// ViewsRefresh — просмотры перечитываются не чаще (PUBLICATION_VIEWS_REFRESH, 1 ч).
	ViewsRefresh time.Duration
}

// DefaultControlRules — значения по умолчанию (03 §5, 05 §2.8, R-4 … R-12).
var DefaultControlRules = ControlRules{
	PublishRetryWindow: 30 * time.Minute, PublishRetryInterval: 30 * time.Second, PublishFindLookback: 40 * time.Minute,
	MaxImages: 10, RemoveRetryWindow: 24 * time.Hour, ClaimWindow: 24 * time.Hour, DisputeDeadline: 72 * time.Hour,
	TopViolationShare: 5000, ReconcileInterval: 5 * time.Minute, ScheduledGrace: 5 * time.Minute, ViewsRefresh: time.Hour,
}

// WithDefaults — значения по умолчанию вместо нулевых.
func (r ControlRules) WithDefaults() ControlRules {
	d := DefaultControlRules
	set := func(v *time.Duration, def time.Duration) {
		if *v == 0 {
			*v = def
		}
	}
	set(&r.PublishRetryWindow, d.PublishRetryWindow)
	set(&r.PublishRetryInterval, d.PublishRetryInterval)
	set(&r.PublishFindLookback, d.PublishFindLookback)
	set(&r.RemoveRetryWindow, d.RemoveRetryWindow)
	set(&r.ClaimWindow, d.ClaimWindow)
	set(&r.DisputeDeadline, d.DisputeDeadline)
	set(&r.ReconcileInterval, d.ReconcileInterval)
	set(&r.ScheduledGrace, d.ScheduledGrace)
	set(&r.ViewsRefresh, d.ViewsRefresh)
	if r.MaxImages == 0 {
		r.MaxImages = d.MaxImages
	}
	if r.TopViolationShare == 0 {
		r.TopViolationShare = d.TopViolationShare
	}
	return r
}

// Причины P-09 в reason_text (R-3, R-16).
const (
	PublishChannelNotActive = "channel_not_active"
	PublishVersionNotReady  = "version_not_ready"
	PublishMarkingMismatch  = "marking_mismatch"
	PublishTooManyImages    = "too_many_images"
	PublishDowntime         = "downtime"
	PublishRetriesExhausted = "retries_exhausted"
)

// PreconditionError — предусловие S-21 шаг 1 не выполнено: сразу P-09 precondition_failed.
type PreconditionError struct{ Reason string }

func (e *PreconditionError) Error() string {
	return "placement: publish precondition failed: " + e.Reason
}

// MediaRef — изображение версии поста для публикации (R-16).
type MediaRef struct {
	DerivativeID uuid.UUID
	ContentType  string
	Size         int64
}

// Content — версия поста для публикации (R-1, R-3).
type Content struct {
	VersionID uuid.UUID
	Status    VersionStatus
	Erid      string
	FinalText string
	// MarkingOK — итоговый текст совпал с собранным заново из body и параметров маркировки (I-01).
	MarkingOK bool
	Media     []MediaRef
}

// CheckPublish — предусловия публикации (S-21 шаг 1, I-01, I-08, R-3, R-16).
func CheckPublish(p *Placement, channel ChannelOffer, c Content, r ControlRules) error {
	switch {
	case p.Status != StatusScheduled:
		return &PreconditionError{Reason: string(p.Status)}
	case !channel.Active:
		return &PreconditionError{Reason: PublishChannelNotActive}
	case c.VersionID != p.PostVersionID || c.Status != VersionReady || c.Erid == "" || c.FinalText == "":
		return &PreconditionError{Reason: PublishVersionNotReady}
	case !c.MarkingOK:
		return &PreconditionError{Reason: PublishMarkingMismatch}
	case len(c.Media) > r.MaxImages:
		return &PreconditionError{Reason: PublishTooManyImages}
	}
	return nil
}

// PublishRetry — что делать после неудачной попытки публикации (R-4, T-03): повторить через
// retryAfter или завершить P-09, если окно повторов от publish_at истекло.
func (p *Placement) PublishRetry(now time.Time, r ControlRules) (retryAfter time.Duration, fail bool) {
	if p.PublishAt == nil {
		return 0, true
	}
	deadline := p.PublishAt.Add(r.PublishRetryWindow)
	if !now.Before(deadline) {
		return 0, true
	}
	if left := deadline.Sub(now); left < r.PublishRetryInterval {
		return left, false
	}
	return r.PublishRetryInterval, false
}

// PublishExpired — задание публикации взято позже окна T-03 (07 §2, «После простоя»).
func (p *Placement) PublishExpired(now time.Time, r ControlRules) bool {
	return p.PublishAt != nil && now.Sub(*p.PublishAt) > r.PublishRetryWindow
}

// RemoveRetry — задержка следующей попытки снятия: min(30 с × 2^attempts, 1 ч) до
// remove_at + T-10 (R-7); exhausted — окно истекло, повторов больше нет.
func (p *Placement) RemoveRetry(attempts int, now time.Time, r ControlRules) (retryAfter time.Duration, exhausted bool) {
	base := now
	if p.RemoveAt != nil {
		base = *p.RemoveAt
	}
	deadline := base.Add(r.RemoveRetryWindow)
	if !now.Before(deadline) {
		return 0, true
	}
	d := 30 * time.Second
	for i := 0; i < attempts && d < time.Hour; i++ {
		d *= 2
	}
	d = min(d, time.Hour)
	if left := deadline.Sub(now); left < d {
		d = left
	}
	return d, false
}

// Publish — P-08 (S-21 шаг 4): published_at — время сообщения по MAX, remove_at считается
// от него заново, задание снятия переносится; уведомления обоим.
func (p *Placement) Publish(msg SentMessage, now time.Time) (StateChange, error) {
	change, err := p.Apply(P08Publish, ActorSystem, "", now)
	if err != nil {
		return StateChange{}, err
	}
	at := msg.At
	if at.IsZero() {
		at = now
	}
	at = at.UTC()
	removeAt := at.Add(time.Duration(p.DurationHours) * time.Hour)
	p.PublishedAt, p.RemoveAt = &at, &removeAt
	change.MoveJobs = []JobMove{{DedupeKey: RemoveKey(p.ID), RunAt: removeAt}}
	change.Notices = p.notifyBoth(NoticePublished)
	if msg.URL != "" {
		for i := range change.Notices {
			change.Notices[i].Data = map[string]any{"publication_url": msg.URL}
		}
	}
	return change, nil
}

// FailPublish — P-09: публикация не состоялась; задание снятия отменяется, удержание снимается.
// text — уточнение причины для истории (код ошибки MAX, непрошедшее предусловие).
func (p *Placement) FailPublish(reason, text string, now time.Time) (StateChange, error) {
	change, err := p.Apply(P09PublishFailed, ActorSystem, reason, now)
	if err != nil {
		return StateChange{}, err
	}
	change.ReasonText = text
	change.CancelJobs = []string{PublishKey(p.ID), RemoveKey(p.ID)}
	// Публикация не состоялась — администратору тоже (R-9 этапа уведомлений).
	change.Notices = append(p.notifyBoth(NoticeFailed), NoticeTo{Template: NoticeFailed, Role: ActorAdmin})
	return change, nil
}

// Remove — P-10 (S-22 шаг 3): итоговые просмотры, окно претензий и T-05 (R-14).
func (p *Placement) Remove(views *int, removedAt time.Time, r ControlRules) (StateChange, error) {
	change, err := p.Apply(P10Remove, ActorSystem, "", removedAt)
	if err != nil {
		return StateChange{}, err
	}
	claim := removedAt.Add(r.ClaimWindow)
	p.RemovedAt, p.ClaimDeadlineAt = &removedAt, &claim
	if views != nil {
		v := *views
		p.FinalViews = &v
	}
	change.Jobs = []Job{TimerJob(p.ID, TimerClaim, claim, StatusRemoved)}
	change.Notices = p.notifyBoth(NoticeRemoved)
	return change, nil
}

// OpenDispute — P-14 системой по нарушению (R-11): T-05 отменяется, ставится T-06.
func (p *Placement) OpenDispute(kind ViolationKind, now time.Time, r ControlRules) (StateChange, error) {
	change, err := p.Apply(P14OpenDispute, ActorSystem, string(kind), now)
	if err != nil {
		return StateChange{}, err
	}
	change.CancelJobs = []string{TimerKey(p.ID, TimerClaim)}
	change.Jobs = []Job{TimerJob(p.ID, TimerDispute, now.Add(r.DisputeDeadline), StatusDisputed)}
	change.Notices = append(p.notifyBoth(NoticeDisputed), NoticeTo{Template: NoticeDisputed, Role: ActorAdmin})
	return change, nil
}
