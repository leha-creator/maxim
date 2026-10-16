package httpx

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestClientIP(t *testing.T) {
	trusted, err := ParsePrefixes([]string{"10.0.0.0/8", " "})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, remote, xff, want string
	}{
		{"direct", "203.0.113.1:5000", "", "203.0.113.1"},
		{"untrusted proxy header ignored", "203.0.113.1:5000", "198.51.100.7", "203.0.113.1"},
		{"trusted proxy", "10.0.0.2:5000", "198.51.100.7", "198.51.100.7"},
		{"spoofed left hop ignored", "10.0.0.2:5000", "1.2.3.4, 198.51.100.7, 10.0.0.3", "198.51.100.7"},
		{"garbage header", "10.0.0.2:5000", "nope", "10.0.0.2"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got *netip.Addr
			h := ClientInfo(trusted)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = ClientIP(r.Context())
			}))
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = c.remote
			if c.xff != "" {
				req.Header.Set("X-Forwarded-For", c.xff)
			}
			h.ServeHTTP(httptest.NewRecorder(), req)
			if got == nil || got.String() != c.want {
				t.Fatalf("ip = %v, want %s", got, c.want)
			}
		})
	}
}
