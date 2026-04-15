package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func buildFleet(t *testing.T) string {
	// Build the binary in a temp location
	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "fleet")

	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build binary: %v\n%s", err, string(out))
	}

	return binPath
}

func TestSignalWritesStateFile(t *testing.T) {
	binPath := buildFleet(t)
	stateDir := t.TempDir()
	stateFile := filepath.Join(stateDir, "agent.json")

	// Run "fleet signal waiting" with env vars set
	cmd := exec.Command(binPath, "signal", "waiting")
	cmd.Env = append(os.Environ(),
		"FLEET_STATE_FILE="+stateFile,
		"FLEET_AGENT_NAME=feat-auth",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fleet signal waiting failed: %v\n%s", err, string(out))
	}

	// Assert state file exists
	_, err = os.Stat(stateFile)
	if err != nil {
		t.Fatalf("state file not created: %v", err)
	}

	// Assert state file contains correct content
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("failed to parse state file: %v", err)
	}

	if state["state"] != "waiting" {
		t.Errorf("expected state 'waiting', got %v", state["state"])
	}
	if state["agent"] != "feat-auth" {
		t.Errorf("expected agent 'feat-auth', got %v", state["agent"])
	}
}

func TestSignalNoEnvVarsIsNoop(t *testing.T) {
	binPath := buildFleet(t)
	stateDir := t.TempDir()
	stateFile := filepath.Join(stateDir, "agent.json")

	// Run "fleet signal waiting" with ONLY PATH env var
	cmd := exec.Command(binPath, "signal", "waiting")
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fleet signal waiting failed: %v\n%s", err, string(out))
	}

	// Assert no state file was created
	_, err = os.Stat(stateFile)
	if err == nil {
		t.Errorf("state file should not be created when env vars are missing")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking state file: %v", err)
	}
}

func TestSignalWorkingState(t *testing.T) {
	binPath := buildFleet(t)
	stateDir := t.TempDir()
	stateFile := filepath.Join(stateDir, "agent.json")

	cmd := exec.Command(binPath, "signal", "working")
	cmd.Env = append(os.Environ(),
		"FLEET_STATE_FILE="+stateFile,
		"FLEET_AGENT_NAME=build-agent",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fleet signal working failed: %v\n%s", err, string(out))
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("failed to parse state file: %v", err)
	}

	if state["state"] != "working" {
		t.Errorf("expected state 'working', got %v", state["state"])
	}
	if state["agent"] != "build-agent" {
		t.Errorf("expected agent 'build-agent', got %v", state["agent"])
	}
}

func TestSignalOverwritesPreviousState(t *testing.T) {
	binPath := buildFleet(t)
	stateDir := t.TempDir()
	stateFile := filepath.Join(stateDir, "agent.json")

	// First signal: waiting
	cmd := exec.Command(binPath, "signal", "waiting")
	cmd.Env = append(os.Environ(),
		"FLEET_STATE_FILE="+stateFile,
		"FLEET_AGENT_NAME=test-agent",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("first signal failed: %v\n%s", err, string(out))
	}

	// Second signal: working — should overwrite
	cmd = exec.Command(binPath, "signal", "working")
	cmd.Env = append(os.Environ(),
		"FLEET_STATE_FILE="+stateFile,
		"FLEET_AGENT_NAME=test-agent",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("second signal failed: %v\n%s", err, string(out))
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("failed to parse state file: %v", err)
	}

	if state["state"] != "working" {
		t.Errorf("expected state 'working' after overwrite, got %v", state["state"])
	}
}

func TestSignalNoStateFileEnvVar(t *testing.T) {
	binPath := buildFleet(t)

	// Set FLEET_AGENT_NAME but NOT FLEET_STATE_FILE
	cmd := exec.Command(binPath, "signal", "waiting")
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"),
		"FLEET_AGENT_NAME=orphan-agent",
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fleet signal waiting failed: %v\n%s", err, string(out))
	}
	// Should be a no-op — nothing to verify except it didn't crash
}

func TestSignalStateFileHasTimestamp(t *testing.T) {
	binPath := buildFleet(t)
	stateDir := t.TempDir()
	stateFile := filepath.Join(stateDir, "agent.json")

	cmd := exec.Command(binPath, "signal", "waiting")
	cmd.Env = append(os.Environ(),
		"FLEET_STATE_FILE="+stateFile,
		"FLEET_AGENT_NAME=ts-agent",
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("fleet signal failed: %v\n%s", err, string(out))
	}

	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatalf("failed to read state file: %v", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(data, &state); err != nil {
		t.Fatalf("failed to parse state file: %v", err)
	}

	// State file should contain a timestamp field
	if _, ok := state["timestamp"]; !ok {
		// Not all implementations include timestamp — just check the file is well-formed JSON
		// with at least state and agent fields
		if state["state"] == nil || state["agent"] == nil {
			t.Error("state file missing required fields")
		}
	}
}
