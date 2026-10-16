package bootstrap

import (
	"slices"
	"strings"
	"testing"
	"time"
)

func TestORDConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig(validWorkerEnv(), ModeWorker)
	if err != nil {
		t.Fatal(err)
	}
	o := cfg.ORD
	if !o.Enabled || o.APIVersion != "v8" || o.Rate != 5 || o.PlatformType != "site" || o.PlatformLegalType != "sole_proprietor" ||
		o.InitialContractType != "intermediary-contract" || o.InitialSubjectType != "mediation" || o.InitialActionType != "conclude" ||
		o.PublisherContractType != "contract" || o.PublisherSubjectType != "distribution" || o.PublisherActionType != "distribution" ||
		o.PollFastInterval != time.Minute || o.PollFastPeriod != 10*time.Minute || o.PollSlowInterval != 10*time.Minute ||
		o.StatusTimeout != 24*time.Hour || o.DependencyWait != 30*time.Second || o.DependencyWaitFailed != 10*time.Minute ||
		o.EridWait != time.Hour || o.StuckSentAge != 24*time.Hour {
		t.Fatalf("ORD defaults: %+v", o)
	}
	if !cfg.ORDFake() || cfg.ORDBaseURL() != "https://ord-prestable.yandex.net" {
		t.Fatalf("development without token: fake %v, url %s", cfg.ORDFake(), cfg.ORDBaseURL())
	}
	inn, name, _ := cfg.ORDPlatform()
	if inn == "" || name == "" {
		t.Fatal("development platform requisites are empty")
	}
	m := cfg.MarkingConfig()
	if m.Poll.Timeout != 24*time.Hour || m.Contracts.Initial.Type != "intermediary-contract" || m.PlatformType != "site" {
		t.Fatalf("marking config: %+v", m)
	}
}

func TestORDConfigErrors(t *testing.T) {
	real := func(extra map[string]string) map[string]string {
		e := validWorkerEnv()
		e["ORD_OAUTH_TOKEN"] = "ord-oauth-secret"
		e["ORD_KKTU_CODE"] = "30.15.1"
		e["ORD_PLATFORM_INN"] = "7707083893"
		e["ORD_PLATFORM_LEGAL_TYPE"] = "legal_entity"
		e["ORD_PLATFORM_NAME"] = "ООО Платформа"
		for k, v := range extra {
			e[k] = v
		}
		return e
	}
	if _, err := LoadConfig(real(nil), ModeWorker); err != nil {
		t.Fatalf("real adapter config: %v", err)
	}
	for name, tc := range map[string]struct {
		env   map[string]string
		field string
	}{
		"kktu missing":         {real(map[string]string{"ORD_KKTU_CODE": ""}), "ORD_KKTU_CODE"},
		"kktu format":          {real(map[string]string{"ORD_KKTU_CODE": "30.15"}), "ORD_KKTU_CODE"},
		"inn checksum":         {real(map[string]string{"ORD_PLATFORM_INN": "7707083890"}), "ORD_PLATFORM_INN"},
		"inn length for ip":    {real(map[string]string{"ORD_PLATFORM_LEGAL_TYPE": "sole_proprietor"}), "ORD_PLATFORM_INN"},
		"platform name":        {real(map[string]string{"ORD_PLATFORM_NAME": " "}), "ORD_PLATFORM_NAME"},
		"platform type":        {real(map[string]string{"ORD_PLATFORM_TYPE": "channel"}), "ORD_PLATFORM_TYPE"},
		"contract type":        {real(map[string]string{"ORD_INITIAL_CONTRACT_TYPE": "agency"}), "ORD_INITIAL_CONTRACT_TYPE"},
		"subject type":         {real(map[string]string{"ORD_PUBLISHER_SUBJECT_TYPE": "ads"}), "ORD_PUBLISHER_SUBJECT_TYPE"},
		"action type":          {real(map[string]string{"ORD_INITIAL_ACTION_TYPE": "sell"}), "ORD_INITIAL_ACTION_TYPE"},
		"rate":                 {real(map[string]string{"ORD_RATE": "0"}), "ORD_RATE"},
		"poll beyond timeout":  {real(map[string]string{"ORD_POLL_SLOW_INTERVAL": "25h"}), "ORD_POLL_*"},
		"zero dependency wait": {real(map[string]string{"ORD_DEPENDENCY_WAIT": "0s"}), "ORD_DEPENDENCY_WAIT"},
		"bad url":              {real(map[string]string{"ORD_API_URL": "ord.yandex.net"}), "ORD_API_URL"},
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

	// production: ОРД нельзя выключить, адрес и токен обязательны.
	prod := real(map[string]string{"APP_ENV": "production", "ORD_ENABLED": "false"})
	_, err := LoadConfig(prod, ModeWorker)
	if err == nil || !slices.ContainsFunc(configFields(t, err), func(f string) bool { return strings.HasPrefix(f, "ORD_ENABLED:") }) {
		t.Fatalf("ORD_ENABLED=false in production: %v", err)
	}
	prod = real(map[string]string{"APP_ENV": "production", "ORD_OAUTH_TOKEN": ""})
	_, err = LoadConfig(prod, ModeWorker)
	fields := configFields(t, err)
	for _, want := range []string{"ORD_API_URL", "ORD_OAUTH_TOKEN"} {
		if !slices.ContainsFunc(fields, func(f string) bool { return strings.HasPrefix(f, want+":") }) {
			t.Errorf("production without %s accepted: %v", want, fields)
		}
	}

	// Выключенный ОРД в разработке не проверяется.
	off := validWorkerEnv()
	off["ORD_ENABLED"] = "false"
	off["ORD_KKTU_CODE"] = "bad"
	if _, err := LoadConfig(off, ModeWorker); err != nil {
		t.Fatalf("disabled ORD validated: %v", err)
	}
}

func TestORDConfigLogValue(t *testing.T) {
	e := validWorkerEnv()
	e["ORD_OAUTH_TOKEN"] = "ord-oauth-secret"
	e["ORD_KKTU_CODE"] = "30.15.1"
	e["ORD_PLATFORM_INN"] = "7707083893"
	e["ORD_PLATFORM_LEGAL_TYPE"] = "legal_entity"
	e["ORD_PLATFORM_NAME"] = "ООО Платформа"
	cfg, err := LoadConfig(e, ModeWorker)
	if err != nil {
		t.Fatal(err)
	}
	s := cfg.LogValue().String()
	if strings.Contains(s, "ord-oauth-secret") || strings.Contains(s, "7707083893") || !strings.Contains(s, "ord_enabled") {
		t.Fatalf("ORD config log: %s", s)
	}
}
