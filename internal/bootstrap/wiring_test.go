package bootstrap

import (
	"net/http"
	"slices"
	"testing"
)

func TestAssembledApp(t *testing.T) {
	a := newTestApp(t)
	resp, err := http.Get(a.srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("readyz: %d", resp.StatusCode)
	}
	if !slices.Contains(a.worker.Kinds(), "send_login_code") {
		t.Fatalf("worker kinds: %v", a.worker.Kinds())
	}
	resp, err = http.Get(a.srv.URL + "/openapi.json")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("openapi.json outside production: %d", resp.StatusCode)
	}
}
