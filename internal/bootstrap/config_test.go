package bootstrap

import (
	"bytes"
	"encoding/base64"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func validAPIEnv() map[string]string {
	return map[string]string{
		"APP_ENV":           "development",
		"DATABASE_URL":      "postgres://maxim_app:secret-pass@localhost:5432/maxim",
		"S3_ENDPOINT":       "localhost:9000",
		"S3_BUCKET_PRIVATE": "maxim-private",
		"S3_ACCESS_KEY":     "minio",
		"S3_SECRET_KEY":     "minio-secret",
		"APP_SECRETS_KEY":   base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, 32)),
		"AUTH_CODE_PEPPER":  strings.Repeat("p", 32),
	}
}

func configFields(t *testing.T, err error) []string {
	t.Helper()
	var ce *ConfigError
	if !errors.As(err, &ce) {
		t.Fatalf("want *ConfigError, got %v", err)
	}
	return ce.Fields
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(validAPIEnv(), ModeAPI)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	a := cfg.Auth
	if a.LoginCodeTTL != 10*time.Minute || a.LoginCodeAttempts != 5 || a.CodesPerEmailHour != 5 ||
		a.CodesPerIPHour != 20 || a.SessionTTL != 30*24*time.Hour || a.AdminSessionTTL != 12*time.Hour {
		t.Fatalf("auth defaults differ from 07 §4: %+v", a)
	}
	if cfg.Level() != slog.LevelDebug {
		t.Fatalf("development default level = %v, want debug", cfg.Level())
	}
	if len(cfg.SecretsKeyBytes()) != 32 {
		t.Fatal("secrets key is not 32 bytes")
	}
}

func TestLoadConfigErrors(t *testing.T) {
	cases := []struct {
		name  string
		mode  string
		edit  func(map[string]string)
		field string
	}{
		{"missing database", ModeAPI, func(m map[string]string) { delete(m, "DATABASE_URL") }, "DATABASE_URL: required"},
		{"short secrets key", ModeAPI, func(m map[string]string) { m["APP_SECRETS_KEY"] = base64.StdEncoding.EncodeToString([]byte("short")) }, "APP_SECRETS_KEY: must be 32 bytes in base64"},
		{"short pepper", ModeAPI, func(m map[string]string) { m["AUTH_CODE_PEPPER"] = "x" }, "AUTH_CODE_PEPPER: must be at least 32 characters"},
		{"production without secure cookie", ModeAPI, func(m map[string]string) { m["APP_ENV"] = "production" }, "COOKIE_SECURE: must be true in production"},
		{"unknown env", ModeAPI, func(m map[string]string) { m["APP_ENV"] = "staging" }, "APP_ENV: must be development, test or production"},
		{"bad log level", ModeAPI, func(m map[string]string) { m["LOG_LEVEL"] = "verbose" }, "LOG_LEVEL: must be debug, info, warn or error"},
		{"missing storage", ModeAPI, func(m map[string]string) { delete(m, "S3_ENDPOINT") }, "S3_ENDPOINT: required"},
		{"bad proxy cidr", ModeAPI, func(m map[string]string) { m["TRUSTED_PROXY_CIDRS"] = "10.0.0.0/8,nope" }, "TRUSTED_PROXY_CIDRS: invalid CIDR"},
		{"worker without smtp", ModeWorker, func(m map[string]string) {}, "SMTP_HOST: required"},
		{"bad duration", ModeAPI, func(m map[string]string) { m["AUTH_SESSION_TTL"] = "month" }, "AUTH_SESSION_TTL: invalid value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := validAPIEnv()
			c.edit(e)
			_, err := LoadConfig(e, c.mode)
			fields := configFields(t, err)
			if !slices.Contains(fields, c.field) {
				t.Fatalf("fields %v do not contain %q", fields, c.field)
			}
			if strings.Contains(err.Error(), "secret-pass") {
				t.Fatal("error text leaks a secret")
			}
		})
	}
}

