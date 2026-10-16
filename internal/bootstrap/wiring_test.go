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
	for _, kind := range []string{"send_login_code", "process_inbound_event", "send_alert", "check_stuck_objects", "cleanup"} {
		if !slices.Contains(a.worker.Kinds(), kind) {
			t.Fatalf("worker kinds %v have no %s", a.worker.Kinds(), kind)
		}
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
