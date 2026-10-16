package bootstrap

import (
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/mail"
	"net/netip"
	"net/url"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/caarlos0/env/v11"

	"github.com/leha-creator/maxim/internal/platform/logx"
)

// Окружения (07 §8).
const (
	EnvDevelopment = "development"
	EnvTest        = "test"
	EnvProduction  = "production"
)

// Режимы запуска исполняемого файла.
const (
	ModeAPI     = "api"
	ModeWorker  = "worker"
	ModeMigrate = "migrate"
	ModeLegal   = "legal"
)

// Config — вся конфигурация приложения. Источник — только переменные окружения (07 §8).
type Config struct {
	Env      string `env:"APP_ENV" envDefault:"development"`
	LogLevel string `env:"LOG_LEVEL"` // пусто: debug в разработке, info в остальных окружениях

	HTTPAddr      string `env:"HTTP_ADDR" envDefault:":8080"`
	MetricsAddr   string `env:"METRICS_ADDR" envDefault:":9090"`
	PublicBaseURL string `env:"PUBLIC_BASE_URL" envDefault:"http://localhost:8080"`
	// TrustedProxyCIDRs — адреса прокси, чьему X-Forwarded-For можно верить (edge-прокси стенда).
	TrustedProxyCIDRs []string `env:"TRUSTED_PROXY_CIDRS" envSeparator:","`
	CookieSecure      bool     `env:"COOKIE_SECURE" envDefault:"false"`

	DatabaseURL string `env:"DATABASE_URL"`
	// DBAppRole — роль, под которой работает приложение; миграции выдают ей права (04 §1).
	DBAppRole      string `env:"DB_APP_ROLE" envDefault:"maxim_app"`
	DBMaxConns     int32  `env:"DB_MAX_CONNS" envDefault:"10"`
	S3             S3Config
	SMTP           SMTPConfig
	MailFrom       string `env:"MAIL_FROM"`
	SecretsKey     string `env:"APP_SECRETS_KEY"`  // base64, 32 байта: шифрование секретов в job.payload (R-1)
	AuthCodePepper string `env:"AUTH_CODE_PEPPER"` // секрет HMAC кода входа (R-3)

	Auth AuthConfig

	MAX       MAXConfig
	Alert     AlertConfig
	Scheduler SchedulerConfig
	Retention RetentionConfig
}

// MAXConfig — MAX Bot API (05 §2.1, 05 §2.8).
type MAXConfig struct {
	APIURL string `env:"MAX_API_URL" envDefault:"https://platform-api2.max.ru"`
	// BotToken — токен бота; пусто в разработке — поддельный адаптер.
	BotToken string `env:"MAX_BOT_TOKEN"`
	// CACertFile — корневой сертификат Минцифры (PEM), добавляется к системным.
	CACertFile  string  `env:"MAX_CA_CERT_FILE"`
	RateGlobal  float64 `env:"MAX_RATE_GLOBAL" envDefault:"25"`
	RatePerChat float64 `env:"MAX_RATE_PER_CHAT" envDefault:"1"`
}

// AlertConfig — адресаты оповещений администратору (07 §3, R-5).
type AlertConfig struct {
	Emails    []string `env:"ALERT_EMAILS" envSeparator:","`
	MAXChatID int64    `env:"ALERT_MAX_CHAT_ID"`
}

// SchedulerConfig — пороги планировщика и контроля (07 §2, §3).
type SchedulerConfig struct {
	DowntimeThreshold    time.Duration `env:"DOWNTIME_THRESHOLD" envDefault:"30m"`
	StuckInboundEventAge time.Duration `env:"STUCK_INBOUND_EVENT_AGE" envDefault:"10m"`
}

// RetentionConfig — сроки хранения для задания cleanup (04 §8, R-10).
type RetentionConfig struct {
	OutboundRequest time.Duration `env:"RETENTION_OUTBOUND_REQUEST" envDefault:"4320h"` // 180 суток
	JobFinished     time.Duration `env:"RETENTION_JOB_FINISHED" envDefault:"720h"`      // 30 суток
	JobFailed       time.Duration `env:"RETENTION_JOB_FAILED" envDefault:"4320h"`
	InboundEvent    time.Duration `env:"RETENTION_INBOUND_EVENT" envDefault:"4320h"`
	AuthCode        time.Duration `env:"RETENTION_AUTH_CODE" envDefault:"168h"` // 7 суток после истечения
	Session         time.Duration `env:"RETENTION_SESSION" envDefault:"168h"`
	Alert           time.Duration `env:"RETENTION_ALERT" envDefault:"2160h"` // 90 суток после закрытия
}

