package ws

import (
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestHubShutdown_ClosesActiveClients confirms Shutdown drops every open
// WebSocket connection so the server can exit without leaking goroutines.
func TestHubShutdown_ClosesActiveClients(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHub("/tmp/fake", "/tmp/fake", "fleet", logger)

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn1, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial 1: %v", err)
	}
	defer conn1.Close()

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial 2: %v", err)
	}
	defer conn2.Close()

	// Give the upgrade goroutines a moment to register both clients.
	time.Sleep(50 * time.Millisecond)
	if got := hub.ClientCount(); got != 2 {
		t.Fatalf("expected 2 clients before shutdown, got %d", got)
	}

	closed := hub.Shutdown()
	if closed != 2 {
		t.Errorf("Shutdown reported %d closures, want 2", closed)
	}
	if got := hub.ClientCount(); got != 0 {
		t.Errorf("ClientCount after Shutdown = %d, want 0", got)
	}

	// Each client's read should now return an error promptly.
	conn1.SetReadDeadline(time.Now().Add(time.Second))
	if _, _, err := conn1.ReadMessage(); err == nil {
		t.Error("conn1: expected read error after Shutdown, got nil")
	}
}

// TestHubShutdown_IsIdempotent verifies a second call returns 0 and does not
// blow up. Real server-shutdown paths may call this defensively.
func TestHubShutdown_IsIdempotent(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHub("/tmp/fake", "/tmp/fake", "fleet", logger)

	if n := hub.Shutdown(); n != 0 {
		t.Errorf("first Shutdown on empty hub reported %d, want 0", n)
	}
	if n := hub.Shutdown(); n != 0 {
		t.Errorf("second Shutdown reported %d, want 0", n)
	}
}

// TestHubShutdown_RejectsLateConnections proves that a client trying to
// connect after Shutdown is refused with 503, not silently accepted.
func TestHubShutdown_RejectsLateConnections(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHub("/tmp/fake", "/tmp/fake", "fleet", logger)

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	hub.Shutdown()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected dial to fail after Shutdown")
	}
	if resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		gotCode := 0
		if resp != nil {
			gotCode = resp.StatusCode
		}
		t.Errorf("expected 503 after Shutdown, got %d", gotCode)
	}
}
