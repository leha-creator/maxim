package maxfake

import (
	"context"
	"fmt"
	"slices"

	"github.com/leha-creator/maxim/internal/integration/max"
)

// SetChat задаёт сведения о чате, которые вернёт GetChat.
func (c *Client) SetChat(chat max.Chat) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.chats == nil {
		c.chats = map[int64]max.Chat{}
	}
	c.chats[chat.ChatID] = chat
}

// SetMembership задаёт права бота в чате, которые вернёт GetMembership.
func (c *Client) SetMembership(chatID int64, m max.Membership) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.members == nil {
		c.members = map[int64]max.Membership{}
	}
	c.members[chatID] = m
}

// GetChat возвращает чат, заданный SetChat; неизвестный чат — отказ chat.not.found.
func (c *Client) GetChat(_ context.Context, chatID int64) (max.Chat, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failErr != nil {
		return max.Chat{}, c.failErr
	}
	chat, ok := c.chats[chatID]
	if !ok {
		return max.Chat{}, fmt.Errorf("maxfake: chat %d not found", chatID)
	}
	return chat, nil
}

// GetMembership возвращает права, заданные SetMembership; по умолчанию бот не администратор.
func (c *Client) GetMembership(_ context.Context, chatID int64) (max.Membership, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failErr != nil {
		return max.Membership{}, c.failErr
	}
	m := c.members[chatID]
	m.Permissions = slices.Clone(m.Permissions)
	if m.Permissions == nil {
		m.Permissions = []string{}
	}
	return m, nil
}

// DeleteMessage запоминает удалённое сообщение.
func (c *Client) DeleteMessage(_ context.Context, _ int64, messageID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failErr != nil {
		return c.failErr
	}
	c.deleted = append(c.deleted, messageID)
	return nil
}

// LeaveChat запоминает чат, который покинул бот.
func (c *Client) LeaveChat(_ context.Context, chatID int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failErr != nil {
		return c.failErr
	}
	c.left = append(c.left, chatID)
	return nil
}

// ListSubscriptions возвращает подписки, созданные Subscribe.
func (c *Client) ListSubscriptions(context.Context) ([]max.Subscription, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failErr != nil {
		return nil, c.failErr
	}
	return slices.Clone(c.subs), nil
}

// Subscribe запоминает подписку.
func (c *Client) Subscribe(_ context.Context, url string, updateTypes []string, _ string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failErr != nil {
		return c.failErr
	}
	c.subs = append(c.subs, max.Subscription{URL: url, UpdateTypes: slices.Clone(updateTypes)})
	return nil
}

// Deleted — идентификаторы удалённых сообщений.
func (c *Client) Deleted() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.deleted)
}

// Left — чаты, которые покинул бот.
func (c *Client) Left() []int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.left)
}