// MAXFake сообщает, что вместо MAX работает поддельный адаптер: в тестовом
// окружении всегда, в разработке — без токена бота.
func (c Config) MAXFake() bool {
	return c.Env == EnvTest || (c.Env == EnvDevelopment && c.MAX.BotToken == "")
}

// S3Config — объектное хранилище (07 §1).
type S3Config struct {
	Endpoint      string `env:"S3_ENDPOINT"`
	Region        string `env:"S3_REGION" envDefault:"ru-central1"`
	BucketPrivate string `env:"S3_BUCKET_PRIVATE"`
	AccessKey     string `env:"S3_ACCESS_KEY"`
	SecretKey     string `env:"S3_SECRET_KEY"`
	UseSSL        bool   `env:"S3_USE_SSL" envDefault:"true"`
}

// SMTPConfig — почтовый сервис (A-13, A-19).
type SMTPConfig struct {
	Host     string `env:"SMTP_HOST"`
	Port     int    `env:"SMTP_PORT" envDefault:"587"`
	User     string `env:"SMTP_USER"`
	Password string `env:"SMTP_PASSWORD"`
	// TLS: `starttls` (по умолчанию), `tls` — сразу TLS, `none` — без шифрования (только разработка).
	TLS string `env:"SMTP_TLS" envDefault:"starttls"`
}

// AuthConfig — параметры входа и сессий, значения по умолчанию из 07 §4.
type AuthConfig struct {
	LoginCodeTTL      time.Duration `env:"AUTH_LOGIN_CODE_TTL" envDefault:"600s"`
	LoginCodeAttempts int           `env:"AUTH_LOGIN_CODE_ATTEMPTS" envDefault:"5"`
	CodesPerEmailHour int           `env:"AUTH_CODES_PER_EMAIL_HOUR" envDefault:"5"`
	CodesPerIPHour    int           `env:"AUTH_CODES_PER_IP_HOUR" envDefault:"20"`
	SessionTTL        time.Duration `env:"AUTH_SESSION_TTL" envDefault:"720h"`
	AdminSessionTTL   time.Duration `env:"AUTH_ADMIN_SESSION_TTL" envDefault:"12h"`
}

// ConfigError перечисляет поля с ошибками. Значения полей в текст не попадают.
type ConfigError struct {
	Fields []string
}

func (e *ConfigError) Error() string {
	return "invalid configuration: " + strings.Join(e.Fields, ", ")
}

