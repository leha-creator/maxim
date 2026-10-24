package rocketwork

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestParseWebhook(t *testing.T) {
	secret := []byte("whsec")
	body := WebhookPayload("ev-1", ServiceEventPayment, "p-1")
	good := http.Header{SignatureHeader: {Sign(secret, body)}}
	ev, err := ParseWebhook(secret, good, body)
	if err != nil || ev.Kind != EventPaymentStatus || ev.ObjectID != "p-1" || ev.EventID != "ev-1" {
		t.Fatalf("event %+v %v", ev, err)
	}
	lower := map[string][]string{"x-rocketwork-signature": {Sign(secret, body)}}
	if _, err := ParseWebhook(secret, lower, body); err != nil {
		t.Fatalf("header name case: %v", err)
	}
	cases := map[string]struct {
		secret []byte
		header http.Header
		body   []byte
		want   error
	}{
		"other secret":  {[]byte("other"), good, body, ErrSignature},
		"changed body":  {secret, good, append([]byte(nil), append(body, ' ')...), ErrSignature},
		"no header":     {secret, http.Header{}, body, ErrSignature},
		"empty secret":  {nil, good, body, ErrSignature},
		"unknown event": {secret, nil, WebhookPayload("e", "payment.created", "p"), ErrFormat},
		"too large":     {secret, nil, []byte(strings.Repeat("a", MaxWebhookBody+1)), ErrFormat},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			h := c.header
			if h == nil {
				h = http.Header{SignatureHeader: {Sign(c.secret, c.body)}}
			}
			if _, err := ParseWebhook(c.secret, h, c.body); !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestStatusMapping(t *testing.T) {
	for raw, want := range map[string]string{"invited": ContractorInvited, "registering": ContractorPending,
		"documents_check": ContractorPending, "active": ContractorActive, "blocked": ContractorBlocked, "deleted": ContractorBlocked} {
		if got, _, known := ContractorStatusFrom(raw); got != want || !known {
			t.Errorf("contractor %s → %s", raw, got)
		}
	}
	if got, reason, known := ContractorStatusFrom("fns_unbound"); got != ContractorBlocked || reason != FailureTaxStatusLost || !known {
		t.Errorf("fns_unbound → %s %s", got, reason)
	}
	if got, _, known := ContractorStatusFrom("new_status"); got != ContractorPending || known {
		t.Errorf("unknown contractor status → %s", got)
	}
	if got, known := PaymentStatusFrom("whatever"); got != PaymentSent || known {
		t.Errorf("unknown payment status → %s", got)
	}
	for raw, want := range map[string]string{"insufficient_funds": FailureInsufficientDeposit, "fns_unbound": FailureTaxStatusLost,
		"income_limit_exceeded": FailureIncomeLimitExceeded, "contractor_blocked": FailureContractorNotActive, "boom": FailureProviderError} {
		if got := FailureCodeFrom(raw); got != want {
			t.Errorf("failure %s → %s", raw, got)
		}
	}
}

func TestRedactJSON(t *testing.T) {
	out := string(redactJSON([]byte(`{"inn":"500100732259","phone":"+79991234567","full_name":"Иванов Иван Петрович",
		"onboarding_url":"https://o/1","amount_kopecks":100}`)))
	for _, s := range []string{"500100732259", "+79991234567", "Иван Петрович", "https://o/1"} {
		if strings.Contains(out, s) {
			t.Errorf("%q not masked: %s", s, out)
		}
	}
	if !strings.Contains(out, `"amount_kopecks":100`) {
		t.Errorf("amount lost: %s", out)
	}
	if string(redactJSON([]byte("not json"))) != "not json" {
		t.Error("non-JSON changed")
	}
}