func TestMigrateNeedsOnlyDatabase(t *testing.T) {
	if _, err := LoadConfig(map[string]string{"DATABASE_URL": "postgres://x@y/z"}, ModeMigrate); err != nil {
		t.Fatalf("migrate: %v", err)
	}
}

func TestConfigLogValueHidesSecrets(t *testing.T) {
	cfg, err := LoadConfig(validAPIEnv(), ModeAPI)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("config", "config", cfg)
	out := buf.String()
	for _, secret := range []string{"secret-pass", "minio-secret", cfg.SecretsKey, cfg.AuthCodePepper} {
		if strings.Contains(out, secret) {
			t.Fatalf("log contains secret %q: %s", secret, out)
		}
	}
	if !strings.Contains(out, "maxim_app:***@localhost") {
		t.Fatalf("database url is not redacted as expected: %s", out)
	}
}

func validWorkerEnv() map[string]string {
	e := validAPIEnv()
	e["SMTP_HOST"] = "localhost"
	e["MAIL_FROM"] = "noreply@maxim.test"
	return e
}

func TestWorkerConfig(t *testing.T) {
	cfg, err := LoadConfig(validWorkerEnv(), ModeWorker)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.MAX.RateGlobal != 25 || cfg.MAX.RatePerChat != 1 || cfg.Scheduler.DowntimeThreshold != 30*time.Minute ||
		cfg.Scheduler.StuckInboundEventAge != 10*time.Minute || cfg.Retention.OutboundRequest != 180*24*time.Hour {
		t.Fatalf("defaults differ from 05 §2.8, 07 §2–§3, 04 §8: %+v %+v %+v", cfg.MAX, cfg.Scheduler, cfg.Retention)
	}
	if !cfg.MAXFake() {
		t.Fatal("development without MAX_BOT_TOKEN must use the fake adapter")
	}

	pemFile := filepath.Join(t.TempDir(), "bad.pem")
	if err := os.WriteFile(pemFile, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name  string
		edit  func(map[string]string)
		field string
	}{
		{"production without bot token", func(m map[string]string) { m["APP_ENV"] = "production"; m["ALERT_EMAILS"] = "oncall@maxim.ru" }, "MAX_BOT_TOKEN: required in production"},
		{"production without alert recipients", func(m map[string]string) { m["APP_ENV"] = "production"; m["MAX_BOT_TOKEN"] = "t" },
			"ALERT_EMAILS: at least one alert recipient (ALERT_EMAILS or ALERT_MAX_CHAT_ID) is required in production"},
		{"bad pem", func(m map[string]string) { m["MAX_CA_CERT_FILE"] = pemFile }, "MAX_CA_CERT_FILE: no PEM certificates found"},
		{"missing pem", func(m map[string]string) { m["MAX_CA_CERT_FILE"] = pemFile + ".missing" }, "MAX_CA_CERT_FILE: cannot be read"},
		{"zero rate", func(m map[string]string) { m["MAX_RATE_GLOBAL"] = "0" }, "MAX_RATE_*: must be positive"},
		{"bad alert email", func(m map[string]string) { m["ALERT_EMAILS"] = "oncall@maxim.ru,nope" }, "ALERT_EMAILS: invalid address"},
		{"zero retention", func(m map[string]string) { m["RETENTION_SESSION"] = "0s" }, "RETENTION_*: must be positive"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := validWorkerEnv()
			c.edit(e)
			_, err := LoadConfig(e, ModeWorker)
			if fields := configFields(t, err); !slices.Contains(fields, c.field) {
				t.Fatalf("fields %v do not contain %q", fields, c.field)
			}
		})
	}
}

func TestConfigLogValueHidesBotToken(t *testing.T) {
	e := validWorkerEnv()
	e["MAX_BOT_TOKEN"] = "max-bot-secret"
	cfg, err := LoadConfig(e, ModeWorker)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	slog.New(slog.NewJSONHandler(&buf, nil)).Info("config", "config", cfg)
	if strings.Contains(buf.String(), "max-bot-secret") {
		t.Fatal("log contains MAX bot token")
	}
	if cfg.MAXFake() {
		t.Fatal("development with MAX_BOT_TOKEN must use the live adapter")
	}
}
