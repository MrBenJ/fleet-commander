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

func TestHubBroadcast(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHub("/tmp/fake", "/tmp/fake", "fleet", logger)

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Fatalf("expected 1 client, got %d", hub.ClientCount())
	}

	hub.Broadcast(Event{
		Type:    "context_message",
		Agent:   "test-agent",
		Message: "hello from test",
	})

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}

	if !strings.Contains(string(msg), "hello from test") {
		t.Errorf("expected 'hello from test', got: %s", msg)
	}
}

func TestHubMultipleClients(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHub("/tmp/fake", "/tmp/fake", "fleet", logger)

	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")

	conn1, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	defer conn1.Close()
	conn2, _, _ := websocket.DefaultDialer.Dial(wsURL, nil)
	defer conn2.Close()

	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 2 {
		t.Fatalf("expected 2 clients, got %d", hub.ClientCount())
	}

	hub.Broadcast(Event{Type: "agent_stopped", Agent: "x"})

	for i, conn := range []*websocket.Conn{conn1, conn2} {
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("client %d read failed: %v", i, err)
		}
		if !strings.Contains(string(msg), "agent_stopped") {
			t.Errorf("client %d: expected agent_stopped, got: %s", i, msg)
		}
	}
}
