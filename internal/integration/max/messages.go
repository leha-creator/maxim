package max

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"unicode/utf8"

	"github.com/leha-creator/maxim/internal/platform/job"
	"github.com/leha-creator/maxim/internal/platform/logx"
	"github.com/leha-creator/maxim/internal/platform/ratelimit"
)

// Bot — сведения о боте (GET /me).
type Bot struct {
	UserID   int64  `json:"user_id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

// GetMe проверяет токен и возвращает сведения о боте.
func (c *Client) GetMe(ctx context.Context) (Bot, error) {
	resp, err := c.do(ctx, request{operation: "GetMe", method: http.MethodGet, path: "/me"})
	if err != nil {
		return Bot{}, err
	}
	var b Bot
	if err := json.Unmarshal(resp.Body, &b); err != nil {
		return Bot{}, job.Permanent(fmt.Errorf("max GetMe: decode response: %w", err))
	}
	c.log.DebugContext(ctx, "max bot checked", logx.KeyOp, "MAX.GetMe", "bot_user_id", b.UserID)
	return b, nil
}

// SendChatMessage отправляет текст в чат (например, чат дежурных, R-5).
// Возвращает идентификатор сообщения.
func (c *Client) SendChatMessage(ctx context.Context, chatID int64, text string) (string, error) {
	return c.sendMessage(ctx, "SendChatMessage", "chat_id", chatID, c.chatBucket("chat", chatID), text)
}

// SendUserMessage отправляет текст пользователю MAX (блогеру по
// channel.added_by_max_user_id, A-41). Доставка не гарантирована (Q-38).
func (c *Client) SendUserMessage(ctx context.Context, userID int64, text string) (string, error) {
	return c.sendMessage(ctx, "SendUserMessage", "user_id", userID, c.chatBucket("user", userID), text)
}

type newMessage struct {
	Text   string `json:"text"`
	Format string `json:"format,omitempty"`
}

type sentMessage struct {
	Message struct {
		Body struct {
			MID string `json:"mid"`
		} `json:"body"`
	} `json:"message"`
}

func (c *Client) sendMessage(ctx context.Context, op, param string, id int64, bucket ratelimit.Bucket, text string) (string, error) {
	if text == "" || utf8.RuneCountInString(text) > MaxTextLength {
		return "", job.Permanent(fmt.Errorf("max %s: text length must be 1..%d characters", op, MaxTextLength))
	}
	idStr := strconv.FormatInt(id, 10)
	resp, err := c.do(ctx, request{
		operation: op, method: http.MethodPost, path: "/messages",
		query:   url.Values{param: {idStr}},
		body:    newMessage{Text: text, Format: "markdown"},
		buckets: []ratelimit.Bucket{bucket},
		object:  param, objectID: idStr,
	})
	if err != nil {
		return "", err
	}
	var m sentMessage
	if err := json.Unmarshal(resp.Body, &m); err != nil {
		return "", job.Permanent(fmt.Errorf("max %s: decode response: %w", op, err))
	}
	// Текст сообщения в журнал не пишется (07 §5.4).
	c.log.InfoContext(ctx, "max message sent", logx.KeyOp, "MAX."+op, param, id, "message_id", m.Message.Body.MID)
	return m.Message.Body.MID, nil
}
