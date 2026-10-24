package bootstrap

import (
	"net/url"
	"strings"
	"time"
)

// NotificationConfig — уведомления (этап «Уведомления и отчёт по кампании», R-16, R-17).
type NotificationConfig struct {
	// CabinetBaseURL — адрес кабинета для ссылок в письмах и сообщениях бота (R-17).
	// Пусто вне production — devCabinetBaseURL.
	CabinetBaseURL string `env:"CABINET_BASE_URL"`
	// SendAttempts — max_attempts задания send_notification (07 §2).
	SendAttempts int `env:"NOTIFICATION_SEND_ATTEMPTS" envDefault:"5"`
	// Retention — срок хранения отправленных и прочитанных уведомлений (R-16, 04 §8).
	Retention time.Duration `env:"NOTIFICATION_RETENTION" envDefault:"8760h"` // 365 суток
}

// devCabinetBaseURL — кабинет разработки (Vite).
const devCabinetBaseURL = "http://localhost:5173"

// CabinetBaseURL — адрес кабинета без завершающей косой черты.
func (c Config) CabinetBaseURL() string {
	u := strings.TrimRight(strings.TrimSpace(c.Notification.CabinetBaseURL), "/")
	if u == "" && c.Env != EnvProduction {
		return devCabinetBaseURL
	}
	return u
}

// validateNotification — адрес кабинета (https в production), попытки и срок хранения.
func (c Config) validateNotification(add func(field, reason string)) {
	u, err := url.Parse(c.CabinetBaseURL())
	if err != nil || u.Host == "" || (u.Scheme != "https" && (c.Env == EnvProduction || u.Scheme != "http")) {
		add("CABINET_BASE_URL", "must be an absolute URL (https in production)")
	}
	if c.Notification.SendAttempts < 1 || c.Notification.SendAttempts > 20 {
		add("NOTIFICATION_SEND_ATTEMPTS", "must be 1..20")
	}
	if c.Notification.Retention < 24*time.Hour {
		add("NOTIFICATION_RETENTION", "must be at least 24h")
	}
}
