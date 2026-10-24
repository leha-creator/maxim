// Package domain — правила модуля администратора (этап «API администратора и отчётность ОРД»):
// второе подтверждение ручных денежных операций (R-2). Модуль не пишет чужие таблицы: операцию
// исполняет владелец через порт Executor (R-3).
package domain

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Kind — вид операции, ждущей подтверждения.
type Kind string

const (
	KindLedgerManual   Kind = "ledger_manual"
	KindDepositTopup   Kind = "deposit_topup"
	KindPayoutMarkPaid Kind = "payout_mark_paid"
)

// Valid — известный вид.
func (k Kind) Valid() bool {
	switch k {
	case KindLedgerManual, KindDepositTopup, KindPayoutMarkPaid:
		return true
	}
	return false
}

// Status — состояние запроса.
type Status string

const (
	StatusPending  Status = "pending"
	StatusApproved Status = "approved"
	StatusRejected Status = "rejected"
	StatusExpired  Status = "expired"
)

// Valid — известное состояние.
func (s Status) Valid() bool {
	switch s {
	case StatusPending, StatusApproved, StatusRejected, StatusExpired:
		return true
	}
	return false
}

// Ошибки правил подтверждения.
var (
	// ErrSameAdmin — автор запроса подтверждает сам (409 same_admin).
	ErrSameAdmin = errors.New("admin: approval by the requesting admin")
	// ErrApprovalDecided — запрос уже решён (409 approval_decided).
	ErrApprovalDecided = errors.New("admin: approval is already decided")
	// ErrApprovalExpired — запрос старше срока (409 approval_expired).
	ErrApprovalExpired = errors.New("admin: approval is expired")
	// ErrInvalidApproval — неполный запрос (вид, сумма, основание, ключ).
	ErrInvalidApproval = errors.New("admin: invalid approval request")
	// ErrNotFound — запроса нет.
	ErrNotFound = errors.New("admin: not found")
	// ErrDuplicateKey — запрос с тем же ключом вставлен одновременно (409 idempotency_conflict).
	ErrDuplicateKey = errors.New("admin: duplicate idempotency key")
	// ErrNoExecutor — для вида не подключён исполнитель (ошибка сборки).
	ErrNoExecutor = errors.New("admin: no executor for approval kind")
)

// Пределы полей (миграция 00014).
const (
	MaxSummary = 500
	MaxReason  = 1000
	MaxKey     = 200
)

// Approval — запрос второго подтверждения (admin_approval).
type Approval struct {
	ID             uuid.UUID
	Kind           Kind
	Payload        json.RawMessage
	Amount         int64
	Summary        string
	Reason         string
	IdempotencyKey string
	// ObjectID — выплата для payout_mark_paid (из Payload, для ответа).
	ObjectID       *uuid.UUID
	RequestedBy    uuid.UUID
	Status         Status
	DecidedBy      *uuid.UUID
	DecidedAt      *time.Time
	DecisionReason string
	ResultID       *uuid.UUID
	ExpiresAt      time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Request — что попросил исполнить администратор.
type Request struct {
	Kind     Kind
	Key      string
	Payload  any
	Amount   int64
	Summary  string
	Reason   string
	ObjectID *uuid.UUID
	AdminID  uuid.UUID
}

// NewApproval создаёт запрос в pending со сроком ttl.
func NewApproval(id uuid.UUID, r Request, ttl time.Duration, now time.Time) (Approval, error) {
	reason, summary, key := strings.TrimSpace(r.Reason), strings.TrimSpace(r.Summary), strings.TrimSpace(r.Key)
	switch {
	case !r.Kind.Valid(), r.Amount <= 0, r.AdminID == uuid.Nil,
		reason == "", len([]rune(reason)) > MaxReason,
		summary == "", len([]rune(summary)) > MaxSummary,
		key == "", len(key) > MaxKey, ttl <= 0:
		return Approval{}, ErrInvalidApproval
	}
	payload, err := json.Marshal(r.Payload)
	if err != nil {
		return Approval{}, errors.Join(ErrInvalidApproval, err)
	}
	return Approval{ID: id, Kind: r.Kind, Payload: payload, Amount: r.Amount, Summary: summary, Reason: reason,
		IdempotencyKey: key, ObjectID: r.ObjectID, RequestedBy: r.AdminID, Status: StatusPending,
		ExpiresAt: now.Add(ttl), CreatedAt: now, UpdatedAt: now}, nil
}

// Expired — запрос в pending, срок прошёл.
func (a *Approval) Expired(now time.Time) bool {
	return a.Status == StatusPending && !now.Before(a.ExpiresAt)
}

// Expire переводит просроченный запрос в expired; true — переведён.
func (a *Approval) Expire(now time.Time) bool {
	if !a.Expired(now) {
		return false
	}
	a.Status, a.UpdatedAt = StatusExpired, now
	return true
}

// CanApprove проверяет, что adminID может подтвердить запрос сейчас.
func (a *Approval) CanApprove(adminID uuid.UUID, now time.Time) error {
	switch {
	case a.Status == StatusExpired || a.Expired(now):
		return ErrApprovalExpired
	case a.Status != StatusPending:
		return ErrApprovalDecided
	case a.RequestedBy == adminID:
		return ErrSameAdmin
	}
	return nil
}

// Approve — подтверждение другим администратором; resultID — проводка или выплата исполнения.
func (a *Approval) Approve(adminID, resultID uuid.UUID, now time.Time) error {
	if err := a.CanApprove(adminID, now); err != nil {
		return err
	}
	a.Status, a.DecidedBy, a.DecidedAt, a.ResultID, a.UpdatedAt = StatusApproved, &adminID, &now, &resultID, now
	return nil
}

// Reject — отклонение (любым senior, в том числе автором — отзыв) с основанием.
func (a *Approval) Reject(adminID uuid.UUID, reason string, now time.Time) error {
	reason = strings.TrimSpace(reason)
	if reason == "" || len([]rune(reason)) > MaxReason {
		return ErrInvalidApproval
	}
	switch {
	case a.Status == StatusExpired || a.Expired(now):
		return ErrApprovalExpired
	case a.Status != StatusPending:
		return ErrApprovalDecided
	}
	a.Status, a.DecidedBy, a.DecidedAt, a.DecisionReason, a.UpdatedAt = StatusRejected, &adminID, &now, reason, now
	return nil
}

// Page — страница списка.
type Page struct {
	Offset, Limit int
}

// Store — хранилище запросов; методы работают в транзакции из context.
type Store interface {
	Insert(ctx context.Context, a Approval) error
	ByKey(ctx context.Context, key string) (Approval, error)
	GetForUpdate(ctx context.Context, id uuid.UUID) (Approval, error)
	Update(ctx context.Context, a Approval) error
	// ExpireBefore переводит pending со сроком до now в expired; возвращает переведённые.
	ExpireBefore(ctx context.Context, now time.Time) ([]Approval, error)
	// List — по состоянию (nil — все): pending старые первыми, остальные — новые первыми.
	List(ctx context.Context, status *Status, p Page) ([]Approval, int, error)
	CountPending(ctx context.Context) (int, error)
}

// Executor исполняет подтверждённую операцию своего вида в транзакции подтверждения;
// возвращает идентификатор результата (проводка, выплата).
type Executor interface {
	Execute(ctx context.Context, a Approval, approverID uuid.UUID) (uuid.UUID, error)
}
