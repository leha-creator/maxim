// Package domain — правила выплат блогерам (05 §4, 06 §2.3, §4): исполнитель Рокет Ворк,
// попытки выплаты и их переходы, коды отказов, расписание опроса, контроль депозита и
// годовой лимит самозанятого. Без базы, сети и журнала.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// LegalType — тип лица блогера (04 §9); значения совпадают с organization.legal_type.
type LegalType string

const (
	LegalEntity    LegalType = "legal_entity"
	SoleProprietor LegalType = "sole_proprietor"
	SelfEmployed   LegalType = "self_employed"
)

// ContractorRequired — нужен ли исполнитель Рокет Ворк (A-36, R-3): юрлицу — нет,
// ему платят переводом вне сервиса.
func ContractorRequired(t LegalType) bool { return t != LegalEntity }

// ContractorStatus — состояние исполнителя (04 §7, rocketwork_contractor.status).
type ContractorStatus string

const (
	// ContractorNone — строки нет: онбординг не начат (только в ответах API).
	ContractorNone    ContractorStatus = "none"
	ContractorInvited ContractorStatus = "invited"
	ContractorPending ContractorStatus = "pending"
	ContractorActive  ContractorStatus = "active"
	ContractorBlocked ContractorStatus = "blocked"
)

// Valid — состояние из перечня таблицы (без none).
func (s ContractorStatus) Valid() bool {
	switch s {
	case ContractorInvited, ContractorPending, ContractorActive, ContractorBlocked:
		return true
	}
	return false
}

// Contractor — исполнитель Рокет Ворк организации-блогера.
type Contractor struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	// ExternalID — идентификатор в Рокет Ворк; пусто, пока ensure_contractor не выполнен.
	ExternalID      string
	Status          ContractorStatus
	BlockedReason   string
	OnboardingURL   string
	StatusCheckedAt *time.Time
	LastError       string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// NewContractor — строка invited без ссылки: задание ensure_contractor её заполнит (I-13).
func NewContractor(id, orgID uuid.UUID, now time.Time) Contractor {
	return Contractor{ID: id, OrganizationID: orgID, Status: ContractorInvited, CreatedAt: now, UpdatedAt: now}
}

// CanAccept — исполнитель готов к выплатам (P-03, S-14).
func (c Contractor) CanAccept() bool { return c.Status == ContractorActive }

// Registered — исполнитель создан в Рокет Ворк.
func (c Contractor) Registered() bool { return c.ExternalID != "" }

// Final — опрос больше не нужен до смены статуса сервисом: active опрашивается раз в сутки,
// blocked — не опрашивается (ждём вебхук или «Проверить статус»).
func (c Contractor) Final() bool { return c.Status == ContractorBlocked }

// ProviderContractor — исполнитель в ответе адаптера, уже в наших терминах (R-2).
type ProviderContractor struct {
	ExternalID    string
	Status        ContractorStatus
	BlockedReason string
	OnboardingURL string
}

// Apply переносит ответ адаптера. changed — сменился статус (для журнала и уведомлений).
// Пустые поля ответа не затирают известные значения.
func (c *Contractor) Apply(p ProviderContractor, at time.Time) (changed bool) {
	if p.ExternalID != "" {
		c.ExternalID = p.ExternalID
	}
	if p.OnboardingURL != "" {
		c.OnboardingURL = p.OnboardingURL
	}
	if p.Status.Valid() && p.Status != c.Status {
		c.Status = p.Status
		changed = true
	}
	if c.Status == ContractorBlocked {
		c.BlockedReason = p.BlockedReason
	} else {
		c.BlockedReason = ""
	}
	checked := at
	c.StatusCheckedAt = &checked
	c.LastError = ""
	c.UpdatedAt = at
	return changed
}

// Stale — последний известный статус старше maxAge (R-7): перед выплатой нужен GetContractor.
func (c Contractor) Stale(now time.Time, maxAge time.Duration) bool {
	return c.StatusCheckedAt == nil || now.Sub(*c.StatusCheckedAt) > maxAge
}
