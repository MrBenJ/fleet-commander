package hangar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
)

func startTestServer(t *testing.T) (*Server, context.CancelFunc) {
	t.Helper()
	srv := NewServer(Config{Port: 0})
	ctx, cancel := context.WithCancel(context.Background())

	go func() { srv.Start(ctx) }()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if srv.Addr() != "" {
			return srv, cancel
		}
		time.Sleep(10 * time.Millisecond)
	}

	cancel()
	t.Fatal("server did not start in time")
	return nil, nil
}

func TestHealthEndpoint(t *testing.T) {
	srv, cancel := startTestServer(t)
	defer cancel()

	resp, err := http.Get(fmt.Sprintf("http://%s/api/health", srv.Addr()))
	if err != nil {
		t.Fatalf("health request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]string
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if result["status"] != "ok" {
		t.Fatalf("expected status ok, got %s", result["status"])
	}
}

func TestServerRoutes(t *testing.T) {
	srv, cancel := startTestServer(t)
	defer cancel()

	base := fmt.Sprintf("http://%s", srv.Addr())

	// Health check
	resp, err := http.Get(base + "/api/health")
	if err != nil {
		t.Fatalf("health: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("health: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Unknown API route should 404
	resp, err = http.Get(base + "/api/nonexistent")
	if err != nil {
		t.Fatalf("404 test: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}
