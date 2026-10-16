// Package secretbox шифрует секреты, которые приходится хранить в базе, —
// например код входа в полезной нагрузке задания до отправки письма (R-1).
// Ключ хранится вне базы, в переменной окружения APP_SECRETS_KEY (07 §4).
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
)

// KeySize — длина ключа AES-256.
const KeySize = 32

// ErrOpen — данные повреждены или зашифрованы другим ключом.
var ErrOpen = errors.New("secretbox: cannot open sealed data")

// Box шифрует и расшифровывает данные AES-256-GCM.
type Box struct {
	aead cipher.AEAD
}

// New создаёт Box с ключом длиной KeySize.
func New(key []byte) (*Box, error) {
	if len(key) != KeySize {
		return nil, fmt.Errorf("secretbox: key must be %d bytes", KeySize)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("secretbox: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("secretbox: %w", err)
	}
	return &Box{aead: aead}, nil
}

// Seal возвращает base64(nonce || шифртекст).
func (b *Box) Seal(plaintext []byte) (string, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("secretbox: nonce: %w", err)
	}
	return base64.StdEncoding.EncodeToString(b.aead.Seal(nonce, nonce, plaintext, nil)), nil
}

// Open расшифровывает результат Seal.
func (b *Box) Open(sealed string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(sealed)
	if err != nil || len(raw) < b.aead.NonceSize() {
		return nil, ErrOpen
	}
	n := b.aead.NonceSize()
	out, err := b.aead.Open(nil, raw[:n], raw[n:], nil)
	if err != nil {
		return nil, ErrOpen
	}
	return out, nil
}
