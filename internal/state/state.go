package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// AgentState is the contents of a state file written by fleet hooks.
type AgentState struct {
	Agent     string    `json:"agent"`
	State     string    `json:"state"` // "waiting" or "working"
	UpdatedAt time.Time `json:"updated_at"`
}

// IsStale returns true if the state is older than ttl.
func (s AgentState) IsStale(ttl time.Duration) bool {
	return time.Since(s.UpdatedAt) > ttl
}

// Write atomically writes the agent state to path.
func Write(path, agentName, stateStr string) error {
	// Validate stateStr
	if stateStr != "waiting" && stateStr != "working" {
		return fmt.Errorf("invalid state: %q, must be 'waiting' or 'working'", stateStr)
	}

	state := &AgentState{
		Agent:     agentName,
		State:     stateStr,
		UpdatedAt: time.Now(),
	}

	data, err := json.Marshal(state)
	if err != nil {
		return err
	}

	// Ensure the directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Write atomically using a temporary file
	tmpFile, err := os.CreateTemp(dir, ".state.tmp")
	if err != nil {
		return err
	}

	if _, err := tmpFile.Write(data); err != nil {
		os.Remove(tmpFile.Name())
		tmpFile.Close()
		return err
	}

	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpFile.Name())
		return err
	}

	// Atomic rename
	if err := os.Rename(tmpFile.Name(), path); err != nil {
		os.Remove(tmpFile.Name())
		return err
	}
	return nil
}

// Read reads and parses the state file at path.
func Read(path string) (*AgentState, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state AgentState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}
