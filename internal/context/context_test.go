//go:build !windows

package context_test

import (
	"os"
	"path/filepath"
	"testing"

	fleetctx "github.com/teknal/fleet-commander/internal/context"
)

func TestLoadMissingFileReturnsEmptyContext(t *testing.T) {
	dir := t.TempDir()
	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx.Shared != "" {
		t.Errorf("expected empty shared, got %q", ctx.Shared)
	}
	if len(ctx.Agents) != 0 {
		t.Errorf("expected empty agents map, got %v", ctx.Agents)
	}
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	ctx := &fleetctx.Context{
		Shared: "use JWT auth",
		Agents: map[string]string{
			"auth-agent": "User model done",
		},
	}
	if err := fleetctx.Save(dir, ctx); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Shared != "use JWT auth" {
		t.Errorf("shared mismatch: got %q", loaded.Shared)
	}
	if loaded.Agents["auth-agent"] != "User model done" {
		t.Errorf("agent mismatch: got %q", loaded.Agents["auth-agent"])
	}
}

func TestWriteAgent(t *testing.T) {
	dir := t.TempDir()

	// First agent writes
	if err := fleetctx.WriteAgent(dir, "auth-agent", "User model done"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// Second agent writes — should not clobber first
	if err := fleetctx.WriteAgent(dir, "api-agent", "Endpoints defined"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Agents["auth-agent"] != "User model done" {
		t.Errorf("auth-agent mismatch: got %q", ctx.Agents["auth-agent"])
	}
	if ctx.Agents["api-agent"] != "Endpoints defined" {
		t.Errorf("api-agent mismatch: got %q", ctx.Agents["api-agent"])
	}
}

func TestWriteShared(t *testing.T) {
	dir := t.TempDir()

	// Write an agent first
	if err := fleetctx.WriteAgent(dir, "auth-agent", "User model done"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// Write shared — should not clobber agent
	if err := fleetctx.WriteShared(dir, "API uses JWT"); err != nil {
		t.Fatalf("WriteShared failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Shared != "API uses JWT" {
		t.Errorf("shared mismatch: got %q", ctx.Shared)
	}
	if ctx.Agents["auth-agent"] != "User model done" {
		t.Errorf("auth-agent was clobbered: got %q", ctx.Agents["auth-agent"])
	}
}

func TestLoadMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "context.json")
	os.WriteFile(path, []byte("{invalid json"), 0644)

	_, err := fleetctx.Load(dir)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}
