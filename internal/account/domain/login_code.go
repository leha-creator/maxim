package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"net/netip"
	"time"

	"github.com/google/uuid"
)

// CodeLength — длина кода входа (07 §4).
const CodeLength = 6

// LoginFailure — причина неудачного входа. Клиенту причина не сообщается:
// он получает одинаковый ответ 401, причина пишется в журнал.
type LoginFailure string

const (
	FailureWrongCode        LoginFailure = "wrong_code"
	FailureExpired          LoginFailure = "expired"
	FailureAttemptsExceeded LoginFailure = "attempts_exceeded"
	FailureNoCode           LoginFailure = "no_code"
	FailureBlocked          LoginFailure = "blocked"
)

func (f LoginFailure) Error() string { return "login failed: " + string(f) }

// LoginCode — одноразовый код входа (04 §2, auth_code с purpose = login).
type LoginCode struct {
	ID          uuid.UUID
	Email       string
	CodeHash    string
	ExpiresAt   time.Time
	Attempts    int
	ConsumedAt  *time.Time
	RequestedIP *netip.Addr
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// GenerateCode возвращает случайный код из CodeLength цифр.
func GenerateCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("generate login code: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// HashCode — HMAC-SHA256 кода с секретом из окружения (R-3). Почта входит в
// сообщение, поэтому одинаковые коды разных адресов дают разные хеши, а по
// утёкшей базе без секрета код не перебрать.
func HashCode(pepper []byte, email, code string) string {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte(NormalizeEmail(email)))
	m.Write([]byte{0})
	m.Write([]byte(code))
	return hex.EncodeToString(m.Sum(nil))
}

// NewLoginCode создаёт запись кода; сам код в ней не хранится.
func NewLoginCode(id uuid.UUID, email, code string, pepper []byte, ip *netip.Addr, ttl time.Duration, now time.Time) *LoginCode {
	return &LoginCode{
		ID:          id,
		Email:       NormalizeEmail(email),
		CodeHash:    HashCode(pepper, email, code),
		ExpiresAt:   now.Add(ttl),
		RequestedIP: ip,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Verify проверяет введённый код. Неверный код увеличивает Attempts; верный —
// гасит код. Сравнение хешей — за постоянное время.
func (c *LoginCode) Verify(pepper []byte, code string, maxAttempts int, now time.Time) error {
	switch {
	case c.ConsumedAt != nil:
		return FailureNoCode
	case !now.Before(c.ExpiresAt):
		return FailureExpired
	case c.Attempts >= maxAttempts:
		return FailureAttemptsExceeded
	}
	c.UpdatedAt = now
	if !hmac.Equal([]byte(HashCode(pepper, c.Email, code)), []byte(c.CodeHash)) {
		c.Attempts++
		return FailureWrongCode
	}
	c.ConsumedAt = &now
	return nil
}

// Expired сообщает, что код просрочен к моменту now.
func (c *LoginCode) Expired(now time.Time) bool { return !now.Before(c.ExpiresAt) }
