package bootstrap

import (
	"context"
	"errors"
	"io"
	"log/slog"
)

// Временные заглушки режимов: заменяются в задачах 12, 13 плана.

func runAPI(ctx context.Context, cfg Config, log *slog.Logger) error {
	return errors.New("api mode is not wired yet")
}

func runLegal(ctx context.Context, cfg Config, log *slog.Logger, args []string, out io.Writer) error {
	return errors.New("legal command is not wired yet")
}
