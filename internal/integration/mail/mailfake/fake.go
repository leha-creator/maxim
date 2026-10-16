// Package mailfake — почта в памяти для тестов и разработки без SMTP (07 §9).
package mailfake

import (
	"context"
	"sync"
	"time"
)

// LoginCodeMessage — отправленное письмо с кодом входа.
type LoginCodeMessage struct {
	To        string
	Code      string
	ExpiresIn time.Duration
}

// Sender запоминает письма вместо отправки.
type Sender struct {
	mu      sync.Mutex
	sent    []LoginCodeMessage
	failErr error
}

// New создаёт пустой Sender.
func New() *Sender { return &Sender{} }

// SendLoginCode запоминает письмо или возвращает ошибку, заданную FailWith.
func (s *Sender) SendLoginCode(ctx context.Context, email, code string, expiresIn time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failErr != nil {
		return s.failErr
	}
	s.sent = append(s.sent, LoginCodeMessage{To: email, Code: code, ExpiresIn: expiresIn})
	return nil
}

// FailWith заставляет следующие отправки возвращать err; nil — снова отправлять.
func (s *Sender) FailWith(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failErr = err
}

// Last — последнее письмо на адрес.
func (s *Sender) Last(email string) (LoginCodeMessage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := len(s.sent) - 1; i >= 0; i-- {
		if s.sent[i].To == email {
			return s.sent[i], true
		}
	}
	return LoginCodeMessage{}, false
}

// Count — сколько писем отправлено.
func (s *Sender) Count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sent)
}
