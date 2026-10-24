package bootstrap

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestNotificationConfig(t *testing.T) {
	cfg, err := LoadConfig(validAPIEnv(), ModeAPI)
	if err != nil {
		t.Fatal(err)
	}
	n := cfg.Notification
	if n.SendAttempts != 5 || n.Retention != 8760*time.Hour || cfg.CabinetBaseURL() != devCabinetBaseURL {
		t.Fatalf("notification defaults: %+v, cabinet %q", n, cfg.CabinetBaseURL())
	}
	if !strings.Contains(cfg.LogValue().String(), "cabinet_base_url") {
		t.Fatal("cabinet_base_url is not logged")
	}

	for _, tc := range []struct {
		key, value string
		ok         bool
	}{
		{"CABINET_BASE_URL", "https://cabinet.example.ru/", true},
		{"CABINET_BASE_URL", "http://localhost:5173", true},
		{"CABINET_BASE_URL", "cabinet.example.ru", false},
		{"CABINET_BASE_URL", "ftp://cabinet.example.ru", false},
		{"NOTIFICATION_SEND_ATTEMPTS", "0", false},
		{"NOTIFICATION_SEND_ATTEMPTS", "1", true},
		{"NOTIFICATION_SEND_ATTEMPTS", "20", true},
		{"NOTIFICATION_SEND_ATTEMPTS", "21", false},
		{"NOTIFICATION_RETENTION", "1h", false},
		{"NOTIFICATION_RETENTION", "24h", true},
	} {
		env := validAPIEnv()
		env[tc.key] = tc.value
		_, err := LoadConfig(env, ModeAPI)
		switch {
		case tc.ok && err != nil:
			t.Errorf("%s=%s: %v", tc.key, tc.value, err)
		case !tc.ok && (err == nil || !slices.ContainsFunc(configFields(t, err), func(f string) bool { return strings.HasPrefix(f, tc.key) })):
			t.Errorf("%s=%s: %v", tc.key, tc.value, err)
		}
	}

	env := validAPIEnv()
	env["CABINET_BASE_URL"] = "https://cabinet.example.ru/"
	if cfg, err := LoadConfig(env, ModeAPI); err != nil || cfg.CabinetBaseURL() != "https://cabinet.example.ru" {
		t.Fatalf("trailing slash: %v %q", err, cfg.CabinetBaseURL())
	}

	// production: адрес кабинета обязателен и только https.
	for _, value := range []string{"", "http://cabinet.example.ru"} {
		prod := validWorkerEnv()
		prod["APP_ENV"] = "production"
		prod["CABINET_BASE_URL"] = value
		_, err := LoadConfig(prod, ModeWorker)
		if err == nil || !slices.ContainsFunc(configFields(t, err), func(f string) bool { return strings.HasPrefix(f, "CABINET_BASE_URL") }) {
			t.Errorf("production CABINET_BASE_URL=%q accepted: %v", value, err)
		}
	}
}
