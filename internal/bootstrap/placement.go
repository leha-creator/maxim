package bootstrap

import (
	"github.com/leha-creator/maxim/internal/bootstrap/bridge"
	channelapp "github.com/leha-creator/maxim/internal/channel/app"
	placementapp "github.com/leha-creator/maxim/internal/placement/app"
	"github.com/leha-creator/maxim/internal/placement/domain"
	placementhttp "github.com/leha-creator/maxim/internal/placement/httpapi"
	placementpg "github.com/leha-creator/maxim/internal/placement/postgres"
	"github.com/leha-creator/maxim/internal/platform/alert"
	"github.com/leha-creator/maxim/internal/platform/audit"
	"github.com/leha-creator/maxim/internal/platform/job"
	"github.com/leha-creator/maxim/internal/platform/logx"
	"github.com/leha-creator/maxim/internal/platform/stuck"
	"github.com/leha-creator/maxim/internal/platform/tx"
	postapp "github.com/leha-creator/maxim/internal/post/app"
)

// placementHandlers — псевдоним для встраивания в apiImpl рядом с обработчиками других модулей.
type placementHandlers = placementhttp.Handlers

// stuckScheduledPlacement — проверка зависших размещений в scheduled (07 §3).
const stuckScheduledPlacement = "placement_scheduled_overdue"

// newPlacementService собирает модуль размещений. Порты других модулей — переходники
// пакета bridge; порты будущих модулей — пустые реализации с метками этапов (R-2 … R-4):
//
//	Holds          — UnlimitedHolds, этап «Приём оплаты и учёт денег»
//	ContractorGate — AllowAllContractors, этап «Выплаты блогерам через Рокет Ворк»
//	Notifier       — NopNotifier, этап «Уведомления и отчёт по кампании»
//
// Реализации модуля размещений подставляются в модули каналов и post (R-5, S-25, R-8).
func newPlacementService(cfg Config, in *infra, txRunner tx.Runner, queue job.Scheduler, auditLog audit.Recorder,
	channels *channelapp.Service, posts *postapp.Service) (*placementapp.Service, *placementpg.Store) {
	log := in.log.With(logx.KeyModule, "placement")
	store := placementpg.New(in.pool, log)
	p := cfg.Placement
	svc := placementapp.New(placementapp.Deps{
		Carts: store.Carts(), Orders: store.Orders(), Placements: store.Placements(), History: store.History(),
		Channels: bridge.PlacementChannels{Svc: channels}, Campaigns: bridge.PlacementCampaigns{Svc: posts},
		Versions:    bridge.PlacementVersions{Svc: posts},
		Holds:       bridge.UnlimitedHolds{Log: log},
		Contractors: bridge.AllowAllContractors{Log: log},
		Notifier:    bridge.NopNotifier{Log: log},
		Tx:          txRunner, Jobs: queue, Audit: auditLog, Clock: in.clock, Log: in.log,
		Config: placementapp.Config{
			FeeRateBP: cfg.Channel.FeeRateBP,
			Timing: domain.Timing{
				AcceptTimeout: p.AcceptTimeout, ScheduleTimeout: p.ScheduleTimeout, WindowGuard: p.WindowGuard,
				MinLead: p.MinLead, SlotStep: p.SlotStep, ReeditGuard: p.ReeditGuard,
			},
			Cart: domain.CartRules{MaxItems: p.CartMaxItems},
		},
	})
	posts.SetVersionListener(bridge.VersionListener{Svc: svc})
	link := bridge.ChannelPlacements{Svc: svc}
	channels.SetPlacementPorts(link, link, link)
	return svc, store
}

// stuckScheduledCheck — размещение в scheduled дольше порога после publish_at (07 §3).
func stuckScheduledCheck(cfg Config, svc *placementapp.Service) stuck.Check {
	return stuck.Check{Name: stuckScheduledPlacement, Threshold: cfg.Placement.StuckScheduledAge, Level: alert.LevelError,
		Find: svc.OverdueScheduled}
}
