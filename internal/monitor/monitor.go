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

	// Empty content = probably starting up
	if lastLine == "" {
		return StateStarting
	}

	// Claude Code waiting-for-input patterns
	waitingPatterns := []string{
		// Claude Code prompt
		">",
		"❯",
		// Question patterns
		"?",
		"(y/n)",
		"(Y/n)",
		"(yes/no)",
		"[Y/n]",
		"[y/N]",
		// Permission prompts
		"Allow?",
		"Approve?",
		"Continue?",
		"Proceed?",
		// Tool use confirmation
		"Do you want",
		"Would you like",
		"Should I",
		// Generic input indicators
		"Enter ",
		"Type ",
		"Input:",
		"Press ",
	}

	for _, pattern := range waitingPatterns {
		if strings.HasSuffix(lastLine, pattern) || strings.Contains(lastLine, pattern) {
			return StateWaiting
		}
	}

	// Claude Code specific: check for the input prompt marker
	// Claude Code typically shows a colored ">" or similar at the bottom
	if isClaudeCodePrompt(lastLine) {
		return StateWaiting
	}

	// If we see active output indicators, it's working
	workingPatterns := []string{
		"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏", // spinners
		"...",
		"Running",
		"Executing",
		"Writing",
		"Reading",
		"Searching",
		"Thinking",
	}

	for _, pattern := range workingPatterns {
		if strings.Contains(lastLine, pattern) {
			return StateWorking
		}
	}

	// Default: assume working if we can't tell
	return StateWorking
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
