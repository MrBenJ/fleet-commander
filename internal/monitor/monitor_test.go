package monitor_test

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/MrBenJ/fleet-commander/internal/monitor"
	"github.com/MrBenJ/fleet-commander/internal/state"
)

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
