package security

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newRequest(host, origin string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/test", nil)
	r.Host = host
	if origin != "" {
		r.Header.Set("Origin", origin)
	}
	return r
}

func TestAllowWebSocket_SameOrigin(t *testing.T) {
	v := New(false)

	cases := []struct {
		name   string
		host   string
		origin string
		want   bool
	}{
		{"exact match 127.0.0.1", "127.0.0.1:4242", "http://127.0.0.1:4242", true},
		{"exact match localhost", "localhost:4242", "http://localhost:4242", true},
		{"loopback alias", "127.0.0.1:4242", "http://localhost:4242", true},
		{"loopback alias reverse", "localhost:4242", "http://127.0.0.1:4242", true},
		{"empty origin allowed (non-browser)", "127.0.0.1:4242", "", true},
		{"cross-origin attacker", "127.0.0.1:4242", "http://evil.com", false},
		{"cross-origin same port", "127.0.0.1:4242", "http://192.168.1.5:4242", false},
		{"loopback different port", "127.0.0.1:4242", "http://localhost:5173", false},
		{"malformed origin", "127.0.0.1:4242", "::::not a url", false},
		{"http schema only no host", "127.0.0.1:4242", "http://", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := v.AllowWebSocket(newRequest(tc.host, tc.origin))
			if got != tc.want {
				t.Errorf("AllowWebSocket(host=%q, origin=%q) = %v, want %v",
					tc.host, tc.origin, got, tc.want)
			}
		})
	}
}

func TestAllowWebSocket_DevModeAllowsVite(t *testing.T) {
	v := New(true)

	// Vite dev server on loopback should be allowed.
	if !v.AllowWebSocket(newRequest("127.0.0.1:4242", "http://localhost:5173")) {
		t.Error("dev mode should allow localhost:5173 Vite origin")
	}
	if !v.AllowWebSocket(newRequest("127.0.0.1:4242", "http://127.0.0.1:5173")) {
		t.Error("dev mode should allow 127.0.0.1:5173 Vite origin")
	}

	// Non-Vite cross-origin still rejected.
	if v.AllowWebSocket(newRequest("127.0.0.1:4242", "http://evil.com:5173")) {
		t.Error("dev mode should not allow arbitrary :5173 hosts")
	}
	if v.AllowWebSocket(newRequest("127.0.0.1:4242", "http://localhost:6000")) {
		t.Error("dev mode should not allow non-5173 loopback ports")
	}
}

func TestAllowCrossSiteRequest_SameOrigin(t *testing.T) {
	v := New(false)

	if !v.AllowCrossSiteRequest(newRequest("127.0.0.1:4242", "http://127.0.0.1:4242")) {
		t.Error("same-origin should be allowed")
	}
	if v.AllowCrossSiteRequest(newRequest("127.0.0.1:4242", "http://evil.com")) {
		t.Error("cross-origin should be rejected")
	}
	// Empty Origin (curl, scripts, server-side) is allowed because the threat
	// is browser-based and browsers send Origin on cross-origin POSTs.
	if !v.AllowCrossSiteRequest(newRequest("127.0.0.1:4242", "")) {
		t.Error("absent Origin should be allowed (non-browser clients)")
	}
}

func TestAllow_NilRequest(t *testing.T) {
	v := New(false)
	if v.AllowWebSocket(nil) {
		t.Error("nil request should not be allowed for WebSocket")
	}
	if v.AllowCrossSiteRequest(nil) {
		t.Error("nil request should not be allowed for cross-site")
	}
}

func TestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"":           false,
		"localhost":  true,
		"LOCALHOST":  true,
		"127.0.0.1":  true,
		"127.0.0.5":  true,
		"::1":        true,
		"[::1]":      true,
		"10.0.0.1":   false,
		"example.com": false,
	}
	for in, want := range cases {
		if got := isLoopback(in); got != want {
			t.Errorf("isLoopback(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestHostsEqual(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"127.0.0.1:4242", "127.0.0.1:4242", true},
		{"localhost:4242", "127.0.0.1:4242", true},
		{"127.0.0.1:4242", "localhost:4242", true},
		{"localhost:5173", "127.0.0.1:4242", false},
		{"evil.com:4242", "127.0.0.1:4242", false},
	}
	for _, tc := range cases {
		if got := hostsEqual(tc.a, tc.b); got != tc.want {
			t.Errorf("hostsEqual(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
