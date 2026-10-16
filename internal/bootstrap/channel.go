package bootstrap

import (
	"context"
	"net/http"
	"slices"
	"strings"

	channelapp "github.com/leha-creator/maxim/internal/channel/app"
	"github.com/leha-creator/maxim/internal/channel/domain"
	channelhttp "github.com/leha-creator/maxim/internal/channel/httpapi"
	channelpg "github.com/leha-creator/maxim/internal/channel/postgres"
	"github.com/leha-creator/maxim/internal/integration/max"
	"github.com/leha-creator/maxim/internal/platform/audit"
	"github.com/leha-creator/maxim/internal/platform/httpx"
	"github.com/leha-creator/maxim/internal/platform/inbound"
	"github.com/leha-creator/maxim/internal/platform/job"
	"github.com/leha-creator/maxim/internal/platform/logx"
	"github.com/leha-creator/maxim/internal/platform/tx"
)

// channelHandlers — псевдоним для встраивания в apiImpl рядом с обработчиками account.
type channelHandlers = channelhttp.Handlers

// maxWebhookPattern — маршрут приёма событий MAX (операция maxWebhook, R-4).
const maxWebhookPattern = "POST " + BaseURL + "/webhooks/max/{secret}"

// channelMAX — переходник адаптера MAX к портам модуля каналов (ARCHITECTURE.md,
// «Модуль → внешняя система»).
type channelMAX struct{ api maxAPI }

var (
	_ domain.ChatReader  = channelMAX{}
	_ domain.ChatManager = channelMAX{}
)

func (c channelMAX) GetChat(ctx context.Context, chatID int64) (domain.ChatInfo, error) {
	chat, err := c.api.GetChat(ctx, chatID)
	if err != nil {
		return domain.ChatInfo{}, err
	}
	return domain.ChatInfo{
		Title: chat.Title, Description: chat.Description, IconURL: chat.IconURL(), Link: chat.Link,
		IsPublic: chat.IsPublic, IsChannel: chat.Type == max.ChatTypeChannel, SubscribersCount: chat.ParticipantsCount,
	}, nil
}

func (c channelMAX) GetMembership(ctx context.Context, chatID int64) (domain.Membership, error) {
	m, err := c.api.GetMembership(ctx, chatID)
	if err != nil {
		return domain.Membership{}, err
	}
	return domain.Membership{IsAdmin: m.IsAdmin, Permissions: m.Permissions}, nil
}

func (c channelMAX) DeleteMessage(ctx context.Context, chatID int64, messageID string) error {
	return c.api.DeleteMessage(ctx, chatID, messageID)
}

func (c channelMAX) LeaveChat(ctx context.Context, chatID int64) error {
	return c.api.LeaveChat(ctx, chatID)
}

func (c channelMAX) SendUserMessage(ctx context.Context, userID int64, text string) (string, error) {
	return c.api.SendUserMessage(ctx, userID, text)
}

// newChannelService собирает модуль каналов. Порты MAX подключаются только в worker.
func newChannelService(cfg Config, in *infra, txRunner tx.Runner, queue job.Enqueuer, auditLog audit.Recorder) *channelapp.Service {
	store := channelpg.New(in.pool, in.log.With(logx.KeyModule, "channel"))
	deps := channelapp.Deps{
		Channels: store.Channels(), LinkCodes: store.LinkCodes(), Categories: store.Categories(),
		Alerts: in.alerter, Tx: txRunner, Jobs: queue, Audit: auditLog, Clock: in.clock, Log: in.log,
		Config: channelapp.Config{
			LinkCodeTTL: cfg.Channel.LinkCodeTTL, LinkCodesPerHour: cfg.Channel.LinkCodesPerHour,
			UnclaimedTTL: cfg.Channel.UnclaimedTTL, RknThreshold: cfg.Channel.RknThreshold,
			Pepper: []byte(cfg.AuthCodePepper), BotName: cfg.MAX.BotName,
		},
	}
	if in.max != nil {
		port := channelMAX{api: in.max}
		deps.Chats, deps.Bot = port, port
	}
	// Availability не задан: до модуля размещений работает пустая реализация (R-11).
	return channelapp.New(deps)
}

// newMAXWebhook — приём событий MAX (R-3, R-4) с данными клиента для журнала.
func newMAXWebhook(cfg Config, in *infra, txRunner tx.Runner, queue job.Enqueuer, trusted httpx.Middleware) http.Handler {
	log := in.log.With(logx.KeyModule, "inbound")
	rec := inbound.NewRecorder(in.pool, txRunner, queue, in.clock, log)
	var reg channelhttp.Registerer
	if in.reg != nil {
		reg = in.reg
	}
	h := channelhttp.NewWebhook(rec, in.alerter, in.clock, in.log, reg, channelhttp.WebhookOptions{
		PathSecret: cfg.MAX.WebhookPathSecret, HeaderSecret: cfg.MAX.WebhookSecret, SecretHeader: max.SecretHeader,
	})
	if cfg.MAX.WebhookPathSecret == "" {
		in.log.Warn("max webhook path secret is not set, all webhook requests are rejected", logx.KeyOp, "Bootstrap.Webhook")
	}
	return httpx.Chain(h, trusted)
}

// maxWebhookURL — полный адрес подписки: публичный адрес и секретная часть пути.
func maxWebhookURL(cfg Config) string {
	return strings.TrimRight(cfg.MAX.WebhookURL, "/") + "/" + cfg.MAX.WebhookPathSecret
}

// ensureMAXSubscription подписывает бота на Webhook, если подписки на наш адрес нет (R-5).
// Неудача не останавливает worker: события догонит сверка, администратор оповещён.
func ensureMAXSubscription(ctx context.Context, cfg Config, in *infra) {
	const op = "Bootstrap.MAXSubscription"
	if cfg.Env != EnvProduction || cfg.MAX.WebhookURL == "" {
		in.log.DebugContext(ctx, "max webhook subscription skipped", logx.KeyOp, op, "env", cfg.Env)
		return
	}
	url := maxWebhookURL(cfg)
	if err := subscribeMAX(ctx, in.max, url, cfg.MAX.WebhookSecret); err != nil {
		in.log.ErrorContext(ctx, "max webhook subscription failed", logx.KeyOp, op, logx.KeyError, err.Error())
		if in.alerter != nil {
			in.alerter.RaiseError(ctx, "max_subscription", "", "Не удалось подписать бота MAX на Webhook", nil)
		}
		return
	}
	in.log.InfoContext(ctx, "max webhook subscription ok", logx.KeyOp, op)
}

// subscribeMAX — подписка, если адреса нет среди текущих подписок бота.
func subscribeMAX(ctx context.Context, api maxAPI, url, secret string) error {
	subs, err := api.ListSubscriptions(ctx)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(subs, func(s max.Subscription) bool { return s.URL == url }) {
		return nil
	}
	return api.Subscribe(ctx, url, max.ChannelUpdateTypes, secret)
}
