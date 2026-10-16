// Command maxim — бэкенд биржи рекламы в каналах MAX.
//
// Режимы:
//
//	maxim api                       — HTTP API кабинетов
//	maxim worker                    — обработчик заданий
//	maxim migrate up|down|status    — миграции базы
//	maxim legal publish|list        — оферты и правила (до API администратора, R-8)
//	maxim admin grant|revoke        — роль администратора (до API администратора)
//	maxim admin ord errors|retry    — ошибки Яндекс ОРД и повтор (S-20, до API администратора)
//	maxim dev post-version          — двинуть версию поста заказа (только development и test, R-23)
//
// Конфигурация — переменные окружения, см. .env.example.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/leha-creator/maxim/internal/bootstrap"
)

// version задаётся при сборке: -ldflags "-X main.version=<sha>".
var version = "dev"

const usage = `usage: maxim <command> [args]

commands:
  api                      run HTTP API
  worker                   run job worker
  migrate up|down|status   apply, roll back one or show migrations
  legal publish|list       publish a legal document version or list current ones
  admin grant|revoke       grant (--email, --role moderator|senior) or revoke (--email) an admin role
  admin ord errors         list ORD objects in error [--limit N]
  admin ord retry          --object <type>:<id> --reason <text> --email <admin>: retry an ORD object
  dev post-version         --order <number> approve|ready|reject [--erid X] [--reason Y] (development and test only;
                           ready only with ORD_ENABLED=false)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	code := bootstrap.Run(ctx, bootstrap.RunParams{
		Command: os.Args[1],
		Args:    os.Args[2:],
		Environ: os.Environ(),
		Version: version,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	})
	stop()
	if code == 2 {
		fmt.Fprint(os.Stderr, usage)
	}
	os.Exit(code)
}
