package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// Порты модуля. Репозитории реализует channel/postgres, адаптеры MAX — integration/max
// через переходник в bootstrap. Методы репозиториев работают в транзакции из context,
// если она открыта.

// Channels — каналы. Кабинетные методы принимают организацию явно (07 §4).
type Channels interface {
	Create(ctx context.Context, c *Channel) error
	// Save сохраняет канал, если в базе lock_version = c.LockVersion (версия при чтении),
	// и увеличивает c.LockVersion; иначе ErrStaleVersion (04 §1).
	Save(ctx context.Context, c *Channel) error
	// GetByMaxChatIDForUpdate — канал по идентификатору чата MAX с блокировкой строки (R-6).
	GetByMaxChatIDForUpdate(ctx context.Context, maxChatID int64) (*Channel, error)
	// GetForUpdate — канал по идентификатору с блокировкой строки (задания worker).
	GetForUpdate(ctx context.Context, id uuid.UUID) (*Channel, error)
	// GetForOrganization — канал организации; ErrNotFound для чужого.
	GetForOrganization(ctx context.Context, orgID, id uuid.UUID) (*Channel, error)
	// GetForOrganizationForUpdate — то же с блокировкой строки.
	GetForOrganizationForUpdate(ctx context.Context, orgID, id uuid.UUID) (*Channel, error)
	ListByOrganization(ctx context.Context, orgID uuid.UUID) ([]*Channel, error)
	// ListUnclaimedBefore — каналы в unclaimed, ждущие кода с момента раньше before (R-12).
	ListUnclaimedBefore(ctx context.Context, before time.Time, limit int) ([]uuid.UUID, error)
	// ListForMetrics — каналы в moderation, active, suspended с id больше after,
	// порцией по id (R-13).
	ListForMetrics(ctx context.Context, after uuid.UUID, limit int) ([]uuid.UUID, error)
}

// LinkCodes — коды подключения (R-15: строки auth_code с purpose = channel_link).
type LinkCodes interface {
	Create(ctx context.Context, c *LinkCode) error
	// ConsumeLive гасит все живые коды организации: действует только последний (R-2).
	ConsumeLive(ctx context.Context, orgID uuid.UUID, now time.Time) error
	// FindLiveForUpdate — живой код по хешу с блокировкой строки; ErrNotFound, если нет.
	FindLiveForUpdate(ctx context.Context, codeHash string, now time.Time) (*LinkCode, error)
	// Consume гасит код.
	Consume(ctx context.Context, c *LinkCode) error
	CountSince(ctx context.Context, orgID uuid.UUID, since time.Time) (int, error)
}

// Category — категория канала (04 §3, channel_category).
type Category struct {
	ID       uuid.UUID
	Slug     string
	Name     string
	Position int
}

// Categories — справочник категорий.
type Categories interface {
	ListActive(ctx context.Context) ([]Category, error)
	// ExistsActive сообщает, что категория есть и включена.
	ExistsActive(ctx context.Context, id uuid.UUID) (bool, error)
}

// ChatReader — чтение канала в MAX. Вызывается только из заданий worker (R-7).
type ChatReader interface {
	GetChat(ctx context.Context, maxChatID int64) (ChatInfo, error)
	GetMembership(ctx context.Context, maxChatID int64) (Membership, error)
}

// ChatManager — действия бота в MAX. Вызывается только из заданий worker (R-7).
type ChatManager interface {
	DeleteMessage(ctx context.Context, maxChatID int64, messageID string) error
	LeaveChat(ctx context.Context, maxChatID int64) error
	SendUserMessage(ctx context.Context, maxUserID int64, text string) (string, error)
}

// Formats — справочник форматов размещения (03 §2).
type Formats interface {
	// ListActive — активные форматы по порядку.
	ListActive(ctx context.Context) ([]Format, error)
}

// Prices — цены каналов. Загружаются вместе с каналом (Channel.Prices).
type Prices interface {
	// Replace заменяет набор цен канала: отсутствующие форматы удаляются (R-5).
	Replace(ctx context.Context, channelID uuid.UUID, prices []Price, now time.Time) error
}

