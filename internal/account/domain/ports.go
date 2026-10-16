package domain

import (
	"context"
	"io"
	"net/netip"
	"time"

	"github.com/google/uuid"
)

// Порты модуля. Репозитории реализует account/postgres, остальное — адаптеры,
// которые связывает bootstrap. Все методы работают в транзакции из context, если она открыта.

// Users — пользователи.
type Users interface {
	// GetByEmail возвращает ErrNotFound, если пользователя нет.
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	Create(ctx context.Context, u *User) error
	SetLastLogin(ctx context.Context, id uuid.UUID, at time.Time) error
	// SetAdminRole ставит или снимает (nil) роль администратора (R-2 этапа «Каталог»).
	SetAdminRole(ctx context.Context, id uuid.UUID, role *AdminRole, at time.Time) error
}

// WindowCount — сколько записей в окне и когда появилась самая ранняя из них.
type WindowCount struct {
	Count  int
	Oldest time.Time
}

// AuthCodes — коды входа.
type AuthCodes interface {
	Create(ctx context.Context, c *LoginCode) error
	// LatestActiveForUpdate — последний непогашенный код адреса с блокировкой строки;
	// ErrNotFound, если такого нет.
	LatestActiveForUpdate(ctx context.Context, email string) (*LoginCode, error)
	// ConsumeActive гасит все непогашенные коды адреса: действует только последний.
	ConsumeActive(ctx context.Context, email string, now time.Time) error
	// Save сохраняет попытки и отметку погашения.
	Save(ctx context.Context, c *LoginCode) error
	CountByEmailSince(ctx context.Context, email string, since time.Time) (WindowCount, error)
	CountByIPSince(ctx context.Context, ip netip.Addr, since time.Time) (WindowCount, error)
}

// Sessions — сессии.
type Sessions interface {
	Create(ctx context.Context, s *Session) error
	// GetByTokenHash возвращает ErrNotFound, если сессии нет.
	GetByTokenHash(ctx context.Context, hash string) (*Session, error)
	SetExpiresAt(ctx context.Context, id uuid.UUID, expiresAt, now time.Time) error
	Revoke(ctx context.Context, id uuid.UUID, now time.Time) error
}

// Organizations — организации, участники и профили. Методы кабинета принимают
// идентификатор организации явно (07 §4).
type Organizations interface {
	// Create сохраняет организацию, её профили и участника-владельца.
	Create(ctx context.Context, o *Organization, owner uuid.UUID) error
	// GetByUser возвращает организацию, где пользователь — участник; ErrNotFound, если её нет.
	GetByUser(ctx context.Context, userID uuid.UUID) (*Organization, error)
	// GetForUpdate читает организацию с блокировкой строки.
	GetForUpdate(ctx context.Context, organizationID uuid.UUID) (*Organization, error)
	// Update сохраняет реквизиты и создаёт недостающие профили.
	Update(ctx context.Context, o *Organization) error
	INNTaken(ctx context.Context, inn string) (bool, error)
}

// LegalDocuments — оферты и их принятие.
type LegalDocuments interface {
	// Current — действующие версии по видам (наибольший version).
	Current(ctx context.Context) ([]LegalDocument, error)
	AcceptedIDs(ctx context.Context, organizationID uuid.UUID) (map[uuid.UUID]bool, error)
	// Accept записывает принятие; повторное принятие того же документа ничего не меняет.
	Accept(ctx context.Context, a LegalAcceptance) error
	// LatestVersion — наибольший version вида; 0, если версий нет.
	LatestVersion(ctx context.Context, kind LegalDocumentKind) (int, error)
	Create(ctx context.Context, d *LegalDocument) error
}

// LoginCodeSender отправляет письмо с кодом. Вызывается только из задания (07 §1).
type LoginCodeSender interface {
	SendLoginCode(ctx context.Context, email, code string, expiresIn time.Duration) error
}

// FileStorage — закрытая область объектного хранилища (07 §4).
type FileStorage interface {
	PutPrivate(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
}
