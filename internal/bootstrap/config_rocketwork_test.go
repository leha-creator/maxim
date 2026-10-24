package bootstrap

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestRocketWorkConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(validWorkerEnv(), ModeWorker)
	if err != nil {
		t.Fatal(err)
	}
	r := cfg.RocketWork
	if cfg.RocketWorkMode() != RocketWorkFake || r.Rate != 5 || r.DepositCheckInterval != time.Hour || r.DepositTolerance != 100 ||
		r.ContractorPollMaxAge != 720*time.Hour || r.PollFast != 5*time.Minute || r.PollSlow != time.Hour ||
		r.NPDAnnualLimit != 240_000_000 {
		t.Fatalf("defaults: %+v", r)
	}
	if len(cfg.RocketWorkWebhookSecret()) == 0 {
		t.Fatal("development fake has no webhook secret")
	}
	p := cfg.PayoutAppConfig()
	if !p.ProviderEnabled || p.PollFast != 5*time.Minute || p.DepositTolerance != 100 || p.NPDAnnualLimit != 240_000_000 {
		t.Fatalf("payout config: %+v", p)
	}
	if strings.Contains(cfg.LogValue().String(), rocketWorkDevSecret) {
		t.Fatal("secret in the log")
	}
}

func TestRocketWorkConfigErrors(t *testing.T) {
	env := func(extra map[string]string) map[string]string {
		e := validWorkerEnv()
		for k, v := range extra {
			e[k] = v
		}
		return e
	}
	live := map[string]string{"ROCKETWORK_MODE": "live", "ROCKETWORK_TOKEN": "rw-token", "ROCKETWORK_WEBHOOK_SECRET": "rw-webhook-secret-0123"}
	cfg, err := LoadConfig(env(live), ModeWorker)
	if err != nil {
		t.Fatalf("live config: %v", err)
	}
	if strings.Contains(cfg.LogValue().String(), "rw-token") || strings.Contains(cfg.LogValue().String(), "rw-webhook-secret") {
		t.Fatal("rocketwork secrets in the log")
	}
	with := func(base map[string]string, k, v string) map[string]string {
		e := env(base)
		e[k] = v
		return e
	}
	for name, tc := range map[string]struct {
		env   map[string]string
		field string
	}{
		"unknown mode":        {env(map[string]string{"ROCKETWORK_MODE": "sandbox"}), "ROCKETWORK_MODE"},
		"live without token":  {with(live, "ROCKETWORK_TOKEN", ""), "ROCKETWORK_TOKEN"},
		"live without secret": {with(live, "ROCKETWORK_WEBHOOK_SECRET", ""), "ROCKETWORK_WEBHOOK_SECRET"},
		"short secret":        {env(map[string]string{"ROCKETWORK_WEBHOOK_SECRET": "short"}), "ROCKETWORK_WEBHOOK_SECRET"},
		"bad url":             {with(live, "ROCKETWORK_BASE_URL", "api.rocketwork.ru"), "ROCKETWORK_BASE_URL"},
		"zero rate":           {with(live, "ROCKETWORK_RATE", "0"), "ROCKETWORK_RATE"},
		"fast over slow":      {env(map[string]string{"PAYOUT_POLL_FAST": "2h"}), "PAYOUT_POLL_FAST"},
		"negative tolerance":  {env(map[string]string{"ROCKETWORK_DEPOSIT_TOLERANCE": "-1"}), "ROCKETWORK_DEPOSIT_TOLERANCE"},
		"zero npd limit":      {env(map[string]string{"PAYOUT_NPD_ANNUAL_LIMIT": "0"}), "PAYOUT_NPD_ANNUAL_LIMIT"},
		"zero check interval": {env(map[string]string{"ROCKETWORK_DEPOSIT_CHECK_INTERVAL": "0s"}), "ROCKETWORK_DEPOSIT_CHECK_INTERVAL"},
	} {
		_, err := LoadConfig(tc.env, ModeWorker)
		if err == nil {
			t.Errorf("%s: accepted", name)
			continue
		}
		if !slices.ContainsFunc(configFields(t, err), func(f string) bool { return strings.HasPrefix(f, tc.field+":") }) {
			t.Errorf("%s: fields %v, want %s", name, configFields(t, err), tc.field)
		}
	}
}

// R-1: в production по умолчанию disabled, подделка запрещена.
func TestRocketWorkConfigProduction(t *testing.T) {
	cfg := Config{Env: EnvProduction}
	if cfg.RocketWorkMode() != RocketWorkDisabled || cfg.PayoutAppConfig().ProviderEnabled {
		t.Fatalf("production default mode %s", cfg.RocketWorkMode())
	}
	if len(cfg.RocketWorkWebhookSecret()) != 0 {
		t.Fatal("development secret in production")
	}
	var fields []string
	cfg.RocketWork = RocketWorkConfig{Mode: RocketWorkFake, Rate: 5, DepositCheckInterval: time.Hour, ContractorPollMaxAge: time.Hour,
		PollFast: time.Minute, PollSlow: time.Hour, NPDAnnualLimit: 1}
	cfg.validateRocketWork(func(field, reason string) { fields = append(fields, field+": "+reason) })
	if !slices.Contains(fields, "ROCKETWORK_MODE: fake is not allowed in production") {
		t.Fatalf("fake in production: %v", fields)
	}
}
