package domain

import (
	"strconv"
	"time"

	"github.com/google/uuid"
)

// TimerCode — номер таймера размещения (03 §5).
type TimerCode string

const (
	// TimerAccept — T-01: принятие блогером → P-05.
	TimerAccept TimerCode = "T-01"
	// TimerSchedule — T-13: планирование публикации → P-18.
	TimerSchedule TimerCode = "T-13"
	// TimerWindow — T-14: окно дат истекает, пока размещение в created → P-06 window_elapsed (R-11).
	TimerWindow TimerCode = "T-14"
	// TimerClaim — T-05: окно претензий после снятия → P-13 (этап «Приёмка», R-14).
	TimerClaim TimerCode = "T-05"
	// TimerDispute — T-06: срок решения по спору → оповещение, состояние не меняется (R-11).
	TimerDispute TimerCode = "T-06"
)

// Виды заданий модуля (07 §2).
const (
	KindPlacementTimer   = "placement_timer"
	KindPublishPlacement = "publish_placement"
	KindRemovePlacement  = "remove_placement"
	// KindReconcilePublications — сверка публикаций раз в T-09 (07 §3, R-12).
	KindReconcilePublications = "reconcile_publications"
	// KindCheckPublication — проверка одной живой публикации по сверке (R-12).
	KindCheckPublication = "check_publication"
	// KindPlacementReminder — напоминание блогеру о сроке ответа T-01 (R-8 этапа уведомлений).
	KindPlacementReminder = "placement_reminder"
)

// Timing — сроки и правила времени (A-10, 07 §8); значения — из конфигурации.
type Timing struct {
	// AcceptTimeout — T-01 (PLACEMENT_ACCEPT_TIMEOUT, 24 ч).
	AcceptTimeout time.Duration
	// ScheduleTimeout — T-13 (PLACEMENT_SCHEDULE_TIMEOUT, 24 ч).
	ScheduleTimeout time.Duration
	// WindowGuard — запас до конца окна (PLACEMENT_WINDOW_GUARD, 1 ч): P-02, T-01, T-14, оплата.
	WindowGuard time.Duration
	// MinLead — publish_at не раньше now + MinLead; перенос — пока до publish_at больше MinLead (PLACEMENT_MIN_LEAD, 1 ч).
	MinLead time.Duration
	// SlotStep — кратность publish_at (PLACEMENT_SLOT_STEP, 5 мин).
	SlotStep time.Duration
	// ReeditGuard — правка поста отменяет размещения, у которых до конца окна меньше (PLACEMENT_REEDIT_GUARD, 24 ч).
	ReeditGuard time.Duration
}

// DefaultTiming — значения по умолчанию (03 §5, R-10, R-13, R-15).
var DefaultTiming = Timing{
	AcceptTimeout: 24 * time.Hour, ScheduleTimeout: 24 * time.Hour, WindowGuard: time.Hour,
	MinLead: time.Hour, SlotStep: 5 * time.Minute, ReeditGuard: 24 * time.Hour,
}

// WithDefaults — значения по умолчанию вместо нулевых.
func (t Timing) WithDefaults() Timing {
	d := DefaultTiming
	if t.AcceptTimeout == 0 {
		t.AcceptTimeout = d.AcceptTimeout
	}
	if t.ScheduleTimeout == 0 {
		t.ScheduleTimeout = d.ScheduleTimeout
	}
	if t.WindowGuard == 0 {
		t.WindowGuard = d.WindowGuard
	}
	if t.MinLead == 0 {
		t.MinLead = d.MinLead
	}
	if t.SlotStep == 0 {
		t.SlotStep = d.SlotStep
	}
	if t.ReeditGuard == 0 {
		t.ReeditGuard = d.ReeditGuard
	}
	return t
}

// AcceptDeadline — срок T-01: 24 ч от P-02, но не позже чем за WindowGuard до конца окна (R-10).
func AcceptDeadline(sentAt time.Time, w Window, t Timing) time.Time {
	return earliest(sentAt.Add(t.AcceptTimeout), w.End.Add(-t.WindowGuard))
}

// ScheduleDeadline — срок T-13: 24 ч от P-03, но не позже начала последнего слота окна (R-10, D-15).
func ScheduleDeadline(acceptedAt time.Time, w Window, t Timing) time.Time {
	return earliest(acceptedAt.Add(t.ScheduleTimeout), w.LastSlot(t.SlotStep))
}

// WindowDeadline — срок T-14: размещение в created отменяется за WindowGuard до конца окна (R-11).
func WindowDeadline(w Window, t Timing) time.Time { return w.End.Add(-t.WindowGuard) }

