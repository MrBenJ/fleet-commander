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
