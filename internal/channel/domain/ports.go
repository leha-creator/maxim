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

// AvailabilityListener — последствия смены доступности канала для размещений
// (S-25 шаг 2, R-11). Вызывается в той же транзакции, что и смена состояния.
// Реализацию даст модуль размещений; до него bootstrap подставляет пустую.
type AvailabilityListener interface {
	ChannelUnavailable(ctx context.Context, change StatusChange) error
	ChannelRestored(ctx context.Context, change StatusChange) error
}
