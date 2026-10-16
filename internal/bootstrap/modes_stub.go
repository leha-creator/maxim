package bootstrap

import (
	"context"
	"errors"
	"io"
	"log/slog"
)

// Временные заглушки режимов: заменяются в задачах 3, 4, 6, 13 плана.

func runAPI(ctx context.Context, cfg Config, log *slog.Logger) error {
	return errors.New("api mode is not wired yet")
}

func runWorker(ctx context.Context, cfg Config, log *slog.Logger) error {
	return errors.New("worker mode is not wired yet")
}

func runMigrate(ctx context.Context, cfg Config, log *slog.Logger, args []string) error {
	return errors.New("migrate mode is not wired yet")
}

func runLegal(ctx context.Context, cfg Config, log *slog.Logger, args []string, out io.Writer) error {
	return errors.New("legal command is not wired yet")
}
