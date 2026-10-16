// Package maxfake — MAX Bot API в памяти для тестов и разработки без бота (05 §1, 07 §9).
package maxfake

import (
	"context"
	"fmt"
	"sync"

	"github.com/leha-creator/maxim/internal/integration/max"
)

// Message — отправленное сообщение.
type Message struct {
	ChatID int64 // 0, если сообщение пользователю
	UserID int64 // 0, если сообщение в чат
	Text   string
	ID     string
}

// Client запоминает сообщения вместо отправки.
type Client struct {
	mu      sync.Mutex
	sent    []Message
	failErr error
	seq     int
}

// New создаёт пустой Client.
func New() *Client { return &Client{} }

// GetMe возвращает поддельного бота.
func (c *Client) GetMe(context.Context) (max.Bot, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failErr != nil {
		return max.Bot{}, c.failErr
	}
	return max.Bot{UserID: 1, Name: "maxim (fake)", Username: "maxim_fake_bot"}, nil
}

// SendChatMessage запоминает сообщение в чат.
func (c *Client) SendChatMessage(_ context.Context, chatID int64, text string) (string, error) {
	return c.add(Message{ChatID: chatID, Text: text})
}

// SendUserMessage запоминает сообщение пользователю.
func (c *Client) SendUserMessage(_ context.Context, userID int64, text string) (string, error) {
	return c.add(Message{UserID: userID, Text: text})
}

func (c *Client) add(m Message) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failErr != nil {
		return "", c.failErr
	}
	c.seq++
	m.ID = fmt.Sprintf("mid.fake.%d", c.seq)
	c.sent = append(c.sent, m)
	return m.ID, nil
}

// FailWith заставляет следующие вызовы возвращать err; nil — снова работать.
func (c *Client) FailWith(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failErr = err
}

// Sent возвращает копию отправленных сообщений.
func (c *Client) Sent() []Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]Message(nil), c.sent...)
}
