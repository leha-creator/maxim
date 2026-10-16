// Package app — сценарии модуля каналов (S-12, S-25): коды подключения и операции
// кабинета блогера, разбор событий бота MAX, задания чтения прав, удаления
// сообщения с кодом и выхода из чата. Транзакции, задания, журнал действий и
// журнал приложения — здесь; правила — в domain.
package app

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/leha-creator/maxim/internal/channel/domain"
	"github.com/leha-creator/maxim/internal/platform/alert"
	"github.com/leha-creator/maxim/internal/platform/apperr"
	"github.com/leha-creator/maxim/internal/platform/audit"
	"github.com/leha-creator/maxim/internal/platform/clock"
	"github.com/leha-creator/maxim/internal/platform/job"
	"github.com/leha-creator/maxim/internal/platform/logx"
	"github.com/leha-creator/maxim/internal/platform/tx"
)

// KeyChannelID — поле журнала (07 §5.1).
const KeyChannelID = "channel_id"

// Config — параметры модуля (07 §8); значения по умолчанию — из документов.
type Config struct {
	// LinkCodeTTL — срок кода подключения (A-41: 60 минут).
	LinkCodeTTL time.Duration
	// LinkCodesPerHour — предел кодов организации в час (R-2).
	LinkCodesPerHour int
	// UnclaimedTTL — сколько канал ждёт кода, прежде чем бот уйдёт (A-41: 7 суток).
	UnclaimedTTL time.Duration
	// RknThreshold — порог подписчиков, с которого нужен номер РКН (D-11).
	RknThreshold int
	// Pepper — секрет HMAC кода (R-2); тот же, что у кодов входа.
	Pepper []byte
	// BotName — имя бота платформы в инструкции, например @maxim_ads_bot.
	BotName string
	// EarlyCodeWindow — сколько ждать bot_added для кода в неизвестном канале (R-6).
	EarlyCodeWindow time.Duration
	// EarlyCodeRetry — через сколько повторить разбор такого кода.
	EarlyCodeRetry time.Duration
}

func (c *Config) defaults() {
	if c.LinkCodeTTL == 0 {
		c.LinkCodeTTL = time.Hour
	}
	if c.LinkCodesPerHour == 0 {
		c.LinkCodesPerHour = 10
	}
	if c.UnclaimedTTL == 0 {
		c.UnclaimedTTL = 7 * 24 * time.Hour
	}
	if c.RknThreshold == 0 {
		c.RknThreshold = 10000
	}
	if c.EarlyCodeWindow == 0 {
		c.EarlyCodeWindow = 5 * time.Minute
	}
	if c.EarlyCodeRetry == 0 {
		c.EarlyCodeRetry = time.Minute
	}
}

// Alerter — оповещения администратору (07 §3).
type Alerter interface {
	Raise(ctx context.Context, a alert.Alert) error
}

// Deps — зависимости сервиса; связывает bootstrap. ChatReader и ChatManager нужны
// только в режиме worker.
type Deps struct {
	Channels     domain.Channels
	LinkCodes    domain.LinkCodes
	Categories   domain.Categories
	Chats        domain.ChatReader
	Bot          domain.ChatManager
	Availability domain.AvailabilityListener
	Alerts       Alerter
	Tx           tx.Runner
	Jobs         job.Enqueuer
	Audit        audit.Recorder
	Clock        clock.Clock
	Log          *slog.Logger
	Config       Config
}

// Service — сценарии модуля каналов.
type Service struct {
	Deps
	log *slog.Logger
}

// New создаёт сервис.
func New(d Deps) *Service {
	d.Config.defaults()
	if d.Availability == nil {
		d.Availability = NopAvailability{Log: d.Log}
	}
	return &Service{Deps: d, log: d.Log.With(logx.KeyModule, "channel")}
}

// NopAvailability — пустая реализация AvailabilityListener до модуля размещений (R-11).
type NopAvailability struct{ Log *slog.Logger }

func (n NopAvailability) ChannelUnavailable(ctx context.Context, c domain.StatusChange) error {
	n.Log.DebugContext(ctx, "channel availability lost, no placements module yet", logx.KeyOp, "Channel.Availability",
		KeyChannelID, c.ChannelID, "from", c.From, "to", c.To)
	return nil
}

func (n NopAvailability) ChannelRestored(ctx context.Context, c domain.StatusChange) error {
	n.Log.DebugContext(ctx, "channel availability restored, no placements module yet", logx.KeyOp, "Channel.Availability",
		KeyChannelID, c.ChannelID, "from", c.From, "to", c.To)
	return nil
}

// save сохраняет канал и сообщает о смене доступности в той же транзакции (R-11).
// Смена состояния пишется в журнал действий и журнал приложения.
func (s *Service) save(ctx context.Context, op string, c *domain.Channel, change domain.StatusChange) error {
	if err := s.Channels.Save(ctx, c); err != nil {
		if errors.Is(err, domain.ErrStaleVersion) {
			return apperr.Conflict("channel was changed concurrently").WithCause(err)
		}
		return err
	}
	if !change.Changed() {
		return nil
	}
	switch {
	case change.AvailabilityLost():
		if err := s.Availability.ChannelUnavailable(ctx, change); err != nil {
			return err
		}
	case change.AvailabilityRestored():
		if err := s.Availability.ChannelRestored(ctx, change); err != nil {
			return err
		}
	}
	if err := s.Audit.Record(ctx, audit.Entry{
		Action: "channel.status_changed", ObjectType: "channel", ObjectID: c.ID.String(),
		Data:           map[string]any{"from": change.From, "to": change.To, "reason": change.Reason},
		OrganizationID: c.OrganizationID,
	}); err != nil {
		return err
	}
	level := slog.LevelInfo
	if change.AvailabilityLost() {
		level = slog.LevelWarn
	}
	s.log.Log(ctx, level, "channel status changed", logx.KeyOp, op, KeyChannelID, c.ID,
		"from", change.From, "to", change.To, "reason", change.Reason, "missing", c.MissingPermissions())
	return nil
}

func orgOf(c *domain.Channel) uuid.UUID {
	if c.OrganizationID == nil {
		return uuid.Nil
	}
	return *c.OrganizationID
}
