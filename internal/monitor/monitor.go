package monitor

import (
	"strings"
	"time"

	"github.com/teknal/fleet-commander/internal/tmux"
)

// AgentState represents what the agent is currently doing
type AgentState string

const (
	StateWorking  AgentState = "working"  // Agent is actively producing output
	StateWaiting  AgentState = "waiting"  // Agent is waiting for user input
	StateStopped  AgentState = "stopped"  // Session not running
	StateStarting AgentState = "starting" // Session just created
)

// Snapshot holds a point-in-time capture of an agent's terminal
type Snapshot struct {
	AgentName string
	Content   string
	State     AgentState
	LastLine  string
	Timestamp time.Time
}

// Monitor watches agent tmux sessions and detects their state
type Monitor struct {
	tmux      *tmux.Manager
	snapshots map[string]*Snapshot
}

// NewMonitor creates a new agent monitor
func NewMonitor(tm *tmux.Manager) *Monitor {
	return &Monitor{
		tmux:      tm,
		snapshots: make(map[string]*Snapshot),
	}
}

// Check captures the current state of an agent
func (m *Monitor) Check(agentName string) *Snapshot {
	snap := &Snapshot{
		AgentName: agentName,
		Timestamp: time.Now(),
	}

	// Check if session exists
	if !m.tmux.SessionExists(agentName) {
		snap.State = StateStopped
		m.snapshots[agentName] = snap
		return snap
	}

	// Capture pane content
	content, err := m.tmux.CapturePane(agentName)
	if err != nil {
		snap.State = StateStopped
		m.snapshots[agentName] = snap
		return snap
	}

	snap.Content = content
	snap.LastLine = getLastNonEmptyLine(content)
	snap.State = detectState(snap.LastLine, content)

	m.snapshots[agentName] = snap
	return snap
}

// GetSnapshot returns the last snapshot for an agent
func (m *Monitor) GetSnapshot(agentName string) *Snapshot {
	return m.snapshots[agentName]
}

// detectState analyzes terminal content to determine agent state
func detectState(lastLine, fullContent string) AgentState {
	lastLine = strings.TrimSpace(lastLine)
	stripped := stripANSI(fullContent)

	// Empty content = probably starting up
	if strings.TrimSpace(stripped) == "" {
		return StateStarting
	}

	// Check the FULL content for Claude Code patterns (not just last line)
	// Claude Code shows multi-line prompts, so the last line alone isn't enough

	// Claude Code permission/action prompts
	// These appear as multi-line blocks ending with "Esc to cancel"
	if strings.Contains(stripped, "Esc to cancel") {
		return StateWaiting
	}

	// Claude Code tool confirmation: "Do you want to proceed?"
	if strings.Contains(stripped, "Do you want to proceed") {
		return StateWaiting
	}

	// Claude Code numbered choice menus (❯ 1. Yes / 2. No pattern)
	if strings.Contains(stripped, "❯") && (strings.Contains(stripped, "1. Yes") || strings.Contains(stripped, "1.") && strings.Contains(stripped, "2.")) {
		return StateWaiting
	}

	// Claude Code asking a question in conversation
	// Check last few lines for question patterns
	lines := strings.Split(stripped, "\n")
	lastFew := getLastNonEmptyLines(lines, 5)
	for _, line := range lastFew {
		trimmed := strings.TrimSpace(line)

		// Question marks at end of a substantive line (not spinners or progress)
		if strings.HasSuffix(trimmed, "?") && len(trimmed) > 5 {
			return StateWaiting
		}

		// Yes/No prompts
		if strings.Contains(trimmed, "(y/n)") ||
			strings.Contains(trimmed, "(Y/n)") ||
			strings.Contains(trimmed, "[Y/n]") ||
			strings.Contains(trimmed, "[y/N]") {
			return StateWaiting
		}

		// Permission patterns
		if strings.Contains(trimmed, "Allow") && strings.HasSuffix(trimmed, "?") {
			return StateWaiting
		}
	}

	// Claude Code input prompt: just ">" or "❯" on its own line
	lastNonEmpty := strings.TrimSpace(stripANSI(lastLine))
	if lastNonEmpty == ">" || lastNonEmpty == "❯" || lastNonEmpty == "$" {
		return StateWaiting
	}

	// Active work indicators (check last few lines)
	for _, line := range lastFew {
		trimmed := strings.TrimSpace(line)
		for _, pattern := range []string{
			"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏", // spinners
			"Thinking",
			"in progress",
			"◼", // in-progress task marker
		} {
			if strings.Contains(trimmed, pattern) {
				return StateWorking
			}
		}
	}

	// Default: assume working
	return StateWorking
}

// getLastNonEmptyLines returns the last N non-empty lines
func getLastNonEmptyLines(lines []string, n int) []string {
	var result []string
	for i := len(lines) - 1; i >= 0 && len(result) < n; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			result = append(result, lines[i])
		}
	}
	return result
}

// isClaudeCodePrompt detects Claude Code's input prompt
func isClaudeCodePrompt(line string) bool {
	trimmed := strings.TrimSpace(line)

	// Claude Code shows ">" as its prompt when waiting for input
	// It may have ANSI escape codes around it
	stripped := stripANSI(trimmed)
	stripped = strings.TrimSpace(stripped)

	if stripped == ">" || stripped == "❯" || stripped == "$" {
		return true
	}

	return false
}

// stripANSI removes ANSI escape sequences from a string
func stripANSI(s string) string {
	var result strings.Builder
	inEscape := false

	for i := 0; i < len(s); i++ {
		if s[i] == '\033' {
			inEscape = true
			continue
		}
		if inEscape {
			if (s[i] >= 'a' && s[i] <= 'z') || (s[i] >= 'A' && s[i] <= 'Z') {
				inEscape = false
			}
			continue
		}
		result.WriteByte(s[i])
	}

	return result.String()
}

// getLastNonEmptyLine returns the last non-empty line from content
func getLastNonEmptyLine(content string) string {
	lines := strings.Split(content, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed != "" {
			return lines[i]
		}
	}
	return ""
}
