package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// User — пользователь. Единственный идентификатор для входа — почта (A-13).
type User struct {
	ID          uuid.UUID
	Email       string
	AdminRole   *AdminRole
	Status      Status
	LastLoginAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NewUser создаёт пользователя при первом входе.
func NewUser(id uuid.UUID, email string, now time.Time) *User {
	return &User{ID: id, Email: NormalizeEmail(email), Status: StatusActive, CreatedAt: now, UpdatedAt: now}
}

// IsAdmin сообщает, что у пользователя есть роль администратора.
func (u *User) IsAdmin() bool { return u.AdminRole != nil }

// Blocked сообщает, что пользователь заблокирован.
func (u *User) Blocked() bool { return u.Status == StatusBlocked }

// NormalizeEmail приводит адрес к виду, в котором он хранится и хешируется.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
