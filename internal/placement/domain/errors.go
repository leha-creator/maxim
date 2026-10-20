package domain

import (
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/leha-creator/maxim/internal/platform/money"
)

// RuleError — нарушение правила с машинной причиной: app передаёт её в details.reason
// ответа 422. Сравнивается по указателю (errors.Is).
type RuleError struct {
	// Field — поле запроса, к которому относится причина.
	Field  string
	Reason string
}

func (e *RuleError) Error() string { return "placement: " + e.Field + ": " + e.Reason }

// ConflictError — действие невозможно в текущем состоянии данных: app отвечает 409
// с details.reason.
type ConflictError struct{ Reason string }

func (e *ConflictError) Error() string { return "placement: conflict: " + e.Reason }

// Причины отказа корзины (R-8).
var (
	ErrFormatUnavailable = &RuleError{Field: "format_id", Reason: "format_unavailable"}
	ErrOwnChannel        = &RuleError{Field: "channel_id", Reason: "own_channel"}
	ErrCartFull          = &RuleError{Field: "channel_id", Reason: "cart_full"}
	ErrAlreadyInCart     = &ConflictError{Reason: "already_in_cart"}
	// ErrChannelNotInCatalog — канала нет в каталоге: 404 (R-8).
	ErrChannelNotInCatalog = errors.New("placement: channel is not in catalog")
)

// Причины отказа в выборе времени публикации (R-13).
var (
	ErrPublishAtRequired          = &RuleError{Field: "publish_at", Reason: "publish_at_required"}
	ErrOutsideWindow              = &RuleError{Field: "publish_at", Reason: "outside_window"}
	ErrOutsideTime                = &RuleError{Field: "publish_at", Reason: "outside_time"}
	ErrTooSoon                    = &RuleError{Field: "publish_at", Reason: "too_soon"}
	ErrNotAligned                 = &RuleError{Field: "publish_at", Reason: "not_aligned"}
	ErrChannelUnavailableInterval = &RuleError{Field: "publish_at", Reason: "channel_unavailable_interval"}
	ErrSlotTaken                  = &RuleError{Field: "publish_at", Reason: "slot_taken"}
	ErrUnknownMode                = &RuleError{Field: "mode", Reason: "unknown"}
)

// Причины 409 действий над размещениями и заказами (R-3, R-15, R-17).
var (
	ErrContractorInactive    = &ConflictError{Reason: "contractor_inactive"}
	ErrChannelNotActive      = &ConflictError{Reason: "channel_not_active"}
	ErrPostVersionInProgress = &ConflictError{Reason: "post_version_in_progress"}
	ErrNothingToUpdate       = &ConflictError{Reason: "nothing_to_update"}
	ErrNothingToCancel       = &ConflictError{Reason: "nothing_to_cancel"}
	// ErrCampaignNotDraft — кампания уже оформлена (R-6): 409 conflict.
	ErrCampaignNotDraft = &ConflictError{Reason: "campaign_not_draft"}
)

// ErrVersionNotReady — P-02 без версии поста в ready (I-01 в части этапа).
var ErrVersionNotReady = errors.New("placement: post version is not ready")

// ErrNoActiveHold — P-02 без активного удержания (R-7, I-03): ошибка в коде, а не сценарий.
var ErrNoActiveHold = errors.New("placement: no active hold")

// IncompleteError — кампания не готова к оплате (R-6): 409 campaign_incomplete.
type IncompleteError struct {
	MissingFields []string
	PostProblems  []string
}

func (e *IncompleteError) Error() string {
	return fmt.Sprintf("placement: campaign incomplete: fields [%s], post [%s]",
		strings.Join(e.MissingFields, ","), strings.Join(e.PostProblems, ","))
}

// CartChangedError — корзина разошлась с показанной (R-7): 409 cart_changed.
type CartChangedError struct{ Items []uuid.UUID }

func (e *CartChangedError) Error() string {
	return fmt.Sprintf("placement: cart changed: %d items", len(e.Items))
}

// InsufficientBalanceError — баланса не хватает на удержания (R-2): 409 insufficient_balance.
type InsufficientBalanceError struct{ Missing money.Kopecks }

func (e *InsufficientBalanceError) Error() string {
	return fmt.Sprintf("placement: insufficient balance: missing %d", e.Missing)
}

// Ошибки публикаций, нарушений и споров (I-11, R-10).
var (
	// ErrMessageNotFound — сообщения нет в MAX (удалено).
	ErrMessageNotFound = errors.New("placement: max message not found")
	// ErrPublicationExists — у размещения уже есть публикация (I-11).
	ErrPublicationExists = errors.New("placement: publication already exists")
	// ErrDuplicateViolation — нарушение по этому событию уже записано (R-10).
	ErrDuplicateViolation = errors.New("placement: violation already recorded for event")
	// ErrDisputeExists — у размещения уже есть спор (I-11).
	ErrDisputeExists = errors.New("placement: dispute already exists")
)
