package monitor

import (
	"strings"
	"time"

	"github.com/MrBenJ/fleet-commander/internal/driver"
	"github.com/MrBenJ/fleet-commander/internal/state"
	"github.com/MrBenJ/fleet-commander/internal/tmux"
)

// AgentState is an alias for driver.AgentState. The canonical definition
// lives in internal/driver/. This alias keeps existing monitor consumers
// compiling without changing their imports.
type AgentState = driver.AgentState

const (
	StateWorking  = driver.StateWorking
	StateWaiting  = driver.StateWaiting
	StateStopped  = driver.StateStopped
	StateStarting = driver.StateStarting
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
	drivers   map[string]driver.Driver
}

// NewMonitor creates a new agent monitor
func NewMonitor(tm *tmux.Manager) *Monitor {
	return &Monitor{
		tmux:      tm,
		snapshots: make(map[string]*Snapshot),
		drivers:   make(map[string]driver.Driver),
	}
}

// SetDriver registers a driver for agent-specific state detection.
func (m *Monitor) SetDriver(agentName string, drv driver.Driver) {
	m.drivers[agentName] = drv
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

	// Try driver-based detection first
	if drv, ok := m.drivers[agentName]; ok {
		stripped := stripANSI(content)
		allLines := strings.Split(stripped, "\n")
		bottomLines := getLastNonEmptyLines(allLines, paneBottomLines)
		if state := drv.DetectState(bottomLines, stripped); state != nil {
			snap.State = AgentState(*state)
			m.snapshots[agentName] = snap
			return snap
		}
	}

	// Fallback to legacy detectState
	snap.State = detectState(snap.LastLine, content)

	m.snapshots[agentName] = snap
	return snap
}

const stateFileTTL = 10 * time.Minute

// Number of non-empty lines from the bottom of the pane to inspect for state detection.
// Higher values risk matching stale prompts; lower values risk missing the current prompt.
const paneBottomLines = 15

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
// DEPRECATION NOTE: Pane scraping is the legacy fallback for state detection.
// The preferred path is Claude Code hooks writing to state files via
// "fleet signal waiting/working". New detection patterns should go into the
// hooks path, not here. This function will eventually be removed once hooks
// are reliable enough to be the sole detection mechanism.
//
// Detection order matters: waiting patterns are checked BEFORE working patterns
// because Claude Code's persistent status bar (which contains "esc to interrupt")
// stays visible even when the agent is idle at an input prompt. If we checked
// for "esc to interrupt" first, every agent would always appear as working.
func detectState(lastLine, fullContent string) AgentState {
	lastLine = strings.TrimSpace(lastLine)
	stripped := stripANSI(fullContent)

	// Empty content = probably starting up
	if strings.TrimSpace(stripped) == "" {
		return StateStarting
	}

	// Only look at the BOTTOM of the pane to avoid matching stale prompts
	// that scrolled up but are still visible in the terminal buffer.
	allLines := strings.Split(stripped, "\n")
	bottom := getLastNonEmptyLines(allLines, paneBottomLines)
	bottomText := strings.Join(bottom, "\n")

	// ── WAITING PATTERNS (checked first — see note above) ──

	// Permission prompts: Claude asks to run a tool and shows "Esc to cancel"
	// in the footer. Note capital "E" — distinct from status bar's lowercase.
	if strings.Contains(bottomText, "Esc to cancel") {
		return StateWaiting
	}

	// Batch tool confirmation: "Do you want to proceed?" when multiple tools queued.
	if strings.Contains(bottomText, "Do you want to proceed") {
		return StateWaiting
	}

	// File edit review: Claude shows diffs and asks the user to "accept edits".
	if strings.Contains(bottomText, "accept edits") {
		return StateWaiting
	}

	// Input mode hint bar: appears when Claude is at the text input prompt.
	if strings.Contains(bottomText, "shift+tab to cycle") {
		return StateWaiting
	}

	// Numbered choice menu: Claude presents options with ❯ cursor marker.
	if strings.Contains(bottomText, "❯") && strings.Contains(bottomText, "1.") && strings.Contains(bottomText, "2.") {
		return StateWaiting
	}

	// Bare input prompt: just ❯ on its own line — Claude is waiting for user input.
	for _, line := range bottom {
		if strings.TrimSpace(line) == "❯" {
			return StateWaiting
		}
	}

	// Question heuristic: a line ending with "?" in the last 3 lines.
	// Min length 10 to avoid matching single-word false positives.
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

	// ── WORKING PATTERNS (only after ruling out all waiting states) ──

	// Status bar: "esc to interrupt" (lowercase) appears when Claude is actively
	// generating. This is checked AFTER waiting patterns because the status bar
	// persists even when a waiting prompt is shown above it.
	if strings.Contains(bottomText, "esc to interrupt") {
		return StateWorking
	}

	// Braille spinner: animated progress indicator during tool execution.
	// Tmux may or may not capture these depending on timing.
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for _, s := range spinners {
		if strings.Contains(bottomText, s) {
			return StateWorking
		}
	}

	// No pattern matched. Default to working rather than unknown — in practice,
	// unrecognized output is usually Claude producing content that doesn't match
	// any specific pattern. The risk is that a genuinely waiting agent shows as
	// working, but this is mitigated by the state file path (hooks) being the
	// primary detection mechanism.
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
