package domain

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/leha-creator/maxim/internal/platform/money"
)

// Порты модуля. Репозитории реализует placement/postgres; порты к другим модулям —
// переходники в bootstrap. Методы работают в транзакции из context, если она открыта.
// Кабинетные выборки принимают организацию явно (07 §4).

// Carts — позиции корзины организации.
type Carts interface {
	// List — позиции организации по времени добавления.
	List(ctx context.Context, orgID uuid.UUID) ([]CartItem, error)
	// Get — позиция организации; ErrNotFound для чужой.
	Get(ctx context.Context, orgID, id uuid.UUID) (CartItem, error)
	// Insert — новая позиция; канал уже в корзине — ErrAlreadyInCart.
	Insert(ctx context.Context, it CartItem) error
	// Update — смена формата и показанного итога.
	Update(ctx context.Context, it CartItem) error
	// Delete — удаление позиции организации; ErrNotFound для чужой.
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	// Clear — очистка корзины после оформления.
	Clear(ctx context.Context, orgID uuid.UUID) error
	Count(ctx context.Context, orgID uuid.UUID) (int, error)
	// ChannelsInCart — какие из каналов в корзине организации.
	ChannelsInCart(ctx context.Context, orgID uuid.UUID, channelIDs []uuid.UUID) (map[uuid.UUID]bool, error)
}

// Orders — заказы.
type Orders interface {
	// NextNumber — следующий номер из order_number_seq (R-21).
	NextNumber(ctx context.Context) (int64, error)
	Insert(ctx context.Context, o Order) error
	// Get — заказ организации; ErrNotFound для чужого.
	Get(ctx context.Context, orgID, id uuid.UUID) (Order, error)
	// GetForUpdate — то же с блокировкой строки.
	GetForUpdate(ctx context.Context, orgID, id uuid.UUID) (Order, error)
	// GetByID — заказ по идентификатору без проверки организации (после проверки доступа к размещению).
	GetByID(ctx context.Context, id uuid.UUID) (Order, error)
	// GetByIDForUpdate — заказ по идентификатору с блокировкой (система: таймеры, события).
	GetByIDForUpdate(ctx context.Context, id uuid.UUID) (Order, error)
	// List — заказы организации, новые сверху, после курсора.
	List(ctx context.Context, orgID uuid.UUID, status *OrderStatus, after *PageCursor, limit int) ([]Order, error)
	// FindByIdempotencyKey — заказ организации, оформленный с ключом (R-18).
	FindByIdempotencyKey(ctx context.Context, orgID uuid.UUID, key string) (Order, bool, error)
	// FindByCurrentVersion — заказ, у которого версия текущая (R-5), без блокировки.
	FindByCurrentVersion(ctx context.Context, versionID uuid.UUID) (Order, bool, error)
	// Update — состояние, текущая версия, ключ правки поста, updated_at.
	Update(ctx context.Context, o Order) error
}

// Placements — размещения.
type Placements interface {
	Insert(ctx context.Context, p Placement) error
	// Get — размещение по идентификатору без блокировки (система: узнать заказ и канал).
	Get(ctx context.Context, id uuid.UUID) (Placement, error)
	// GetForUpdate — размещение по идентификатору с блокировкой (система).
	GetForUpdate(ctx context.Context, id uuid.UUID) (Placement, error)
	// GetForAdvertiser — размещение заказа организации без блокировки; ErrNotFound для чужого.
	GetForAdvertiser(ctx context.Context, orgID, id uuid.UUID) (Placement, error)
	// GetForAdvertiserForUpdate — размещение заказа организации с блокировкой; ErrNotFound для чужого.
	GetForAdvertiserForUpdate(ctx context.Context, orgID, id uuid.UUID) (Placement, error)
	// GetForBlogger — размещение блогера, отправленное ему хотя бы раз (R-16); иначе ErrNotFound.
	GetForBlogger(ctx context.Context, orgID, id uuid.UUID) (Placement, error)
	// GetForBloggerForUpdate — то же с блокировкой строки.
	GetForBloggerForUpdate(ctx context.Context, orgID, id uuid.UUID) (Placement, error)
	// UpdateIfStatus сохраняет размещение, если в базе состояние status и lock_version
	// lockVersion (03 §11); иначе ErrStaleVersion.
	UpdateIfStatus(ctx context.Context, p Placement, status Status, lockVersion int) error
	// ListByOrder — размещения заказа по времени создания.
	ListByOrder(ctx context.Context, orderID uuid.UUID) ([]Placement, error)
	// ListByOrderForUpdate — то же с блокировкой строк в порядке id.
	ListByOrderForUpdate(ctx context.Context, orderID uuid.UUID) ([]Placement, error)
	// ListForBlogger — размещения блогера, отправленные ему, новые сверху, после курсора.
	ListForBlogger(ctx context.Context, orgID uuid.UUID, status *Status, channelID *uuid.UUID, after *PageCursor, limit int) ([]Placement, error)
	// BusyIntervals — интервалы топа размещений канала в scheduled и published, пересекающие
	// [from, to), кроме exclude (I-06).
	BusyIntervals(ctx context.Context, channelID uuid.UUID, from, to time.Time, exclude uuid.UUID) ([]Interval, error)
	// ListByChannel — размещения канала в состояниях statuses без блокировки.
	ListByChannel(ctx context.Context, channelID uuid.UUID, statuses []Status) ([]Placement, error)
	// ScheduledOverlapping — размещения канала в scheduled, у которых [publish_at, remove_at)
	// пересекает [from, to) (R-8 этапа «Каталог»).
	ScheduledOverlapping(ctx context.Context, channelID uuid.UUID, from, to time.Time) ([]uuid.UUID, error)
	// CountByStatus — число размещений по состояниям (07 §5.5).
	CountByStatus(ctx context.Context) (map[Status]int, error)
	// CountByOrders — число размещений заказов по состояниям.
	CountByOrders(ctx context.Context, orderIDs []uuid.UUID) (map[uuid.UUID]map[Status]int, error)
	// OverdueScheduled — размещения в scheduled с publish_at раньше before (07 §3).
	OverdueScheduled(ctx context.Context, before time.Time) ([]uuid.UUID, error)
}

