package ws

import (
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/MrBenJ/fleet-commander/internal/hangar/security"
)

// TestHubWebSocket_RejectsCrossOrigin verifies the WebSocket upgrade is
// rejected when the Origin header does not match the server Host. This
// would have succeeded under the previous CheckOrigin = true behavior.
func TestHubWebSocket_RejectsCrossOrigin(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHubWithValidator("/tmp/fake", "/tmp/fake", "fleet", logger, security.New(false, "127.0.0.1"))

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	headers := http.Header{}
	headers.Set("Origin", "http://evil.example.com")
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err == nil {
		conn.Close()
		t.Fatalf("expected handshake error for cross-origin Origin, dial succeeded")
	}
	if resp == nil {
		t.Fatalf("expected response on failed handshake, got nil")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 on cross-origin upgrade, got %d", resp.StatusCode)
	}

	time.Sleep(20 * time.Millisecond)
	if hub.ClientCount() != 0 {
		t.Errorf("expected 0 clients after rejected handshake, got %d", hub.ClientCount())
	}
}

// TestHubWebSocket_AcceptsSameOrigin verifies the WebSocket upgrade succeeds
// when the Origin matches the server Host.
func TestHubWebSocket_AcceptsSameOrigin(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHubWithValidator("/tmp/fake", "/tmp/fake", "fleet", logger, security.New(false, "127.0.0.1"))

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	// httptest.Server.URL is "http://<host:port>" — reuse it as Origin so
	// the validator sees Origin == Host.
	headers := http.Header{}
	headers.Set("Origin", server.URL)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("same-origin handshake failed: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)
	if hub.ClientCount() != 1 {
		t.Errorf("expected 1 client after same-origin handshake, got %d", hub.ClientCount())
	}
}

// TestHubWebSocket_DevModeAllowsVite verifies the upgrade succeeds for the
// Vite dev server origin when devMode is on. Without dev mode the same
// request must be rejected.
func TestHubWebSocket_DevModeAllowsVite(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHubWithValidator("/tmp/fake", "/tmp/fake", "fleet", logger, security.New(true, "127.0.0.1"))

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	headers := http.Header{}
	headers.Set("Origin", "http://localhost:5173")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatalf("dev-mode Vite handshake failed: %v", err)
	}
	conn.Close()
}

func TestHubWebSocket_DevModeOffRejectsVite(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHubWithValidator("/tmp/fake", "/tmp/fake", "fleet", logger, security.New(false, "127.0.0.1"))

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	headers := http.Header{}
	headers.Set("Origin", "http://localhost:5173")
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err == nil {
		t.Fatal("expected dev-mode-off to reject Vite origin")
	}
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for Vite origin without dev mode, got %v", resp)
	}
}
