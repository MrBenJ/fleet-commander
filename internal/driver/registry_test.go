package driver

import (
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/fleet"
)

func TestAgentStateConstants(t *testing.T) {
	tests := []struct {
		state AgentState
		want  string
	}{
		{StateWorking, "working"},
		{StateWaiting, "waiting"},
		{StateStopped, "stopped"},
		{StateStarting, "starting"},
	}
	for _, tt := range tests {
		if string(tt.state) != tt.want {
			t.Errorf("AgentState %v has value %q, want %q", tt.state, string(tt.state), tt.want)
		}
	}
}

func TestGetForAgent(t *testing.T) {
	t.Run("empty driver defaults to claude-code", func(t *testing.T) {
		agent := &fleet.Agent{Name: "test", Driver: ""}
		d, err := GetForAgent(agent)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Name() != "claude-code" {
			t.Errorf("expected 'claude-code', got %q", d.Name())
		}
	})

	t.Run("explicit claude-code", func(t *testing.T) {
		agent := &fleet.Agent{Name: "test", Driver: "claude-code"}
		d, err := GetForAgent(agent)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Name() != "claude-code" {
			t.Errorf("expected 'claude-code', got %q", d.Name())
		}
	})

	t.Run("codex delegates to Get", func(t *testing.T) {
		agent := &fleet.Agent{Name: "test", Driver: "codex"}
		d, err := GetForAgent(agent)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Name() != "codex" {
			t.Errorf("expected 'codex', got %q", d.Name())
		}
	})

	t.Run("generic with config returns GenericDriver", func(t *testing.T) {
		agent := &fleet.Agent{
			Name:   "test",
			Driver: "generic",
			DriverConfig: &fleet.DriverConfig{
				Command: "my-agent",
			},
		}
		d, err := GetForAgent(agent)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if d.Name() != "generic" {
			t.Errorf("expected 'generic', got %q", d.Name())
		}
		gd, ok := d.(*GenericDriver)
		if !ok {
			t.Fatal("expected *GenericDriver")
		}
		if gd.Config.Command != "my-agent" {
			t.Errorf("expected config command 'my-agent', got %q", gd.Config.Command)
		}
	})

	t.Run("generic without config errors", func(t *testing.T) {
		agent := &fleet.Agent{Name: "test", Driver: "generic"}
		_, err := GetForAgent(agent)
		if err == nil {
			t.Error("expected error for generic driver without config")
		}
	})

	t.Run("unknown driver errors", func(t *testing.T) {
		agent := &fleet.Agent{Name: "test", Driver: "nonexistent"}
		_, err := GetForAgent(agent)
		if err == nil {
			t.Error("expected error for unknown driver")
		}
	})
}
