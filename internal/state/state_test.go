package state_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/teknal/fleet-commander/internal/state"
)

func TestWriteAndRead(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "state.json")

	// Write "waiting" state for "feat-auth"
	err := state.Write(testPath, "feat-auth", "waiting")
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// Read it back
	agentState, err := state.Read(testPath)
	if err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	// Assert Agent and State fields
	if agentState.Agent != "feat-auth" {
		t.Errorf("Agent = %q, want %q", agentState.Agent, "feat-auth")
	}
	if agentState.State != "waiting" {
		t.Errorf("State = %q, want %q", agentState.State, "waiting")
	}
	if agentState.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}
}

func TestReadMissing(t *testing.T) {
	// Try to read from a nonexistent path
	_, err := state.Read(filepath.Join(t.TempDir(), "does-not-exist.json"))
	if err == nil {
		t.Error("Read should return an error for nonexistent file")
	}
}

func TestReadMalformedJSON(t *testing.T) {
	// Create a temporary directory and file with invalid JSON
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "malformed.json")

	// Write invalid JSON
	if err := os.WriteFile(testPath, []byte("{not valid json"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Try to read the malformed file
	_, err := state.Read(testPath)
	if err == nil {
		t.Error("Read should return an error for malformed JSON")
	}
}

func TestIsStale(t *testing.T) {
	// Test with 2 minutes old state and 90 second TTL (should be stale)
	staleState := &state.AgentState{
		Agent:     "test-agent",
		State:     "working",
		UpdatedAt: time.Now().Add(-2 * time.Minute),
	}
	if !staleState.IsStale(90 * time.Second) {
		t.Error("State with UpdatedAt 2 minutes ago should be stale with 90s TTL")
	}

	// Test with current time and 90 second TTL (should not be stale)
	freshState := &state.AgentState{
		Agent:     "test-agent",
		State:     "working",
		UpdatedAt: time.Now(),
	}
	if freshState.IsStale(90 * time.Second) {
		t.Error("State with current UpdatedAt should not be stale with 90s TTL")
	}
}
