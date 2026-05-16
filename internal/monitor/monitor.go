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
			snap.State = *state
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
// Delegates to ClaudeCodeDriver.DetectState so the pattern logic lives in one
// place. When new drivers are added, this fallback stays Claude-specific —
// agents with a registered driver never reach here (see Check).
func detectState(lastLine, fullContent string) AgentState {
	d := &driver.ClaudeCodeDriver{}
	stripped := stripANSI(fullContent)
	allLines := strings.Split(stripped, "\n")
	bottomLines := getLastNonEmptyLines(allLines, paneBottomLines)
	if state := d.DetectState(bottomLines, stripped); state != nil {
		return *state
	}
	return StateWorking
}

// getLastNonEmptyLines returns the last N non-empty lines in their original
// (forward) order.
func getLastNonEmptyLines(lines []string, n int) []string {
	var result []string
	for i := len(lines) - 1; i >= 0 && len(result) < n; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			result = append(result, lines[i])
		}
	}
	// Reverse so lines are in forward order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
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
