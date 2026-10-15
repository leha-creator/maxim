// Package clock — подменяемые часы (07 §9): реальное время плюс сдвиг,
// который задаёт POST /_mock/clock, чтобы срабатывали таймеры.
package clock

import (
	"sync"
	"time"
)

// Clock безопасен для одновременного использования.
type Clock struct {
	mu     sync.Mutex
	offset time.Duration
	now    func() time.Time
}

// New создаёт часы без сдвига поверх time.Now.
func New() *Clock {
	return &Clock{now: time.Now}
}

// NewFixed создаёт часы, которые стоят на месте, — для тестов.
func NewFixed(t time.Time) *Clock {
	return &Clock{now: func() time.Time { return t }}
}

// Now возвращает текущее время в UTC с учётом сдвига, с точностью до секунды.
func (c *Clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now().Add(c.offset).UTC().Truncate(time.Second)
}

// Shift сдвигает часы вперёд.
func (c *Clock) Shift(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.offset += d
}

// ShiftTo сдвигает часы так, чтобы Now() стало не раньше t.
func (c *Clock) ShiftTo(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cur := c.now().Add(c.offset); cur.Before(t) {
		c.offset += t.Sub(cur)
	}
}

// Offset возвращает текущий сдвиг.
func (c *Clock) Offset() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.offset
}

// Reset убирает сдвиг.
func (c *Clock) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.offset = 0
}
