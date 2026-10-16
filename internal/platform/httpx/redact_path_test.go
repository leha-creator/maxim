package httpx

import "testing"

func TestRedactPath(t *testing.T) {
	for in, want := range map[string]string{
		"/api/v1/webhooks/max/s3cr3t":       "/api/v1/webhooks/max/***",
		"/api/v1/webhooks/max/s3cr3t/extra": "/api/v1/webhooks/max/***/extra",
		"/api/v1/webhooks/rocketwork":       "/api/v1/webhooks/rocketwork",
		"/api/v1/blogger/channels":          "/api/v1/blogger/channels",
	} {
		if got := RedactPath(in); got != want {
			t.Errorf("RedactPath(%q) = %q, want %q", in, got, want)
		}
	}
}
