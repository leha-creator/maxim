// Package clock — единственный источник текущего времени (07 §9).
// Код модулей не вызывает time.Now(): он получает Clock через конструктор,
// чтобы тесты могли управлять временем.
package clock

import (
	"sync"
	"time"
)

// Clock возвращает текущее время в UTC.
type Clock interface {
	Now() time.Time
}

// System — настоящие часы.
type System struct{}

// Now возвращает текущее время в UTC.
func (System) Now() time.Time { return time.Now().UTC() }

// Manual — часы с ручным ходом для тестов. Безопасны для конкурентного доступа.
type Manual struct {
	mu  sync.Mutex
	now time.Time
}

// NewManual создаёт часы, показывающие t.
func NewManual(t time.Time) *Manual { return &Manual{now: t.UTC()} }

// Now возвращает установленное время.
func (m *Manual) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

// Set переводит часы на t.
func (m *Manual) Set(t time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = t.UTC()
}

// Advance сдвигает часы на d и возвращает новое время.
func (m *Manual) Advance(d time.Duration) time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.now = m.now.Add(d)
	return m.now
}
