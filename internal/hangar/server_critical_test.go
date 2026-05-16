package hangar

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestServer_BindsToLocalhostByDefault verifies the hangar HTTP server binds
// to 127.0.0.1 by default rather than 0.0.0.0. Before C2 was fixed, the
// server listened on all interfaces and was reachable from the LAN.
func TestServer_BindsToLocalhostByDefault(t *testing.T) {
	srv := NewServer(Config{Port: 0})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startErr := make(chan error, 1)
	go func() { startErr <- srv.Start(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && srv.Addr() == "" {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.Addr() == "" {
		t.Fatal("server did not start in time")
	}

	host, _, err := net.SplitHostPort(srv.Addr())
	if err != nil {
		t.Fatalf("addr %q is not host:port: %v", srv.Addr(), err)
	}
	if host != "127.0.0.1" {
		t.Errorf("default bind host should be 127.0.0.1, got %q (full addr %q)", host, srv.Addr())
	}

	if srv.ListenHost() != "127.0.0.1" {
		t.Errorf("ListenHost() = %q, want 127.0.0.1", srv.ListenHost())
	}

	cancel()
	select {
	case <-startErr:
	case <-time.After(2 * time.Second):
		t.Error("server did not stop in time")
	}
}

// TestServer_RespectsListenConfig verifies that an explicit Listen value is
// honored and used as the bind host.
func TestServer_RespectsListenConfig(t *testing.T) {
	srv := NewServer(Config{Port: 0, Listen: "127.0.0.1"})
	if srv.ListenHost() != "127.0.0.1" {
		t.Errorf("ListenHost() = %q, want 127.0.0.1", srv.ListenHost())
	}
}

// TestServer_CSRFRejectsCrossOriginPost verifies the CSRF middleware
// rejects POST requests with a foreign Origin header. Before C3 was
// fixed, drive-by sites could launch or stop squadrons via XHR.
func TestServer_CSRFRejectsCrossOriginPost(t *testing.T) {
	srv, cancel := startTestServer(t)
	defer cancel()

	base := fmt.Sprintf("http://%s", srv.Addr())

	body := strings.NewReader(`{"name":"x"}`)
	req, err := http.NewRequest(http.MethodPost, base+"/api/squadron/launch", body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", "http://evil.example.com")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-origin POST should be 403, got %d", resp.StatusCode)
	}
}

// TestServer_CSRFAllowsSameOriginPost verifies same-origin POSTs pass the
// CSRF gate. They may fail later with a different status (e.g., 400 for
// validation), but they must not be blocked at 403.
func TestServer_CSRFAllowsSameOriginPost(t *testing.T) {
	srv, cancel := startTestServer(t)
	defer cancel()

	base := fmt.Sprintf("http://%s", srv.Addr())

	body := strings.NewReader(`{"name":"x"}`)
	req, err := http.NewRequest(http.MethodPost, base+"/api/squadron/launch", body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", base)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		t.Error("same-origin POST should not be 403")
	}
}

// TestServer_CSRFAllowsNoOriginPost verifies POSTs without an Origin header
// (CLI tools, scripts) pass the CSRF gate. The threat model is browser-
// driven; non-browser clients are out of scope.
func TestServer_CSRFAllowsNoOriginPost(t *testing.T) {
	srv, cancel := startTestServer(t)
	defer cancel()

	base := fmt.Sprintf("http://%s", srv.Addr())

	body := strings.NewReader(`{"name":"x"}`)
	req, err := http.NewRequest(http.MethodPost, base+"/api/squadron/launch", body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	// no Origin set
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		t.Error("no-Origin POST should not be 403")
	}
}

// TestServer_CSRFDoesNotBlockGET ensures the middleware only fires on
// state-changing methods. GET with a foreign Origin remains allowed.
func TestServer_CSRFDoesNotBlockGET(t *testing.T) {
	srv, cancel := startTestServer(t)
	defer cancel()

	base := fmt.Sprintf("http://%s", srv.Addr())

	req, err := http.NewRequest(http.MethodGet, base+"/api/health", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Origin", "http://evil.example.com")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("cross-origin GET /api/health should still be 200, got %d", resp.StatusCode)
	}
}

// TestSPAHandler_DottedDirectoryRewrites verifies the SPA handler treats
// "/foo.bar/baz" as a client route (not an asset), so the route renders
// index.html. Before C4 was fixed, strings.Contains(".", path) misrouted
// any path containing a dotted directory segment.
func TestSPAHandler_DottedDirectoryRewrites(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{"dotted directory client route", "/foo.bar/baz", "/"},
		{"dotted directory with more depth", "/v1.2/squadrons/abc", "/"},
		{"asset still preserved", "/assets/main.js", "/assets/main.js"},
		{"asset with dotted dir preserved", "/v1.2/assets/main.js", "/v1.2/assets/main.js"},
		{"root unchanged", "/", "/"},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var seenPath string
			downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seenPath = r.URL.Path
			})
			h := &spaHandler{fs: downstream}

			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if seenPath != tt.want {
				t.Errorf("path %q: rewrote to %q, want %q", tt.path, seenPath, tt.want)
			}
		})
	}
}

