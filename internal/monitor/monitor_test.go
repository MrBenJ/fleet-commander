package monitor_test

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/MrBenJ/fleet-commander/internal/driver"
	"github.com/MrBenJ/fleet-commander/internal/monitor"
	"github.com/MrBenJ/fleet-commander/internal/state"
	"github.com/MrBenJ/fleet-commander/internal/tmux"
)

// mockRunner implements tmux.CommandRunner for testing.
type mockRunner struct {
	sessionExists bool
	paneContent   string
}

func (m *mockRunner) Run(name string, args ...string) error {
	// "has-session" is used by SessionExists
	if len(args) > 0 && args[0] == "has-session" {
		if m.sessionExists {
			return nil
		}
		return fmt.Errorf("session not found")
	}
	return nil
}

func (m *mockRunner) Output(name string, args ...string) ([]byte, error) {
	// "capture-pane" is used by CapturePane
	if len(args) > 0 && args[0] == "capture-pane" {
		return []byte(m.paneContent), nil
	}
	return nil, nil
}

func (m *mockRunner) RunInteractive(name string, args ...string) error { return nil }
func (m *mockRunner) LookPath(name string) (string, error)             { return name, nil }

func TestCheckPrefersStateFile(t *testing.T) {
	f, err := os.CreateTemp("", "state-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	if err := state.Write(f.Name(), "feat-auth", "waiting"); err != nil {
		t.Fatal(err)
	}

	mon := monitor.NewMonitor(nil)
	snap := mon.CheckWithStateFile("feat-auth", f.Name())

	if snap.State != monitor.StateWaiting {
		t.Errorf("expected StateWaiting, got %v", snap.State)
	}
}

func TestCheckIgnoresStaleStateFile(t *testing.T) {
	f, err := os.CreateTemp("", "state-stale-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	stale := state.AgentState{
		Agent:     "feat-auth",
		State:     "waiting",
		UpdatedAt: time.Now().Add(-15 * time.Minute),
	}
	data, _ := json.Marshal(stale)
	os.WriteFile(f.Name(), data, 0644)

	mon := monitor.NewMonitor(nil)
	snap := mon.CheckWithStateFile("feat-auth", f.Name())

	if snap.State == monitor.StateWaiting {
		t.Errorf("expected stale state file to be ignored, but got StateWaiting")
	}
}

func TestCheckMissingStateFileFallsBack(t *testing.T) {
	mon := monitor.NewMonitor(nil)
	snap := mon.CheckWithStateFile("no-agent", "/nonexistent/path.json")

	if snap.State != monitor.StateStopped {
		t.Errorf("expected StateStopped, got %v", snap.State)
	}
}

func TestCheckUsesDriverDetectState(t *testing.T) {
	runner := &mockRunner{
		sessionExists: true,
		paneContent:   "Some output\nEsc to cancel",
	}
	tm := tmux.NewManagerWithRunner("fleet", runner)
	mon := monitor.NewMonitor(tm)

	// Register the claude-code driver
	drv, err := driver.Get("claude-code")
	if err != nil {
		t.Fatal(err)
	}
	mon.SetDriver("test-agent", drv)

	snap := mon.Check("test-agent")

	if snap.State != monitor.StateWaiting {
		t.Errorf("expected StateWaiting from driver, got %v", snap.State)
	}
}

func TestCheckDriverDetectsWorking(t *testing.T) {
	runner := &mockRunner{
		sessionExists: true,
		paneContent:   "Generating code...\nesc to interrupt",
	}
	tm := tmux.NewManagerWithRunner("fleet", runner)
	mon := monitor.NewMonitor(tm)

	drv, _ := driver.Get("claude-code")
	mon.SetDriver("test-agent", drv)

	snap := mon.Check("test-agent")

	if snap.State != monitor.StateWorking {
		t.Errorf("expected StateWorking from driver, got %v", snap.State)
	}
}

func TestCheckDriverDetectsStarting(t *testing.T) {
	runner := &mockRunner{
		sessionExists: true,
		paneContent:   "",
	}
	tm := tmux.NewManagerWithRunner("fleet", runner)
	mon := monitor.NewMonitor(tm)

	drv, _ := driver.Get("claude-code")
	mon.SetDriver("test-agent", drv)

	snap := mon.Check("test-agent")

	if snap.State != monitor.StateStarting {
		t.Errorf("expected StateStarting from driver, got %v", snap.State)
	}
}

func TestCheckWithoutDriverUsesLegacyFallback(t *testing.T) {
	runner := &mockRunner{
		sessionExists: true,
		paneContent:   "Some output\nEsc to cancel",
	}
	tm := tmux.NewManagerWithRunner("fleet", runner)
	mon := monitor.NewMonitor(tm)

	// No driver registered — should use legacy detectState
	snap := mon.Check("test-agent")

	if snap.State != monitor.StateWaiting {
		t.Errorf("expected StateWaiting from legacy fallback, got %v", snap.State)
	}
}

func TestCheckSessionNotExistsReturnsStopped(t *testing.T) {
	runner := &mockRunner{
		sessionExists: false,
	}
	tm := tmux.NewManagerWithRunner("fleet", runner)
	mon := monitor.NewMonitor(tm)

	drv, _ := driver.Get("claude-code")
	mon.SetDriver("test-agent", drv)

	snap := mon.Check("test-agent")

	if snap.State != monitor.StateStopped {
		t.Errorf("expected StateStopped, got %v", snap.State)
	}
}

func TestSetDriverOverwritesPrevious(t *testing.T) {
	mon := monitor.NewMonitor(nil)

	drv1, _ := driver.Get("claude-code")
	mon.SetDriver("agent-a", drv1)

	// Overwrite with same driver (just verifying no panic/error)
	drv2, _ := driver.Get("claude-code")
	mon.SetDriver("agent-a", drv2)

	// Verify it's registered by using CheckWithStateFile (falls back to stopped with nil tmux)
	snap := mon.CheckWithStateFile("agent-a", "")
	if snap.State != monitor.StateStopped {
		t.Errorf("expected StateStopped with nil tmux, got %v", snap.State)
	}
}

func TestCheckWithStateFilePrefersStateFileOverDriver(t *testing.T) {
	// Write a state file saying "waiting"
	f, err := os.CreateTemp("", "state-*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	f.Close()

	if err := state.Write(f.Name(), "test-agent", "waiting"); err != nil {
		t.Fatal(err)
	}

	// Set up a tmux runner that would return "working" content
	runner := &mockRunner{
		sessionExists: true,
		paneContent:   "Generating code...\nesc to interrupt",
	}
	tm := tmux.NewManagerWithRunner("fleet", runner)
	mon := monitor.NewMonitor(tm)

	drv, _ := driver.Get("claude-code")
	mon.SetDriver("test-agent", drv)

	// State file should win over driver/pane scraping
	snap := mon.CheckWithStateFile("test-agent", f.Name())

	if snap.State != monitor.StateWaiting {
		t.Errorf("expected StateWaiting from state file, got %v (state file should take priority)", snap.State)
	}
}

func TestCheckDriverHandlesANSI(t *testing.T) {
	runner := &mockRunner{
		sessionExists: true,
		paneContent:   "\x1b[32mEsc to cancel\x1b[0m",
	}
	tm := tmux.NewManagerWithRunner("fleet", runner)
	mon := monitor.NewMonitor(tm)

	drv, _ := driver.Get("claude-code")
	mon.SetDriver("test-agent", drv)

	snap := mon.Check("test-agent")

	if snap.State != monitor.StateWaiting {
		t.Errorf("expected StateWaiting after ANSI stripping, got %v", snap.State)
	}
}