// LoadConfig читает конфигурацию из environ (обычно — env.ToMap(os.Environ()))
// и проверяет её для режима mode.
func LoadConfig(environ map[string]string, mode string) (Config, error) {
	var cfg Config
	if err := env.ParseWithOptions(&cfg, env.Options{Environment: environ}); err != nil {
		return Config{}, &ConfigError{Fields: parseErrorFields(err)}
	}
	if err := cfg.Validate(mode); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func parseErrorFields(err error) []string {
	var agg env.AggregateError
	if !errors.As(err, &agg) {
		return []string{err.Error()}
	}
	keys := envKeys(reflect.TypeFor[Config]())
	fields := make([]string, 0, len(agg.Errors))
	for _, e := range agg.Errors {
		var pe env.ParseError
		if errors.As(e, &pe) {
			name := pe.Name
			if key, ok := keys[name]; ok {
				name = key
			}
			fields = append(fields, name+": invalid value")
			continue
		}
		fields = append(fields, "configuration: "+reflect.TypeOf(e).String())
	}
	return fields
}

// envKeys сопоставляет имя поля структуры с переменной окружения: ошибка
// разбора называет поле, а пользователю нужна переменная.
func envKeys(t reflect.Type) map[string]string {
	out := map[string]string{}
	for i := range t.NumField() {
		f := t.Field(i)
		if f.Type.Kind() == reflect.Struct && f.Tag.Get("env") == "" && f.Type != reflect.TypeFor[time.Duration]() {
			maps.Copy(out, envKeys(f.Type))
			continue
		}
		if key, _, _ := strings.Cut(f.Tag.Get("env"), ","); key != "" {
			out[f.Name] = key
		}
	}
	return out
}

// Validate проверяет конфигурацию. Обязательность полей зависит от режима:
// migrate нужна только база, legal — база и хранилище.
func (c Config) Validate(mode string) error {
	var bad []string
	add := func(field, reason string) { bad = append(bad, field+": "+reason) }
	require := func(field, value string) {
		if strings.TrimSpace(value) == "" {
			add(field, "required")
		}
	}

	switch c.Env {
	case EnvDevelopment, EnvTest, EnvProduction:
	default:
		add("APP_ENV", "must be development, test or production")
	}
	if _, ok := logx.ParseLevel(c.LogLevel, slog.LevelInfo); !ok {
		add("LOG_LEVEL", "must be debug, info, warn or error")
	}

	require("DATABASE_URL", c.DatabaseURL)
	if mode == ModeMigrate {
		require("DB_APP_ROLE", c.DBAppRole)
		return finish(bad)
	}
	if c.DBMaxConns < 1 {
		add("DB_MAX_CONNS", "must be positive")
	}

	needStorage := mode == ModeAPI || mode == ModeLegal
	needSecrets := mode == ModeAPI || mode == ModeWorker
	needMail := mode == ModeWorker
	fakes := c.Env == EnvTest // в тестовом окружении внешние системы поддельные

	if needStorage && !fakes {
		require("S3_ENDPOINT", c.S3.Endpoint)
		require("S3_BUCKET_PRIVATE", c.S3.BucketPrivate)
		require("S3_ACCESS_KEY", c.S3.AccessKey)
		require("S3_SECRET_KEY", c.S3.SecretKey)
	}
	if needSecrets {
		if key, err := base64.StdEncoding.DecodeString(c.SecretsKey); err != nil || len(key) != 32 {
			add("APP_SECRETS_KEY", "must be 32 bytes in base64")
		}
	}
	if mode == ModeAPI {
		if len(c.AuthCodePepper) < 32 {
			add("AUTH_CODE_PEPPER", "must be at least 32 characters")
		}
		if c.Env == EnvProduction && !c.CookieSecure {
			add("COOKIE_SECURE", "must be true in production")
		}
		for _, cidr := range c.TrustedProxyCIDRs {
			if _, err := netip.ParsePrefix(strings.TrimSpace(cidr)); err != nil {
				add("TRUSTED_PROXY_CIDRS", "invalid CIDR")
				break
			}
		}
		a := c.Auth
		if a.LoginCodeTTL <= 0 || a.SessionTTL <= 0 || a.AdminSessionTTL <= 0 {
			add("AUTH_*_TTL", "must be positive")
		}
		if a.LoginCodeAttempts < 1 || a.CodesPerEmailHour < 1 || a.CodesPerIPHour < 1 {
			add("AUTH_* limits", "must be positive")
		}
	}
	if mode == ModeWorker {
		c.validateWorker(add)
	}
	if needMail && !fakes {
		require("SMTP_HOST", c.SMTP.Host)
		require("MAIL_FROM", c.MailFrom)
		switch c.SMTP.TLS {
		case "starttls", "tls":
		case "none":
			if c.Env == EnvProduction {
				add("SMTP_TLS", "none is not allowed in production")
			}
		default:
			add("SMTP_TLS", "must be starttls, tls or none")
		}
	}
	return finish(bad)
}

// validateWorker — MAX, оповещения, пороги и сроки хранения режима worker.
func (c Config) validateWorker(add func(field, reason string)) {
	if c.Env == EnvProduction {
		if strings.TrimSpace(c.MAX.BotToken) == "" {
			add("MAX_BOT_TOKEN", "required in production")
		}
		if len(c.Alert.Emails) == 0 && c.Alert.MAXChatID == 0 {
			add("ALERT_EMAILS", "at least one alert recipient (ALERT_EMAILS or ALERT_MAX_CHAT_ID) is required in production")
		}
	}
	if u, err := url.Parse(c.MAX.APIURL); err != nil || u.Scheme == "" || u.Host == "" {
		add("MAX_API_URL", "must be an absolute URL")
	}
	if c.MAX.CACertFile != "" {
		if pem, err := os.ReadFile(c.MAX.CACertFile); err != nil {
			add("MAX_CA_CERT_FILE", "cannot be read")
		} else if !x509.NewCertPool().AppendCertsFromPEM(pem) {
			add("MAX_CA_CERT_FILE", "no PEM certificates found")
		}
	}
	if c.MAX.RateGlobal <= 0 || c.MAX.RatePerChat <= 0 {
		add("MAX_RATE_*", "must be positive")
	}
	for _, e := range c.Alert.Emails {
		if _, err := mail.ParseAddress(strings.TrimSpace(e)); err != nil {
			add("ALERT_EMAILS", "invalid address")
			break
		}
	}
	if c.Scheduler.DowntimeThreshold <= 0 || c.Scheduler.StuckInboundEventAge <= 0 {
		add("DOWNTIME_THRESHOLD, STUCK_*", "must be positive")
	}
	r := c.Retention
	for _, d := range []time.Duration{r.OutboundRequest, r.JobFinished, r.JobFailed, r.InboundEvent, r.AuthCode, r.Session, r.Alert} {
		if d <= 0 {
			add("RETENTION_*", "must be positive")
			break
		}
	}
}

func finish(bad []string) error {
	if len(bad) == 0 {
		return nil
	}
	sort.Strings(bad)
	return &ConfigError{Fields: bad}
}

// Level — уровень журнала с учётом окружения (07 §5.1).
func (c Config) Level() slog.Level {
	def := slog.LevelInfo
	if c.Env == EnvDevelopment {
		def = slog.LevelDebug
	}
	l, _ := logx.ParseLevel(c.LogLevel, def)
	return l
}

// SecretsKeyBytes — расшифрованный APP_SECRETS_KEY. Вызывать после Validate.
func (c Config) SecretsKeyBytes() []byte {
	key, _ := base64.StdEncoding.DecodeString(c.SecretsKey)
	return key
}

// LogValue выводит конфигурацию без секретов: она пишется в журнал на DEBUG.
func (c Config) LogValue() slog.Value {
	secret := func(v string) string {
		if v == "" {
			return ""
		}
		return "***"
	}
	return slog.GroupValue(
		slog.String("env", c.Env),
		slog.String("log_level", c.Level().String()),
		slog.String("http_addr", c.HTTPAddr),
		slog.String("metrics_addr", c.MetricsAddr),
		slog.String("public_base_url", c.PublicBaseURL),
		slog.Any("trusted_proxy_cidrs", c.TrustedProxyCIDRs),
		slog.Bool("cookie_secure", c.CookieSecure),
		slog.String("database_url", redactURL(c.DatabaseURL)),
		slog.String("db_app_role", c.DBAppRole),
		slog.Int("db_max_conns", int(c.DBMaxConns)),
		slog.String("s3_endpoint", c.S3.Endpoint),
		slog.String("s3_bucket_private", c.S3.BucketPrivate),
		slog.String("s3_access_key", secret(c.S3.AccessKey)),
		slog.String("s3_secret_key", secret(c.S3.SecretKey)),
		slog.Bool("s3_use_ssl", c.S3.UseSSL),
		slog.String("smtp_host", c.SMTP.Host),
		slog.Int("smtp_port", c.SMTP.Port),
		slog.String("smtp_user", secret(c.SMTP.User)),
		slog.String("smtp_password", secret(c.SMTP.Password)),
		slog.String("smtp_tls", c.SMTP.TLS),
		slog.String("app_secrets_key", secret(c.SecretsKey)),
		slog.String("auth_code_pepper", secret(c.AuthCodePepper)),
		slog.String("auth_login_code_ttl", c.Auth.LoginCodeTTL.String()),
		slog.Int("auth_login_code_attempts", c.Auth.LoginCodeAttempts),
		slog.Int("auth_codes_per_email_hour", c.Auth.CodesPerEmailHour),
		slog.Int("auth_codes_per_ip_hour", c.Auth.CodesPerIPHour),
		slog.String("auth_session_ttl", c.Auth.SessionTTL.String()),
		slog.String("auth_admin_session_ttl", c.Auth.AdminSessionTTL.String()),
		slog.String("max_api_url", c.MAX.APIURL),
		slog.String("max_bot_token", secret(c.MAX.BotToken)),
		slog.String("max_ca_cert_file", c.MAX.CACertFile),
		slog.Float64("max_rate_global", c.MAX.RateGlobal),
		slog.Float64("max_rate_per_chat", c.MAX.RatePerChat),
		slog.Int("alert_emails", len(c.Alert.Emails)),
		slog.Bool("alert_max_chat", c.Alert.MAXChatID != 0),
		slog.String("downtime_threshold", c.Scheduler.DowntimeThreshold.String()),
		slog.String("stuck_inbound_event_age", c.Scheduler.StuckInboundEventAge.String()),
	)
}

// redactURL убирает пароль из строки подключения вида postgres://user:pass@host/db.
func redactURL(u string) string {
	scheme := strings.Index(u, "://")
	at := strings.LastIndexByte(u, '@')
	if scheme < 0 || at < scheme {
		return u
	}
	creds := u[scheme+3 : at]
	if colon := strings.IndexByte(creds, ':'); colon >= 0 {
		creds = creds[:colon] + ":***"
	}
	return fmt.Sprintf("%s%s%s", u[:scheme+3], creds, u[at:])
}
