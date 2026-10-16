package domain

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/netip"
	"time"

	"github.com/google/uuid"
)

// sessionTokenBytes — длина случайного токена: не короче 256 бит (07 §4).
const sessionTokenBytes = 32

// sessionTouchInterval — как часто продлевать сессию: не на каждый запрос.
const sessionTouchInterval = time.Hour

// Session — сессия пользователя (04 §2). В базе хранится только хеш токена.
type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	IP        *netip.Addr
	UserAgent *string
	RevokedAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewToken возвращает случайный токен (base64url) и его хеш.
func NewToken() (token, hash string, err error) {
	b := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token), nil
}

// HashToken — SHA-256 токена в hex.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// SessionTTL — срок сессии пользователя: у администратора свой, короче (07 §4).
func SessionTTL(u *User, ttl, adminTTL time.Duration) time.Duration {
	if u.IsAdmin() {
		return adminTTL
	}
	return ttl
}

// NewSession создаёт сессию со сроком ttl от now.
func NewSession(id, userID uuid.UUID, tokenHash string, ip *netip.Addr, userAgent *string, ttl time.Duration, now time.Time) *Session {
	return &Session{
		ID: id, UserID: userID, TokenHash: tokenHash, ExpiresAt: now.Add(ttl),
		IP: ip, UserAgent: userAgent, CreatedAt: now, UpdatedAt: now,
	}
}

// Active сообщает, что сессия не отозвана и не истекла.
func (s *Session) Active(now time.Time) bool {
	return s.RevokedAt == nil && now.Before(s.ExpiresAt)
}

// Touch продлевает сессию до now+ttl, если с прошлого продления прошло больше
// часа. Срок считается от последнего использования (07 §4). Возвращает true,
// если срок изменился и его нужно сохранить.
func (s *Session) Touch(ttl time.Duration, now time.Time) bool {
	lastTouch := s.ExpiresAt.Add(-ttl)
	if now.Sub(lastTouch) < sessionTouchInterval {
		return false
	}
	s.ExpiresAt = now.Add(ttl)
	s.UpdatedAt = now
	return true
}
