package terminal

import (
	"log"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestProxyShutdown_DrainsActiveSessions confirms that registered terminal
// sessions get their cleanup callbacks invoked when Shutdown is called, and
// that the active-session map is cleared so a second Shutdown is a no-op.
func TestProxyShutdown_DrainsActiveSessions(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	p := NewProxy("fleet", logger)

	var calls int32
	cleanup := func() { atomic.AddInt32(&calls, 1) }

	_, unreg1 := p.register(cleanup)
	_, unreg2 := p.register(cleanup)
	defer unreg1()
	defer unreg2()

	closed := p.Shutdown()
	if closed != 2 {
		t.Errorf("Shutdown returned %d, want 2", closed)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("cleanup invoked %d times, want 2", got)
	}

	// Second Shutdown is a no-op (and must not double-invoke cleanups).
	if n := p.Shutdown(); n != 0 {
		t.Errorf("second Shutdown returned %d, want 0", n)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Errorf("cleanup invoked %d times after second Shutdown, want still 2", got)
	}
}

// TestProxyShutdown_UnregisterRemovesFromActive verifies the unregister
// handle removes the entry before Shutdown — otherwise a finished session
// would still get its cleanup re-run on server exit.
func TestProxyShutdown_UnregisterRemovesFromActive(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	p := NewProxy("fleet", logger)

	var calls int32
	cleanup := func() { atomic.AddInt32(&calls, 1) }

	_, unreg := p.register(cleanup)
	unreg()

	if n := p.Shutdown(); n != 0 {
		t.Errorf("Shutdown after unregister returned %d, want 0", n)
	}
	if got := atomic.LoadInt32(&calls); got != 0 {
		t.Errorf("cleanup ran %d times after unregister; should be 0", got)
	}
}

// TestProxyShutdown_RegisterAfterCloseRunsCleanupImmediately makes the
// race-safety guarantee explicit: a session that starts a hair after
// Shutdown sees `closed == true` and gets its cleanup invoked anyway, so we
// don't leak the PTY.
func TestProxyShutdown_RegisterAfterCloseRunsCleanupImmediately(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	p := NewProxy("fleet", logger)

	p.Shutdown()

	done := make(chan struct{})
	cleanup := func() { close(done) }
	_, unreg := p.register(cleanup)
	defer unreg()

	select {
	case <-done:
		// expected
	case <-timeoutFastTest():
		t.Fatal("cleanup not invoked after register-on-closed-proxy")
	}
}

// TestProxyHandleTerminal_RefusesNewSessionsAfterShutdown ensures the HTTP
// handler returns 503 once Shutdown has been called.
func TestProxyHandleTerminal_RefusesNewSessionsAfterShutdown(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	p := NewProxy("fleet", logger)
	p.Shutdown()

	// Synthesize a fake upgrade request — the handler should bail with 503
	// before trying to upgrade.
	req := httptest.NewRequest(http.MethodGet, "/ws/terminal/agent-x", nil)
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	w := httptest.NewRecorder()

	p.HandleTerminal(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 after Shutdown, got %d", w.Code)
	}
}

// timeoutFastTest is a tiny helper that returns a channel firing after a
// short bounded delay so tests don't hang if cleanup never runs.
func timeoutFastTest() <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		// keep imports tight — use time.Sleep with a short literal.
		// 200ms is plenty for an in-process cleanup callback.
		sleepShort()
		close(ch)
	}()
	return ch
}
