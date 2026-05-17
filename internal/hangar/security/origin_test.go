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
	v := New(false, "127.0.0.1")

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
	v := New(true, "127.0.0.1")

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
	v := New(false, "127.0.0.1")

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
	v := New(false, "127.0.0.1")
	if v.AllowWebSocket(nil) {
		t.Error("nil request should not be allowed for WebSocket")
	}
	if v.AllowCrossSiteRequest(nil) {
		t.Error("nil request should not be allowed for cross-site")
	}
}

// TestAllow_DNSRebind asserts that a loopback-bound validator rejects
// requests whose Host header names a non-loopback host, even when Origin
// matches the Host. This is the rebinding scenario:
//
//   - victim visits http://attacker.example:4242
//   - attacker's DNS later resolves attacker.example to 127.0.0.1
//   - the browser sends Host: attacker.example:4242 and matching Origin
//
// Without a Host allowlist, hostsEqual returns true and the gate opens.
func TestAllow_DNSRebind(t *testing.T) {
	loopbackBound := New(false, "127.0.0.1")

	rebindCases := []struct {
		name   string
		host   string
		origin string
	}{
		{"attacker domain", "attacker.example:4242", "http://attacker.example:4242"},
		{"attacker IP", "10.0.0.5:4242", "http://10.0.0.5:4242"},
		{"attacker no origin", "attacker.example:4242", ""},
	}
	for _, tc := range rebindCases {
		t.Run("rejected/"+tc.name, func(t *testing.T) {
			if loopbackBound.AllowWebSocket(newRequest(tc.host, tc.origin)) {
				t.Errorf("loopback-bound validator must reject Host=%q (DNS rebind risk)", tc.host)
			}
			if loopbackBound.AllowCrossSiteRequest(newRequest(tc.host, tc.origin)) {
				t.Errorf("loopback-bound validator must reject CSRF with Host=%q (DNS rebind risk)", tc.host)
			}
		})
	}
}

// TestAllow_BindHostBehavior covers the three bindHost modes.
func TestAllow_BindHostBehavior(t *testing.T) {
	t.Run("wildcard bind permits any Host", func(t *testing.T) {
		v := New(false, "0.0.0.0")
		// When operator opted in to LAN exposure, we can't know all valid
		// hostnames the network resolves us to, so we don't restrict.
		if !v.AllowWebSocket(newRequest("server.lan:4242", "http://server.lan:4242")) {
			t.Error("wildcard bind should accept LAN Host header")
		}
		if !v.AllowWebSocket(newRequest("attacker.example:4242", "http://attacker.example:4242")) {
			t.Error("wildcard bind is permissive on Host by design")
		}
	})

	t.Run("empty bindHost is permissive (test/dev fallback)", func(t *testing.T) {
		v := New(false, "")
		if !v.AllowWebSocket(newRequest("anything.example:4242", "http://anything.example:4242")) {
			t.Error("empty bindHost should not restrict Host")
		}
	})

	t.Run("specific non-loopback host only allows that host", func(t *testing.T) {
		v := New(false, "192.168.1.5")
		if !v.AllowWebSocket(newRequest("192.168.1.5:4242", "http://192.168.1.5:4242")) {
			t.Error("Host matching specific bind should be accepted")
		}
		if v.AllowWebSocket(newRequest("attacker.example:4242", "http://attacker.example:4242")) {
			t.Error("specific bind should reject non-matching Host")
		}
		if v.AllowWebSocket(newRequest("127.0.0.1:4242", "http://127.0.0.1:4242")) {
			t.Error("specific bind should not implicitly accept loopback Host")
		}
	})

	t.Run("loopback bind accepts all loopback aliases", func(t *testing.T) {
		v := New(false, "127.0.0.1")
		for _, host := range []string{"127.0.0.1:4242", "localhost:4242", "[::1]:4242"} {
			if !v.AllowWebSocket(newRequest(host, "")) {
				t.Errorf("loopback bind should accept Host=%q", host)
			}
		}
	})
}

func TestAllowHost(t *testing.T) {
	v := New(false, "127.0.0.1")

	if !v.AllowHost(newRequest("127.0.0.1:4242", "")) {
		t.Error("loopback Host should be allowed")
	}
	if !v.AllowHost(newRequest("localhost:4242", "")) {
		t.Error("localhost alias should be allowed")
	}
	if v.AllowHost(newRequest("attacker.example:4242", "")) {
		t.Error("non-loopback Host must be rejected for loopback-bound server")
	}
	if v.AllowHost(nil) {
		t.Error("nil request must not be allowed")
	}

	// Permissive validator (wildcard or unset bind host) accepts anything.
	permissive := New(false, "")
	if !permissive.AllowHost(newRequest("anything.example:4242", "")) {
		t.Error("permissive validator should accept any Host")
	}
}

func TestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"":            false,
		"localhost":   true,
		"LOCALHOST":   true,
		"127.0.0.1":   true,
		"127.0.0.5":   true,
		"::1":         true,
		"[::1]":       true,
		"10.0.0.1":    false,
		"example.com": false,
	}
	for in, want := range cases {
		if got := isLoopback(in); got != want {
			t.Errorf("isLoopback(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsUnspecified(t *testing.T) {
	cases := map[string]bool{
		"":          false,
		"0.0.0.0":   true,
		"::":        true,
		"[::]":      true,
		"127.0.0.1": false,
		"localhost": false,
		"10.0.0.1":  false,
	}
	for in, want := range cases {
		if got := isUnspecified(in); got != want {
			t.Errorf("isUnspecified(%q) = %v, want %v", in, got, want)
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