// Unavailabilities — интервалы недоступности каналов (S-13).
type Unavailabilities interface {
	// ListLive — интервалы канала, которые ещё не закончились, по началу.
	ListLive(ctx context.Context, channelID uuid.UUID, now time.Time) ([]Unavailability, error)
	Create(ctx context.Context, channelID uuid.UUID, u Unavailability, now time.Time) error
	// Delete удаляет интервал канала; ErrNotFound, если у канала его нет.
	Delete(ctx context.Context, channelID, id uuid.UUID) error
}

// Metrics — суточные снимки метрик (channel_metrics, R-19).
type Metrics interface {
	// Upsert записывает снимок суток; повтор в те же сутки перезаписывает его.
	Upsert(ctx context.Context, channelID uuid.UUID, s MetricsSnapshot, now time.Time) error
	// LastBefore — последний снимок раньше даты date; nil, если его нет.
	LastBefore(ctx context.Context, channelID uuid.UUID, date time.Time) (*MetricsSnapshot, error)
	// Recent — последние n снимков, старые первыми.
	Recent(ctx context.Context, channelID uuid.UUID, n int) ([]MetricsSnapshot, error)
}

// Catalog — выборки каталога рекламодателя (R-7, R-9 … R-11).
type Catalog interface {
	// List — страница каталога по запросу; условие видимости R-7 применяется в выборке.
	List(ctx context.Context, q CatalogQuery) ([]CatalogItem, error)
	// Get — видимый канал каталога; ErrNotFound для невидимого.
	Get(ctx context.Context, id uuid.UUID, feeRateBP, rknThreshold int) (*CatalogItem, error)
}

// AdminFilter — фильтры очереди администратора (R-1).
type AdminFilter struct {
	Status        *Status
	RknStatus     *RknStatus
	RknRecheckDue *bool
	// RecheckBefore — номер подтверждён раньше этого момента: пора проверить снова (R-17).
	RecheckBefore time.Time
	After         *AdminCursor
	Limit         int
}

// AdminCursor — позиция в очереди администратора: (updated_at, id) последнего канала.
type AdminCursor struct {
	UpdatedAt time.Time `json:"u"`
	ID        uuid.UUID `json:"id"`
}

// OrganizationRef — организация-владелец канала для экрана администратора.
type OrganizationRef struct {
	ID   uuid.UUID
	Name string
	INN  string
}

// AdminChannel — канал с владельцем для экрана администратора.
type AdminChannel struct {
	Channel      *Channel
	Organization *OrganizationRef
}

// ModerationEntry — решение модерации или проверки РКН из журнала действий.
type ModerationEntry struct {
	Action      string
	ActorType   string
	ActorUserID *uuid.UUID
	Result      string
	Comment     string
	At          time.Time
}

// AdminChannels — выборки администратора. Вызываются только после проверки роли (07 §4).
type AdminChannels interface {
	ListForAdmin(ctx context.Context, f AdminFilter) ([]AdminChannel, error)
	GetForAdmin(ctx context.Context, id uuid.UUID) (*AdminChannel, error)
	// GetForAdminForUpdate — канал с блокировкой строки для решения администратора.
	GetForAdminForUpdate(ctx context.Context, id uuid.UUID) (*Channel, error)
	// ModerationHistoryForAdmin — решения по каналу, старые первыми.
	ModerationHistoryForAdmin(ctx context.Context, channelID uuid.UUID) ([]ModerationEntry, error)
}

// MessageReader — чтение сообщений канала в MAX (GET /messages?chat_id=). Вызывается
// только из заданий worker (R-7 этапа «Подключение каналов»).
type MessageReader interface {
	// ListMessages — до count сообщений чата за интервал [from, to], новые первыми.
	ListMessages(ctx context.Context, maxChatID int64, from, to time.Time, count int) ([]MessageStat, error)
}

// PlacementSchedule — интервалы канала, занятые размещениями (R-8). Реализацию даст
// модуль размещений; до него bootstrap подставляет пустую.
type PlacementSchedule interface {
	BusyIntervals(ctx context.Context, channelID uuid.UUID, from, to time.Time) ([]Unavailability, error)
}

// AvailabilityListener — последствия смены доступности канала для размещений
// (S-25 шаг 2, R-11). Вызывается в той же транзакции, что и смена состояния.
// Реализацию даст модуль размещений; до него bootstrap подставляет пустую.
type AvailabilityListener interface {
	ChannelUnavailable(ctx context.Context, change StatusChange) error
	ChannelRestored(ctx context.Context, change StatusChange) error
}
