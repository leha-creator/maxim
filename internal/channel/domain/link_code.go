package domain

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LinkCodeLength — длина кода подключения канала (A-41, R-1).
const LinkCodeLength = 8

// linkCodeAlphabet — символы кода без похожих знаков (0/O, 1/I): 32 символа, 40 бит на код.
const linkCodeAlphabet = "23456789ABCDEFGHJKLMNPQRSTUVWXYZ"

// LinkCode — одноразовый код подключения канала (04 §2, auth_code с purpose = channel_link).
// Сам код не хранится.
type LinkCode struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	CodeHash       string
	ExpiresAt      time.Time
	ConsumedAt     *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// GenerateLinkCode возвращает случайный код из LinkCodeLength символов.
func GenerateLinkCode() (string, error) {
	var b strings.Builder
	limit := big.NewInt(int64(len(linkCodeAlphabet)))
	for range LinkCodeLength {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("generate channel link code: %w", err)
		}
		b.WriteByte(linkCodeAlphabet[n.Int64()])
	}
	return b.String(), nil
}

// NormalizeLinkCode проверяет, похож ли текст сообщения на код: пробелы по краям
// и регистр не важны. ok = false — текст не код, сообщение не разбирается дальше.
func NormalizeLinkCode(text string) (code string, ok bool) {
	code = strings.ToUpper(strings.TrimSpace(text))
	if len(code) != LinkCodeLength {
		return "", false
	}
	for i := range len(code) {
		if !strings.ContainsRune(linkCodeAlphabet, rune(code[i])) {
			return "", false
		}
	}
	return code, true
}

// HashLinkCode — HMAC-SHA256 кода с секретом из окружения (R-2). Назначение входит в
// сообщение, поэтому хеш кода подключения не совпадает с хешем такого же кода входа.
func HashLinkCode(pepper []byte, code string) string {
	m := hmac.New(sha256.New, pepper)
	m.Write([]byte("channel_link"))
	m.Write([]byte{0})
	m.Write([]byte(code))
	return hex.EncodeToString(m.Sum(nil))
}

// NewLinkCode создаёт запись кода для организации.
func NewLinkCode(id, orgID uuid.UUID, code string, pepper []byte, ttl time.Duration, now time.Time) *LinkCode {
	return &LinkCode{
		ID:             id,
		OrganizationID: orgID,
		CodeHash:       HashLinkCode(pepper, code),
		ExpiresAt:      now.Add(ttl),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// Live — код не погашен и не просрочен.
func (c *LinkCode) Live(now time.Time) bool {
	return c.ConsumedAt == nil && now.Before(c.ExpiresAt)
}

// Consume гасит код.
func (c *LinkCode) Consume(now time.Time) {
	c.ConsumedAt = &now
	c.UpdatedAt = now
}
