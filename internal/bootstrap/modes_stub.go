package bootstrap

import (
	"context"
	"errors"
	"io"
	"log/slog"
)

// Временная заглушка: заменяется в задаче 13 плана.
func runLegal(ctx context.Context, cfg Config, log *slog.Logger, args []string, out io.Writer) error {
	return errors.New("legal command is not wired yet")
}
