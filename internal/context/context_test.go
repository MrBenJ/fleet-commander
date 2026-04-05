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
	if err := os.WriteFile(path, []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("failed to write test fixture: %v", err)
	}

	_, err := fleetctx.Load(dir)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestAppendLog(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.AppendLog(dir, "auth-agent", "found auth bug"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(ctx.Log))
	}
	if ctx.Log[0].Agent != "auth-agent" {
		t.Errorf("agent mismatch: got %q", ctx.Log[0].Agent)
	}
	if ctx.Log[0].Message != "found auth bug" {
		t.Errorf("message mismatch: got %q", ctx.Log[0].Message)
	}
	if ctx.Log[0].Timestamp.IsZero() {
		t.Error("timestamp should not be zero")
	}
}

func TestAppendLogPreservesExistingData(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.WriteAgent(dir, "auth-agent", "working on auth"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	if err := fleetctx.AppendLog(dir, "api-agent", "endpoints ready"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Agents["auth-agent"] != "working on auth" {
		t.Errorf("agent data clobbered: got %q", ctx.Agents["auth-agent"])
	}
	if len(ctx.Log) != 1 {
		t.Fatalf("expected 1 log entry, got %d", len(ctx.Log))
	}
}

func TestAppendLogMultipleEntries(t *testing.T) {
	dir := t.TempDir()

	if err := fleetctx.AppendLog(dir, "agent-a", "first"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}
	if err := fleetctx.AppendLog(dir, "agent-b", "second"); err != nil {
		t.Fatalf("AppendLog failed: %v", err)
	}

	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(ctx.Log) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(ctx.Log))
	}
	if ctx.Log[0].Message != "first" {
		t.Errorf("first entry: got %q", ctx.Log[0].Message)
	}
	if ctx.Log[1].Message != "second" {
		t.Errorf("second entry: got %q", ctx.Log[1].Message)
	}
}

func TestAppendLogEmptyMessage(t *testing.T) {
	dir := t.TempDir()
	err := fleetctx.AppendLog(dir, "agent-a", "")
	if err == nil {
		t.Fatal("expected error for empty message, got nil")
	}
}

func TestMultiAgentWorkflow(t *testing.T) {
	dir := t.TempDir()

	// User sets shared context
	if err := fleetctx.WriteShared(dir, "API uses JWT. Base path /v2."); err != nil {
		t.Fatalf("WriteShared failed: %v", err)
	}

	// auth-agent writes its section
	if err := fleetctx.WriteAgent(dir, "auth-agent", "User model at internal/auth/user.go. @api-agent merge fleet/auth"); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// api-agent writes its section
	if err := fleetctx.WriteAgent(dir, "api-agent", "Endpoints defined. Waiting on auth model."); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	// Read full context
	ctx, err := fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if ctx.Shared != "API uses JWT. Base path /v2." {
		t.Errorf("shared: got %q", ctx.Shared)
	}
	if ctx.Agents["auth-agent"] != "User model at internal/auth/user.go. @api-agent merge fleet/auth" {
		t.Errorf("auth-agent: got %q", ctx.Agents["auth-agent"])
	}
	if ctx.Agents["api-agent"] != "Endpoints defined. Waiting on auth model." {
		t.Errorf("api-agent: got %q", ctx.Agents["api-agent"])
	}

	// auth-agent overwrites its section
	if err := fleetctx.WriteAgent(dir, "auth-agent", "Auth complete. All tests passing."); err != nil {
		t.Fatalf("WriteAgent failed: %v", err)
	}

	ctx, err = fleetctx.Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if ctx.Agents["auth-agent"] != "Auth complete. All tests passing." {
		t.Errorf("auth-agent after overwrite: got %q", ctx.Agents["auth-agent"])
	}
	// api-agent should be untouched
	if ctx.Agents["api-agent"] != "Endpoints defined. Waiting on auth model." {
		t.Errorf("api-agent was clobbered: got %q", ctx.Agents["api-agent"])
	}
}
