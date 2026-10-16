package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Пределы полей площадки в ОРД.
const (
	maxPlatformName = 100
	maxPlatformURL  = 2000
)

// OrdPlatformTypes — перечисление PlatformType спецификации v8.
var OrdPlatformTypes = []string{"site", "apps", "smarttv", "hbbtv"}

// ChannelRef — канал, одобренный модератором (порт channel/domain.ChannelApprovalListener, R-13).
type ChannelRef struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Title          string
	Link           string
}

// Platform — площадка в ОРД (04 §6, A-32). В ОРД идентифицируется channel_id (R-1).
type Platform struct {
	ID             uuid.UUID
	ChannelID      uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	URL            string
	Sync           SyncState
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// Ref — ссылка для заданий.
func (p Platform) Ref() ObjectRef { return ObjectRef{Type: TypePlatform, ID: p.ID} }

// ExternalID — идентификатор в ОРД.
func (p Platform) ExternalID() string { return p.ChannelID.String() }

// PlatformName — название канала для ОРД: без пробелов по краям, не длиннее 100 рун (R-13).
func PlatformName(title string) string {
	return strings.TrimSpace(truncate(strings.TrimSpace(title), maxPlatformName))
}

// PlatformRequest — тело POST /platforms (API v8).
type PlatformRequest struct {
	OrganizationID string                `json:"organizationId"`
	Platforms      []PlatformRequestItem `json:"platforms"`
}

// PlatformRequestItem — площадка в запросе.
type PlatformRequestItem struct {
	PlatformID string `json:"platformId"`
	IsOwned    bool   `json:"isOwned"`
	Type       string `json:"type"`
	Name       string `json:"name"`
	URL        string `json:"url,omitempty"`
}

// NewPlatformRequest собирает запрос площадки канала (R-13): площадка не своя (isOwned = false),
// тип — ORD_PLATFORM_TYPE (A-32, Q-13), организация — блогер.
func NewPlatformRequest(p Platform, platformType string) PlatformRequest {
	return PlatformRequest{
		OrganizationID: p.OrganizationID.String(),
		Platforms: []PlatformRequestItem{{
			PlatformID: p.ExternalID(), IsOwned: false, Type: platformType,
			Name: p.Name, URL: truncate(p.URL, maxPlatformURL),
		}},
	}
}
