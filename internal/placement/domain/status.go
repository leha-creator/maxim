// Package domain — правила модуля размещений (03, 04 §5, 06 §3): корзина, оформление
// заказа, размещение и его машина состояний P-01 … P-18, таймеры, выбор времени
// публикации, производное состояние заказа. Без базы, сети и журнала.
package domain

import "slices"

// Status — состояние размещения (03 §3, 04 §9).
type Status string

const (
	// StatusNone — размещения ещё нет: исходное состояние P-01.
	StatusNone             Status = ""
	StatusCreated          Status = "created"
	StatusAwaitingBlogger  Status = "awaiting_blogger"
	StatusAwaitingSchedule Status = "awaiting_schedule"
	StatusScheduled        Status = "scheduled"
	StatusPublished        Status = "published"
	StatusRemoved          Status = "removed"
	StatusDisputed         Status = "disputed"
	StatusAccepted         Status = "accepted"
	StatusPaid             Status = "paid"
	StatusDeclined         Status = "declined"
	StatusExpired          Status = "expired"
	StatusCancelled        Status = "cancelled"
	StatusFailed           Status = "failed"
	StatusRefunded         Status = "refunded"
)

// Statuses — все значения перечисления в порядке 04 §9.
var Statuses = []Status{
	StatusCreated, StatusAwaitingBlogger, StatusAwaitingSchedule, StatusScheduled, StatusPublished, StatusRemoved,
	StatusDisputed, StatusAccepted, StatusPaid, StatusDeclined, StatusExpired, StatusCancelled, StatusFailed, StatusRefunded,
}

// terminal — конечные состояния (03 §3). Выход из них невозможен (I-04).
var terminal = []Status{StatusPaid, StatusDeclined, StatusExpired, StatusCancelled, StatusFailed, StatusRefunded}

// Terminal — состояние конечное.
func (s Status) Terminal() bool { return slices.Contains(terminal, s) }

// ReleasesHold — в состоянии удержание снято полностью в пользу рекламодателя:
// все конечные, кроме paid (03 §3).
func (s Status) ReleasesHold() bool { return s.Terminal() && s != StatusPaid }

// BeforePublication — размещение ещё не опубликовано и может вернуться в created (P-11)
// или ждёт версии в created.
func (s Status) BeforePublication() bool {
	switch s {
	case StatusCreated, StatusAwaitingBlogger, StatusAwaitingSchedule, StatusScheduled:
		return true
	}
	return false
}

// ActorType — кто запускает переход (04 §9, actor_type).
type ActorType string

const (
	ActorAdvertiser ActorType = "advertiser"
	ActorBlogger    ActorType = "blogger"
	ActorAdmin      ActorType = "admin"
	ActorSystem     ActorType = "system"
)

// ActorTypes — все значения перечисления в порядке 04 §9.
var ActorTypes = []ActorType{ActorAdvertiser, ActorBlogger, ActorAdmin, ActorSystem}

// Причины перехода (placement.status_reason, placement_state_history.reason_code, 04 §9).
const (
	ReasonAdvertiserCancel   = "advertiser_cancel"
	ReasonAdminCancel        = "admin_cancel"
	ReasonChannelUnavailable = "channel_unavailable"
	ReasonWindowElapsed      = "window_elapsed"
	ReasonBloggerDeclined    = "blogger_declined"
	ReasonAcceptTimeout      = "accept_timeout"
	ReasonScheduleTimeout    = "schedule_timeout"
	ReasonPublishError       = "publish_error"
	ReasonPreconditionFailed = "precondition_failed"
	ReasonNewPostVersion     = "new_post_version"
	ReasonErirRejected       = "erir_rejected"
	ReasonAutoAcceptance     = "auto_acceptance"
)

// Reasons — все причины перехода в порядке 04 §9.
var Reasons = []string{
	ReasonAdvertiserCancel, ReasonAdminCancel, ReasonChannelUnavailable, ReasonWindowElapsed, ReasonBloggerDeclined,
	ReasonAcceptTimeout, ReasonScheduleTimeout, ReasonPublishError, ReasonPreconditionFailed, ReasonNewPostVersion,
	ReasonErirRejected, ReasonAutoAcceptance,
}

// OrderStatus — производное состояние заказа (03 §1.2, 04 §9).
type OrderStatus string

const (
	OrderModeration OrderStatus = "moderation"
	OrderRejected   OrderStatus = "rejected"
	OrderMarking    OrderStatus = "marking"
	OrderInProgress OrderStatus = "in_progress"
	OrderCompleted  OrderStatus = "completed"
	OrderCancelled  OrderStatus = "cancelled"
)

// OrderStatuses — все значения перечисления в порядке 04 §9.
var OrderStatuses = []OrderStatus{OrderModeration, OrderRejected, OrderMarking, OrderInProgress, OrderCompleted, OrderCancelled}

// Closed — заказ завершён: кампания уходит в archived (R-19).
func (s OrderStatus) Closed() bool { return s == OrderCompleted || s == OrderCancelled }

// VersionStatus — состояние версии поста (03 §1.1) на языке модуля размещений.
type VersionStatus string

const (
	VersionPendingModeration VersionStatus = "pending_moderation"
	VersionRejected          VersionStatus = "rejected"
	VersionApproved          VersionStatus = "approved"
	VersionReady             VersionStatus = "ready"
)