// History — история состояний (03 §12). Только добавление.
type History interface {
	Insert(ctx context.Context, h HistoryEntry) error
	// ListByPlacements — записи размещений по времени.
	ListByPlacements(ctx context.Context, placementIDs []uuid.UUID) (map[uuid.UUID][]HistoryEntry, error)
	// FindByKey — запись размещения с ключом идемпотентности (R-18).
	FindByKey(ctx context.Context, placementID uuid.UUID, key string) (HistoryEntry, bool, error)
	// ExistsKeyInOrder — у одного из размещений заказа есть запись с ключом (R-17, R-18).
	ExistsKeyInOrder(ctx context.Context, orderID uuid.UUID, key string) (bool, error)
}

// Channels — каналы для оформления и действий блогера. Реализация — модуль каналов.
type Channels interface {
	// Offers — каналы с ценами и видимостью; отсутствующих в ответе нет.
	Offers(ctx context.Context, channelIDs []uuid.UUID) (map[uuid.UUID]ChannelOffer, error)
	// LockForPlacement — канал с блокировкой строки до конца транзакции (R-14, I-06, I-08);
	// ErrNotFound — канала нет.
	LockForPlacement(ctx context.Context, channelID uuid.UUID) (ChannelOffer, error)
	// Unavailability — интервалы недоступности канала, пересекающие [from, to).
	Unavailability(ctx context.Context, channelID uuid.UUID, from, to time.Time) ([]Interval, error)
}

// Campaigns — кампании. Реализация — модуль post (R-1).
type Campaigns interface {
	// LockForCheckout — кампания организации с блокировкой строки; ErrNotFound для чужой.
	LockForCheckout(ctx context.Context, orgID, campaignID uuid.UUID) (CampaignSnapshot, error)
	// Get — кампания организации (названия и требования для кабинетов).
	Get(ctx context.Context, orgID, campaignID uuid.UUID) (CampaignSnapshot, error)
	// MarkOrdered — draft → ordered со ссылкой на заказ.
	MarkOrdered(ctx context.Context, orgID, campaignID, orderID uuid.UUID) error
	// Archive — ordered → archived (R-19); уже архивная — без изменений.
	Archive(ctx context.Context, orgID, campaignID uuid.UUID) error
}

// VersionInfo — версия поста для ответов и правил модуля.
type VersionInfo struct {
	ID              uuid.UUID
	Number          int
	Status          VersionStatus
	Body            string
	FinalText       string
	AdvertiserName  string
	Erid            string
	MediaURLs       []string
	TokenCarrier    string
	TokenPosition   string
	RejectionReason string
	CreatedAt       time.Time
}

// PostNotReadyError — черновик поста не готов к версии: коды проблем (R-6).
type PostNotReadyError struct{ Problems []string }

func (e *PostNotReadyError) Error() string { return "placement: post is not ready" }

// PostVersions — версии поста. Реализация — модуль post (CreateVersion в транзакции вызывающего).
type PostVersions interface {
	// Create — следующая версия из черновика кампании; черновик с проблемами — PostNotReadyError.
	Create(ctx context.Context, orgID, campaignID, userID uuid.UUID, orderNumber int64) (VersionInfo, error)
	// Get — версия с публичными ссылками изображений.
	Get(ctx context.Context, versionID uuid.UUID) (VersionInfo, error)
}

// HoldRequest — удержание под размещение (06 §2.3). Ключ идемпотентности будущего
// журнала: hold:{placement_id}:{post_version_id} (R-2).
type HoldRequest struct {
	PlacementID   uuid.UUID
	PostVersionID uuid.UUID
	Amount        money.Kopecks
}

// Holds — удержания на балансе рекламодателя (R-2, I-03). Вызывается в транзакции перехода
// (I-10). Реализация — модуль ledger; до него bootstrap подставляет UnlimitedHolds.
type Holds interface {
	// HoldAll удерживает суммы атомарно; размещения с активным удержанием пропускаются.
	// Не хватает баланса — *InsufficientBalanceError, ничего не удержано.
	HoldAll(ctx context.Context, orgID uuid.UUID, reqs []HoldRequest) error
	// Release снимает активное удержание размещения; без него ничего не делает.
	Release(ctx context.Context, placementID uuid.UUID, reason string) error
}

// ContractorGate — блогер может принять запрос (A-36, R-3): исполнитель Рокет Ворк active
// (юрлицо не проверяется). Реализация — модуль payout; до него — AllowAllContractors.
type ContractorGate interface {
	CanAccept(ctx context.Context, bloggerOrgID uuid.UUID) (bool, error)
}

// Notice — уведомление стороне размещения (R-4).
type Notice struct {
	Template       string
	OrganizationID uuid.UUID
	Role           ActorType
	ObjectType     string
	ObjectID       string
	Data           map[string]any
}

// Notifier — уведомления (R-4): вызывается в транзакции перехода и только ставит работу.
// Реализация — модуль notification; до него — NopNotifier.
type Notifier interface {
	Notify(ctx context.Context, n Notice) error
}