// TestIsAssetPath documents the rule used by spaHandler. An asset's last
// path segment contains a dot.
func TestIsAssetPath(t *testing.T) {
	cases := map[string]bool{
		"":                false,
		"/":               false,
		"/foo":            false,
		"/foo/bar":        false,
		"/foo.bar/baz":    false,
		"/foo.bar":        true,
		"/assets/main.js": true,
		"/logo.png":       true,
		"/a/b/c/d.svg":    true,
	}
	for p, want := range cases {
		if got := isAssetPath(p); got != want {
			t.Errorf("isAssetPath(%q) = %v, want %v", p, got, want)
		}
	}
}

// TestServer_GracefulShutdownClosesLogCh verifies that the server closes
// the log channel after Shutdown returns, so consumers (the TUI log pump)
// see a closed channel and exit their range loop without leaking
// goroutines. Before C6 was fixed, the log channel was never closed.
func TestServer_GracefulShutdownClosesLogCh(t *testing.T) {
	srv := NewServer(Config{Port: 0})
	ctx, cancel := context.WithCancel(context.Background())

	startErr := make(chan error, 1)
	go func() { startErr <- srv.Start(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && srv.Addr() == "" {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.Addr() == "" {
		cancel()
		t.Fatal("server did not start")
	}

	// Drain any startup logs.
	go func() {
		for range srv.LogCh {
		}
	}()

	cancel()
	select {
	case err := <-startErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			t.Errorf("Start returned unexpected error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("server did not stop in time")
	}

	// LogCh should be closed by now.
	select {
	case _, ok := <-srv.LogCh:
		if ok {
			t.Error("LogCh should be closed after Start returns")
		}
	default:
		// Channel must be readable (closed channels return immediately) —
		// if it blocks here, it's not closed. Fail the test.
		t.Error("LogCh blocked on read; expected closed channel")
	}
}

// TestServer_StartFailsIfPortTaken sanity-checks the listen error path.
func TestServer_StartFailsIfPortTaken(t *testing.T) {
	// Bind a listener to an ephemeral port and try to start the server on it.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	defer l.Close()

	_, portStr, _ := net.SplitHostPort(l.Addr().String())
	var port int
	fmt.Sscanf(portStr, "%d", &port)

	srv := NewServer(Config{Port: port, Listen: "127.0.0.1"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	startErr := make(chan error, 1)
	go func() { startErr <- srv.Start(ctx) }()

	select {
	case err := <-startErr:
		if err == nil {
			t.Fatal("expected error when port is taken")
		}
		if !strings.Contains(err.Error(), "listen on") {
			t.Errorf("error should mention listen failure, got %q", err.Error())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return on port-taken")
	}
}
