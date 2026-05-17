package hangar

import (
	"bufio"
	"bytes"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/hangar/security"
)

func newCaptureLogger() (*log.Logger, *bytes.Buffer) {
	buf := &bytes.Buffer{}
	return log.New(buf, "", 0), buf
}

func TestAccessLogger_DefaultStatusIsOK(t *testing.T) {
	logger, buf := newCaptureLogger()
	handler := accessLogger(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Write nothing: net/http treats this as 200 OK.
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	handler.ServeHTTP(rr, req)

	got := buf.String()
	if !strings.Contains(got, "GET /api/health -> 200") {
		t.Errorf("log line missing default-OK status; got %q", got)
	}
}

func TestAccessLogger_RecordsExplicitStatus(t *testing.T) {
	logger, buf := newCaptureLogger()
	handler := accessLogger(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusTeapot)
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/launch", nil)
	handler.ServeHTTP(rr, req)

	got := buf.String()
	if !strings.Contains(got, "POST /api/squadron/launch -> 418") {
		t.Errorf("expected POST + 418 in log, got %q", got)
	}
	if !strings.Contains(got, "bytes") {
		t.Errorf("log line should include byte count, got %q", got)
	}
}

func TestAccessLogger_RecordsByteCount(t *testing.T) {
	logger, buf := newCaptureLogger()
	body := strings.Repeat("x", 37)
	handler := accessLogger(logger, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(body))
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(rr, req)

	got := buf.String()
	if !strings.Contains(got, "37 bytes") {
		t.Errorf("expected `37 bytes` in log, got %q", got)
	}
}

// hijackableRecorder is httptest.ResponseRecorder + an http.Hijacker
// implementation, since the stdlib recorder does not satisfy the hijacker
// interface by default.
type hijackableRecorder struct {
	*httptest.ResponseRecorder
	called bool
}

func (h *hijackableRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h.called = true
	c1, c2 := net.Pipe()
	_ = c2 // returned to handler; closing happens after the test
	return c1, bufio.NewReadWriter(bufio.NewReader(c1), bufio.NewWriter(c1)), nil
}

func TestStatusRecorder_ExposesHijacker(t *testing.T) {
	hr := &hijackableRecorder{ResponseRecorder: httptest.NewRecorder()}
	rec := &statusRecorder{ResponseWriter: hr, status: http.StatusOK}

	hj, ok := http.ResponseWriter(rec).(http.Hijacker)
	if !ok {
		t.Fatal("statusRecorder must implement http.Hijacker so WebSocket upgrades succeed")
	}

	_, _, err := hj.Hijack()
	if err != nil {
		t.Fatalf("Hijack returned error: %v", err)
	}
	if !hr.called {
		t.Error("Hijack did not delegate to the underlying ResponseWriter")
	}
	if rec.status != http.StatusSwitchingProtocols {
		t.Errorf("after successful Hijack, status should be 101; got %d", rec.status)
	}
}

// TestHostMiddleware_RejectsRebindOnGET asserts the DNS-rebind defense
// covers read-only methods, not just CSRF-scoped state changes. A loopback-
// bound server must refuse GET /api/fleet (and friends) when the Host header
// names a non-loopback host, since DNS rebinding can give a foreign page
// same-origin read access to repo path and stored prompts.
func TestHostMiddleware_RejectsRebindOnGET(t *testing.T) {
	s := &Server{
		validator: security.New(false, "127.0.0.1"),
	}
	called := false
	handler := s.hostMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	cases := []struct {
		name       string
		method     string
		host       string
		wantStatus int
		wantCalled bool
	}{
		{"GET rebound attacker", http.MethodGet, "attacker.example:4242", http.StatusForbidden, false},
		{"HEAD rebound attacker", http.MethodHead, "attacker.example:4242", http.StatusForbidden, false},
		{"OPTIONS rebound attacker", http.MethodOptions, "attacker.example:4242", http.StatusForbidden, false},
		{"POST rebound attacker", http.MethodPost, "attacker.example:4242", http.StatusForbidden, false},
		{"GET localhost passes", http.MethodGet, "127.0.0.1:4242", http.StatusOK, true},
		{"GET localhost alias passes", http.MethodGet, "localhost:4242", http.StatusOK, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called = false
			rr := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, "/api/fleet", nil)
			req.Host = tc.host
			handler.ServeHTTP(rr, req)

			if rr.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d", rr.Code, tc.wantStatus)
			}
			if called != tc.wantCalled {
				t.Errorf("downstream called = %v, want %v", called, tc.wantCalled)
			}
		})
	}
}

// TestHostMiddleware_WildcardBindAllowsAny asserts an operator who explicitly
// binds 0.0.0.0 (LAN exposure) opts out of Host filtering — the middleware
// must not block their own custom hostnames.
func TestHostMiddleware_WildcardBindAllowsAny(t *testing.T) {
	s := &Server{
		validator: security.New(false, "0.0.0.0"),
	}
	called := false
	handler := s.hostMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/fleet", nil)
	req.Host = "my-server.lan:4242"
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("wildcard bind must accept LAN Host, got status %d", rr.Code)
	}
	if !called {
		t.Error("downstream handler should have been invoked")
	}
}

func TestStatusRecorder_HijackUnsupportedReturnsError(t *testing.T) {
	rr := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: rr, status: http.StatusOK}

	_, _, err := rec.Hijack()
	if err == nil {
		t.Fatal("expected error when underlying ResponseWriter is not a Hijacker")
	}
}
