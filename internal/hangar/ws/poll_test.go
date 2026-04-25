package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
)

func setupRepo(t *testing.T) (repoPath, fleetDir string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	for _, args := range [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command(args[0], args[1:]...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}

	fleetDir = filepath.Join(dir, ".fleet")
	if err := os.MkdirAll(fleetDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := map[string]interface{}{
		"repo_path": dir,
		"fleet_dir": fleetDir,
		"agents":    []map[string]interface{}{},
	}
	cfgData, _ := json.MarshalIndent(cfg, "", "  ")
	os.WriteFile(filepath.Join(fleetDir, "config.json"), cfgData, 0644)

	return dir, fleetDir
}

// dialAndStreamMessages connects to a hub and starts a goroutine that
// pushes every received message onto the returned channel. Used because
// the gorilla websocket connection becomes unusable after a read deadline
// fires, so tests can't mix "expect a message in 2s" with "expect no message
// in 200ms" using SetReadDeadline.
func dialAndStreamMessages(t *testing.T, hub *Hub) (msgs <-chan []byte, cleanup func()) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(hub.HandleWebSocket))
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		server.Close()
		t.Fatalf("dial: %v", err)
	}

	ch := make(chan []byte, 32)
	go func() {
		defer close(ch)
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			ch <- msg
		}
	}()

	// Give the hub a moment to register the connection.
	time.Sleep(50 * time.Millisecond)

	return ch, func() {
		conn.Close()
		server.Close()
	}
}

func waitForMessage(t *testing.T, msgs <-chan []byte, contains string, timeout time.Duration) []byte {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				t.Fatalf("channel closed before receiving %q", contains)
			}
			if strings.Contains(string(msg), contains) {
				return msg
			}
			// Skip irrelevant messages and keep waiting.
		case <-deadline:
			t.Fatalf("timed out waiting for message containing %q", contains)
			return nil
		}
	}
}

func expectNoMessage(t *testing.T, msgs <-chan []byte, window time.Duration) {
	t.Helper()
	select {
	case msg, ok := <-msgs:
		if !ok {
			return // channel closed counts as "no message"
		}
		t.Errorf("expected no message, got: %s", msg)
	case <-time.After(window):
	}
}

func TestPollChannels_BroadcastsNewMessages(t *testing.T) {
	repoPath, fleetDir := setupRepo(t)

	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHub(fleetDir, repoPath, "fleet", logger)

	if _, err := fleetctx.CreateChannel(fleetDir, "ignored", "test channel", []string{"alice", "bob"}); err != nil {
		t.Fatalf("CreateChannel: %v", err)
	}
	chName := "dm-[alice]-[bob]"
	if err := fleetctx.SendToChannel(fleetDir, chName, "alice", "first msg"); err != nil {
		t.Fatalf("SendToChannel: %v", err)
	}

	msgs, cleanup := dialAndStreamMessages(t, hub)
	defer cleanup()

	// First poll: existing log entries get broadcast.
	hub.pollChannels()

	got := waitForMessage(t, msgs, "first msg", 2*time.Second)
	if !strings.Contains(string(got), `"type":"context_message"`) {
		t.Errorf("expected event type 'context_message', got: %s", got)
	}

	// Second poll with no new messages: no broadcast.
	hub.pollChannels()
	expectNoMessage(t, msgs, 200*time.Millisecond)

	// Append another message and poll: the new one should broadcast.
	if err := fleetctx.SendToChannel(fleetDir, chName, "bob", "second msg"); err != nil {
		t.Fatalf("SendToChannel: %v", err)
	}
	hub.pollChannels()
	waitForMessage(t, msgs, "second msg", 2*time.Second)
}

func TestPollChannels_NoFleetDirIsNoOp(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHub("/nonexistent/dir", "/nonexistent/repo", "fleet", logger)
	hub.pollChannels() // must not panic
}

func TestPollAgentStates_BroadcastsOnStateChange(t *testing.T) {
	repoPath, fleetDir := setupRepo(t)

	statePath := filepath.Join(fleetDir, "states", "agent-a.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		t.Fatal(err)
	}

	cfg := map[string]interface{}{
		"repo_path": repoPath,
		"fleet_dir": fleetDir,
		"agents": []map[string]interface{}{
			{
				"name":            "agent-a",
				"branch":          "feat-a",
				"worktree_path":   repoPath,
				"status":          "running",
				"state_file_path": statePath,
			},
		},
	}
	cfgData, _ := json.MarshalIndent(cfg, "", "  ")
	if err := os.WriteFile(filepath.Join(fleetDir, "config.json"), cfgData, 0644); err != nil {
		t.Fatal(err)
	}

	stateBlob := `{"agent":"agent-a","state":"waiting","updated_at":"` + time.Now().UTC().Format(time.RFC3339Nano) + `"}`
	if err := os.WriteFile(statePath, []byte(stateBlob), 0644); err != nil {
		t.Fatal(err)
	}

	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHub(fleetDir, repoPath, "fleet", logger)

	msgs, cleanup := dialAndStreamMessages(t, hub)
	defer cleanup()

	// First poll: the new state should be broadcast.
	hub.pollAgentStates()
	got := waitForMessage(t, msgs, `"state":"waiting"`, 2*time.Second)
	if !strings.Contains(string(got), `"type":"agent_state"`) {
		t.Errorf("expected agent_state event, got: %s", got)
	}
	if !strings.Contains(string(got), `"agent":"agent-a"`) {
		t.Errorf("expected agent=agent-a in event, got: %s", got)
	}

	// Second poll without any state change: no broadcast.
	hub.pollAgentStates()
	expectNoMessage(t, msgs, 200*time.Millisecond)

	// Update the state file to "working" and poll again — should broadcast.
	stateBlob = `{"agent":"agent-a","state":"working","updated_at":"` + time.Now().UTC().Format(time.RFC3339Nano) + `"}`
	if err := os.WriteFile(statePath, []byte(stateBlob), 0644); err != nil {
		t.Fatal(err)
	}
	hub.pollAgentStates()
	waitForMessage(t, msgs, `"state":"working"`, 2*time.Second)
}

func TestPollAgentStates_NoFleetIsNoOp(t *testing.T) {
	logger := log.New(log.Writer(), "[test] ", 0)
	hub := NewHub("/nonexistent/dir", "/nonexistent/repo", "fleet", logger)
	hub.pollAgentStates() // must not panic
}
