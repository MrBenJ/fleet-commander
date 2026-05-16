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

func TestStatusRecorder_HijackUnsupportedReturnsError(t *testing.T) {
	rr := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: rr, status: http.StatusOK}

	_, _, err := rec.Hijack()
	if err == nil {
		t.Fatal("expected error when underlying ResponseWriter is not a Hijacker")
	}
}
