package hangar

import (
	"context"
	"testing"
	"time"
)

// TestServer_StartUnblocksWhenServeReturnsWithoutCtxCancel reproduces the
// deadlock raised in code review: if http.Server.Serve returns before ctx
// is canceled (e.g., a fatal listener error or a direct s.server.Close()),
// the shutdown goroutine must still proceed so Start() returns. The old
// implementation waited on <-ctx.Done() inside the goroutine and
// unconditionally on <-shutdownDone after Serve, producing a hang.
func TestServer_StartUnblocksWhenServeReturnsWithoutCtxCancel(t *testing.T) {
	srv := NewServer(Config{Port: 0, Listen: "127.0.0.1"})

	// ctx that is NOT canceled by the test. The fix should still let Start
	// return because we force the server closed externally.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- srv.Start(ctx) }()

	// Wait for Start to register its listener.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if srv.Addr() != "" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if srv.Addr() == "" {
		t.Fatal("server did not become ready")
	}

	// Drain logs so chanWriter doesn't block on a full LogCh.
	go func() {
		for range srv.LogCh {
		}
	}()

	// Force Serve to return without canceling ctx. Server.Close reads
	// s.server under addrMu so it is race-safe even while Start is still
	// initializing fields.
	if err := srv.Close(); err != nil {
		t.Fatalf("server.Close: %v", err)
	}

	select {
	case <-done:
		// Pass — Start returned, no deadlock.
	case <-time.After(3 * time.Second):
		t.Fatal("Start did not return after server.Close — likely deadlocked on shutdownDone")
	}
}
