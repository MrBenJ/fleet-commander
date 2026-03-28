package monitor

import (
	"strings"
	"time"

	"github.com/teknal/fleet-commander/internal/state"
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

const stateFileTTL = 10 * time.Minute

// CheckWithStateFile checks agent state, preferring the state file over
// tmux pane scraping. Falls back to tmux scraping if the file is absent or stale.
// If m.tmux is nil, returns StateStopped rather than panicking (used in tests).
func (m *Monitor) CheckWithStateFile(agentName, stateFilePath string) *Snapshot {
	if stateFilePath != "" {
		if s, err := state.Read(stateFilePath); err == nil && !s.IsStale(stateFileTTL) {
			snap := &Snapshot{
				AgentName: agentName,
				State:     stateFromString(s.State),
				Timestamp: s.UpdatedAt,
			}
			m.snapshots[agentName] = snap
			return snap
		}
	}
	if m.tmux == nil {
		return &Snapshot{AgentName: agentName, State: StateStopped, Timestamp: time.Now()}
	}
	return m.Check(agentName)
}

func stateFromString(s string) AgentState {
	switch s {
	case "waiting":
		return StateWaiting
	case "working":
		return StateWorking
	default:
		return StateStopped
	}
}

// GetSnapshot returns the last snapshot for an agent
func (m *Monitor) GetSnapshot(agentName string) *Snapshot {
	return m.snapshots[agentName]
}

// detectState analyzes terminal content to determine agent state.
//
// Detection order matters: waiting patterns are checked BEFORE working patterns
// because Claude Code's persistent status bar (which contains "esc to interrupt")
// stays visible even when the agent is idle at an input prompt.
func detectState(lastLine, fullContent string) AgentState {
	lastLine = strings.TrimSpace(lastLine)
	stripped := stripANSI(fullContent)

	// Empty content = probably starting up
	if strings.TrimSpace(stripped) == "" {
		return StateStarting
	}

	// Only look at the BOTTOM of the pane (last 15 non-empty lines)
	// Old prompts may still be visible higher up — ignore them
	allLines := strings.Split(stripped, "\n")
	bottom := getLastNonEmptyLines(allLines, 15)
	bottomText := strings.Join(bottom, "\n")

	// WAITING PATTERNS — check these FIRST because Claude Code's status bar
	// contains "esc to interrupt" even when the agent is at an input prompt.

	// Claude Code permission prompts ("Esc to cancel" footer)
	if strings.Contains(bottomText, "Esc to cancel") {
		return StateWaiting
	}

	// Claude Code tool confirmation
	if strings.Contains(bottomText, "Do you want to proceed") {
		return StateWaiting
	}

	// Claude Code edit acceptance prompt
	if strings.Contains(bottomText, "accept edits") {
		return StateWaiting
	}

	// Claude Code input hint bar
	if strings.Contains(bottomText, "shift+tab to cycle") {
		return StateWaiting
	}

	// Claude Code numbered choice menus
	if strings.Contains(bottomText, "❯") && strings.Contains(bottomText, "1.") && strings.Contains(bottomText, "2.") {
		return StateWaiting
	}

	// Claude Code bare input prompt: ❯ on its own line near the bottom
	for _, line := range bottom {
		if strings.TrimSpace(line) == "❯" {
			return StateWaiting
		}
	}

	// Question at the very bottom (last 3 lines only)
	veryBottom := getLastNonEmptyLines(allLines, 3)
	for _, line := range veryBottom {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, "?") && len(trimmed) > 10 {
			return StateWaiting
		}
		if strings.Contains(trimmed, "(y/n)") || strings.Contains(trimmed, "[Y/n]") {
			return StateWaiting
		}
	}

	// WORKING PATTERNS — only checked after ruling out waiting states

	// Claude Code shows "esc to interrupt" when actively working
	// (but NOT when paired with a waiting indicator above)
	if strings.Contains(bottomText, "esc to interrupt") {
		return StateWorking
	}

	// Spinner characters (may or may not be captured by tmux)
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for _, s := range spinners {
		if strings.Contains(bottomText, s) {
			return StateWorking
		}
	}

	// Default: working
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
