package tui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/monitor"
)

var hooksWarnStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF6666"))

// renderAgentItem renders an agent list item. If repoTag is non-empty, it is
// prepended to the agent name (used in the multi-repo view).
func renderAgentItem(agent *fleet.Agent, state monitor.AgentState, lastLine, repoTag string, index, selectedIndex int) string {
	var indicator string
	switch state {
	case monitor.StateWaiting:
		indicator = waitingStyle.Render("⏳ NEEDS INPUT")
	case monitor.StateWorking:
		indicator = runningStyle.Render("● working")
	case monitor.StateStarting:
		indicator = runningStyle.Render("◐ starting")
	default:
		indicator = stoppedStyle.Render("○ stopped")
	}

	if !agent.HooksOK && agent.Status != "stopped" {
		indicator += " " + hooksWarnStyle.Render("⚠ hooks")
	}

	name := repoTag + agent.Name
	if index == selectedIndex {
		name = selectedItemStyle.Render("> " + name)
	} else {
		name = itemStyle.Render("  " + name)
	}

	desc := statusStyle.Render(fmt.Sprintf("    %s  %s", agent.Branch, indicator))

	preview := ""
	if lastLine != "" && (state == monitor.StateWaiting || state == monitor.StateWorking) {
		line := lastLine
		if len(line) > 60 {
			line = line[:57] + "..."
		}
		if state == monitor.StateWaiting {
			preview = waitingStyle.Render(fmt.Sprintf("    💬 %s", line))
		} else {
			preview = statusStyle.Render(fmt.Sprintf("    … %s", line))
		}
	}

	return name + "\n" + desc + "\n" + preview
}
