package monitor_test

import (
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/monitor"
	"github.com/MrBenJ/fleet-commander/internal/tmux"
)

func TestGetSnapshotReturnsLastCheck(t *testing.T) {
	runner := &mockRunner{
		sessionExists: true,
		paneContent:   "Some output\nEsc to cancel",
	}
	tm := tmux.NewManagerWithRunner("fleet", runner)
	mon := monitor.NewMonitor(tm)

	// Before any Check, GetSnapshot returns nil.
	if got := mon.GetSnapshot("never-checked"); got != nil {
		t.Errorf("expected nil snapshot for unchecked agent, got %+v", got)
	}

	// After Check, GetSnapshot returns the cached snapshot.
	check := mon.Check("agent-a")
	got := mon.GetSnapshot("agent-a")
	if got == nil {
		t.Fatal("GetSnapshot returned nil after Check")
	}
	if got.AgentName != "agent-a" {
		t.Errorf("AgentName = %q, want %q", got.AgentName, "agent-a")
	}
	if got.State != check.State {
		t.Errorf("snapshot state mismatch: got %v, check returned %v", got.State, check.State)
	}
}

func TestGetSnapshotForUnknownAgent(t *testing.T) {
	mon := monitor.NewMonitor(nil)
	if snap := mon.GetSnapshot("ghost"); snap != nil {
		t.Errorf("expected nil for never-seen agent, got %+v", snap)
	}
}

func TestCheckWithStateFile_UnknownStateStringFallsBackToStopped(t *testing.T) {
	// Write a state file with an unknown state value — Read won't reject it
	// (the validator only fires in Write), so stateFromString should map it
	// to StateStopped.
	dir := t.TempDir()
	statePath := dir + "/weird.json"

	if err := writeRawState(statePath, `{"agent":"x","state":"frobnicating","updated_at":"`+nowRFC3339()+`"}`); err != nil {
		t.Fatalf("write raw state: %v", err)
	}

	mon := monitor.NewMonitor(nil)
	snap := mon.CheckWithStateFile("x", statePath)

	if snap.State != monitor.StateStopped {
		t.Errorf("expected StateStopped for unknown state string, got %v", snap.State)
	}
}
