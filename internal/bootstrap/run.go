// Package bootstrap — точка сборки приложения: конфигурация, пулы, адаптеры,
// модули, HTTP-сервер и обработчик заданий. Только этот пакет знает обо всех
// остальных и связывает порты с реализациями.
package bootstrap

import (
	"context"
	"errors"
	"io"
	"log/slog"

	"github.com/caarlos0/env/v11"

	"github.com/leha-creator/maxim/internal/platform/logx"
)

// RunParams — всё, что режиму нужно от процесса.
type RunParams struct {
	Command string
	Args    []string
	Environ []string
	Version string
	Stdout  io.Writer
	Stderr  io.Writer
}

// errUsage — неверные аргументы командной строки: код выхода 2.
var errUsage = errors.New("usage")

// Run выполняет команду и возвращает код выхода процесса.
func Run(ctx context.Context, p RunParams) int {
	mode := p.Command
	switch mode {
	case ModeAPI, ModeWorker, ModeMigrate, ModeLegal:
	default:
		return 2
	}

	component := mode
	if mode == ModeLegal {
		component = "cli"
	}
	cfg, err := LoadConfig(env.ToMap(p.Environ), mode)
	if err != nil {
		log := logx.New(p.Stdout, slog.LevelInfo, component)
		attrs := []any{logx.KeyOp, "Bootstrap.Config"}
		var ce *ConfigError
		if errors.As(err, &ce) {
			attrs = append(attrs, "fields", ce.Fields)
		}
		log.Error("config invalid", attrs...)
		return 1
	}
	log := logx.New(p.Stdout, cfg.Level(), component)
	log.Debug("config loaded", logx.KeyOp, "Bootstrap.Config", "config", cfg)
	log.Info("app started", logx.KeyOp, "Bootstrap.Run", "mode", mode, "version", p.Version, "env", cfg.Env)

	switch mode {
	case ModeAPI:
		err = runAPI(ctx, cfg, log)
	case ModeWorker:
		err = runWorker(ctx, cfg, log)
	case ModeMigrate:
		err = runMigrate(ctx, cfg, log, p.Args)
	case ModeLegal:
		err = runLegal(ctx, cfg, log, p.Args, p.Stdout)
	}
	if errors.Is(err, errUsage) {
		log.Error("invalid arguments", logx.KeyOp, "Bootstrap.Run", "mode", mode)
		return 2
	}
	if err != nil {
		log.Error("app failed", logx.KeyOp, "Bootstrap.Run", "mode", mode, logx.KeyError, err.Error())
		return 1
	}
	log.Info("app stopped", logx.KeyOp, "Bootstrap.Run", "mode", mode)
	return 0
}
