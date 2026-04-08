package tui

import (
	"strings"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/monitor"
)

func TestRenderAgentItem_WaitingState(t *testing.T) {
	agent := &fleet.Agent{Name: "alice", Branch: "feature/alice", Status: "running", HooksOK: true}
	result := renderAgentItem(agent, monitor.StateWaiting, "some output", "", 0, 0)

	if !strings.Contains(result, "NEEDS INPUT") {
		t.Error("expected 'NEEDS INPUT' indicator for waiting state")
	}
	if !strings.Contains(result, "alice") {
		t.Error("expected agent name in output")
	}
	if !strings.Contains(result, "feature/alice") {
		t.Error("expected branch name in output")
	}
}

func TestRenderAgentItem_WorkingState(t *testing.T) {
	agent := &fleet.Agent{Name: "bob", Branch: "feature/bob", Status: "running", HooksOK: true}
	result := renderAgentItem(agent, monitor.StateWorking, "doing stuff", "", 0, 0)

	if !strings.Contains(result, "working") {
		t.Error("expected 'working' indicator")
	}
	// Working state should show preview with "…" prefix
	if !strings.Contains(result, "doing stuff") {
		t.Error("expected last line preview for working state")
	}
}

func TestRenderAgentItem_StoppedState(t *testing.T) {
	agent := &fleet.Agent{Name: "carol", Branch: "main", Status: "stopped", HooksOK: true}
	result := renderAgentItem(agent, monitor.StateStopped, "", "", 0, 0)

	if !strings.Contains(result, "stopped") {
		t.Error("expected 'stopped' indicator")
	}
}

func TestRenderAgentItem_StartingState(t *testing.T) {
	agent := &fleet.Agent{Name: "dan", Branch: "main", Status: "running", HooksOK: true}
	result := renderAgentItem(agent, monitor.StateStarting, "", "", 0, 0)

	if !strings.Contains(result, "starting") {
		t.Error("expected 'starting' indicator")
	}
}

func TestRenderAgentItem_WithRepoTag(t *testing.T) {
	agent := &fleet.Agent{Name: "alice", Branch: "main", Status: "running", HooksOK: true}
	result := renderAgentItem(agent, monitor.StateWorking, "", "[myrepo] ", 0, 1)

	if !strings.Contains(result, "[myrepo] alice") {
		t.Errorf("expected repo tag prepended to name, got: %s", result)
	}
}

func TestRenderAgentItem_WithoutRepoTag(t *testing.T) {
	agent := &fleet.Agent{Name: "alice", Branch: "main", Status: "running", HooksOK: true}
	result := renderAgentItem(agent, monitor.StateWorking, "", "", 0, 1)

	if strings.Contains(result, "[") {
		t.Error("expected no repo tag when empty string passed")
	}
	if !strings.Contains(result, "alice") {
		t.Error("expected agent name in output")
	}
}

func TestRenderAgentItem_LastLineTruncation(t *testing.T) {
	longLine := strings.Repeat("x", 80)
	agent := &fleet.Agent{Name: "alice", Branch: "main", Status: "running", HooksOK: true}
	result := renderAgentItem(agent, monitor.StateWaiting, longLine, "", 0, 0)

	// The truncated line should be 57 chars + "..." = 60 chars
	truncated := longLine[:57] + "..."
	if !strings.Contains(result, truncated) {
		t.Error("expected long last line to be truncated to 57 chars + '...'")
	}
	// Should NOT contain the full 80-char line
	if strings.Contains(result, longLine) {
		t.Error("expected long line to be truncated, but full line found")
	}
}

func TestRenderAgentItem_LastLineExactly60NotTruncated(t *testing.T) {
	line60 := strings.Repeat("y", 60)
	agent := &fleet.Agent{Name: "alice", Branch: "main", Status: "running", HooksOK: true}
	result := renderAgentItem(agent, monitor.StateWorking, line60, "", 0, 0)

	if !strings.Contains(result, line60) {
		t.Error("expected 60-char line to appear without truncation")
	}
}

func TestRenderAgentItem_LastLineShortNotTruncated(t *testing.T) {
	shortLine := "hello world"
	agent := &fleet.Agent{Name: "alice", Branch: "main", Status: "running", HooksOK: true}
	result := renderAgentItem(agent, monitor.StateWaiting, shortLine, "", 0, 0)

	if !strings.Contains(result, shortLine) {
		t.Error("expected short last line to appear without truncation")
	}
}

func TestRenderAgentItem_NoPreviewWhenStopped(t *testing.T) {
	agent := &fleet.Agent{Name: "alice", Branch: "main", Status: "stopped", HooksOK: true}
	result := renderAgentItem(agent, monitor.StateStopped, "some output", "", 0, 0)

	// Stopped state should not show the last line preview
	if strings.Contains(result, "some output") {
		t.Error("expected no preview for stopped state, but last line appeared")
	}
}

func TestRenderAgentItem_HooksWarning_NotStopped(t *testing.T) {
	agent := &fleet.Agent{Name: "alice", Branch: "main", Status: "running", HooksOK: false}
	result := renderAgentItem(agent, monitor.StateWorking, "", "", 0, 0)

	if !strings.Contains(result, "hooks") {
		t.Error("expected hooks warning when HooksOK is false and status is not stopped")
	}
}

func TestRenderAgentItem_NoHooksWarning_WhenStopped(t *testing.T) {
	agent := &fleet.Agent{Name: "alice", Branch: "main", Status: "stopped", HooksOK: false}
	result := renderAgentItem(agent, monitor.StateStopped, "", "", 0, 0)

	if strings.Contains(result, "hooks") {
		t.Error("expected no hooks warning when status is stopped, even if HooksOK is false")
	}
}

func TestRenderAgentItem_NoHooksWarning_WhenHooksOK(t *testing.T) {
	agent := &fleet.Agent{Name: "alice", Branch: "main", Status: "running", HooksOK: true}
	result := renderAgentItem(agent, monitor.StateWorking, "", "", 0, 0)

	if strings.Contains(result, "hooks") {
		t.Error("expected no hooks warning when HooksOK is true")
	}
}

func TestRenderAgentItem_SelectedVsUnselected(t *testing.T) {
	agent := &fleet.Agent{Name: "alice", Branch: "main", Status: "running", HooksOK: true}

	selected := renderAgentItem(agent, monitor.StateWorking, "", "", 0, 0)
	unselected := renderAgentItem(agent, monitor.StateWorking, "", "", 1, 0)

	if !strings.Contains(selected, "> alice") {
		t.Error("expected '> ' prefix for selected item")
	}
	if strings.Contains(unselected, "> alice") {
		t.Error("expected no '> ' prefix for unselected item")
	}
}
