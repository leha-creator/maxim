package domain

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// ErrInvalidCursor — курсор испорчен или от другого списка.
var ErrInvalidCursor = &RuleError{Field: "cursor", Reason: "invalid"}

type cursorJSON struct {
	T time.Time `json:"t"`
	I uuid.UUID `json:"i"`
}

// Encode — непрозрачная строка курсора для next_cursor.
func (c PageCursor) Encode() string {
	raw, _ := json.Marshal(cursorJSON{T: c.CreatedAt, I: c.ID})
	return base64.RawURLEncoding.EncodeToString(raw)
}

// DecodePageCursor разбирает курсор; пустая строка — первая страница.
func DecodePageCursor(s string) (*PageCursor, error) {
	if s == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, ErrInvalidCursor
	}
	var c cursorJSON
	if err := json.Unmarshal(raw, &c); err != nil || c.I == uuid.Nil || c.T.IsZero() {
		return nil, errors.Join(ErrInvalidCursor, err)
	}
	return &PageCursor{CreatedAt: c.T, ID: c.I}, nil
}