func earliest(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

// TimerPayload — полезная нагрузка placement_timer (07 §2): размещение, таймер, ожидаемое состояние.
type TimerPayload struct {
	PlacementID    uuid.UUID `json:"placement_id"`
	Timer          TimerCode `json:"timer"`
	ExpectedStatus Status    `json:"expected_status"`
}

// PlacementPayload — полезная нагрузка publish_placement и remove_placement (R-12).
type PlacementPayload struct {
	PlacementID uuid.UUID `json:"placement_id"`
	// Reason — причина снятия для remove_placement (publication.removal_reason): пусто — по сроку
	// (schedule); refund — полный возврат по спору, admin — admin_remove_post (R-10, R-11 этапа приёмки).
	Reason RemovalReason `json:"reason,omitempty"`
}

// TimerKey — dedupe_key таймера: timer:{placement_id}:{номер} (07 §2).
func TimerKey(id uuid.UUID, code TimerCode) string {
	return "timer:" + id.String() + ":" + string(code)
}

// PublishKey — dedupe_key задания публикации.
func PublishKey(id uuid.UUID) string { return "publish:" + id.String() }

// RemoveKey — dedupe_key задания снятия.
func RemoveKey(id uuid.UUID) string { return "remove:" + id.String() }

// TimerJob — задание placement_timer: при срабатывании проверяет, что размещение всё ещё в expected.
func TimerJob(id uuid.UUID, code TimerCode, runAt time.Time, expected Status) Job {
	return Job{Kind: KindPlacementTimer, DedupeKey: TimerKey(id, code), RunAt: runAt,
		Payload: TimerPayload{PlacementID: id, Timer: code, ExpectedStatus: expected}}
}

// PublishJob — задание publish_placement на publish_at (R-12).
func PublishJob(id uuid.UUID, at time.Time) Job {
	return Job{Kind: KindPublishPlacement, DedupeKey: PublishKey(id), RunAt: at, Payload: PlacementPayload{PlacementID: id}}
}

// RemoveJob — задание remove_placement на remove_at (R-12).
func RemoveJob(id uuid.UUID, at time.Time) Job {
	return Job{Kind: KindRemovePlacement, DedupeKey: RemoveKey(id), RunAt: at, Payload: PlacementPayload{PlacementID: id}}
}

// RemoveNowJob — снятие живой публикации сейчас с причиной reason (refund, admin): прежнее задание
// снятия отменяется в той же транзакции (R-10, R-11 этапа приёмки).
func RemoveNowJob(id uuid.UUID, reason RemovalReason, now time.Time) Job {
	return Job{Kind: KindRemovePlacement, DedupeKey: RemoveKey(id), RunAt: now, Payload: PlacementPayload{PlacementID: id, Reason: reason}}
}

// CheckPublicationPayload — полезная нагрузка check_publication.
type CheckPublicationPayload struct {
	PublicationID uuid.UUID `json:"publication_id"`
}

// CheckPublicationKey — dedupe_key проверки публикации: одна живая на публикацию (R-12).
func CheckPublicationKey(id uuid.UUID) string { return "check_publication:" + id.String() }

// CheckPublicationJob — задание check_publication на runAt.
func CheckPublicationJob(id uuid.UUID, runAt time.Time) Job {
	return Job{Kind: KindCheckPublication, DedupeKey: CheckPublicationKey(id), RunAt: runAt,
		Payload: CheckPublicationPayload{PublicationID: id}}
}

// allJobKeys — все задания размещения до публикации: таймеры, публикация и снятие.
func allJobKeys(id uuid.UUID) []string {
	return []string{TimerKey(id, TimerAccept), TimerKey(id, TimerSchedule), TimerKey(id, TimerWindow), PublishKey(id), RemoveKey(id), ReminderKey(id)}
}

// ReminderLead — за сколько до T-01 блогеру напоминают о запросе (R-8 этапа уведомлений).
const ReminderLead = 3 * time.Hour

// ReminderKey — dedupe_key напоминания о T-01; он же ключ повтора уведомления.
func ReminderKey(id uuid.UUID) string { return "reminder:" + id.String() + ":T-01" }

// ReminderJob — напоминание за ReminderLead до срока ответа; срок ближе — без напоминания.
func ReminderJob(id uuid.UUID, deadline, now time.Time) (Job, bool) {
	at := deadline.Add(-ReminderLead)
	if !at.After(now) {
		return Job{}, false
	}
	return Job{Kind: KindPlacementReminder, DedupeKey: ReminderKey(id), RunAt: at, Payload: PlacementPayload{PlacementID: id}}, true
}

// AcceptReminder — уведомление-напоминание блогеру, если запрос всё ещё ждёт ответа и срок не
// прошёл; иначе false (напоминание устарело).
func (p *Placement) AcceptReminder(now time.Time) (NoticeTo, bool) {
	if p.Status != StatusAwaitingBlogger || p.AcceptDeadlineAt == nil || !p.AcceptDeadlineAt.After(now) {
		return NoticeTo{}, false
	}
	// Ключ повтора с концом срока: после P-11 и нового P-02 напоминание о новом сроке не
	// совпадает с прежним (R-10).
	key := ReminderKey(p.ID) + ":" + strconv.FormatInt(p.AcceptDeadlineAt.Unix(), 10)
	return NoticeTo{Template: NoticeAcceptReminder, Role: ActorBlogger, Key: key}, true
}
