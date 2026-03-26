package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Manager handles tmux session operations
type Manager struct {
	SessionPrefix string
}

// NewManager creates a new tmux manager
func NewManager(prefix string) *Manager {
	if prefix == "" {
		prefix = "fleet"
	}
	return &Manager{SessionPrefix: prefix}
}

// IsAvailable checks if tmux is installed
func (m *Manager) IsAvailable() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// SessionName returns the full tmux session name for an agent
func (m *Manager) SessionName(agentName string) string {
	return fmt.Sprintf("%s-%s", m.SessionPrefix, agentName)
}

// SessionExists checks if a tmux session exists
func (m *Manager) SessionExists(agentName string) bool {
	sessionName := m.SessionName(agentName)
	cmd := exec.Command("tmux", "has-session", "-t", sessionName)
	err := cmd.Run()
	return err == nil
}

// findFleetTmuxConf locates fleet.tmux.conf relative to the fleet binary
func findFleetTmuxConf() string {
	// Find the binary's directory
	exe, err := os.Executable()
	if err == nil {
		// Resolve symlinks
		exe, err = filepath.EvalSymlinks(exe)
		if err == nil {
			confPath := filepath.Join(filepath.Dir(exe), "fleet.tmux.conf")
			if _, err := os.Stat(confPath); err == nil {
				return confPath
			}
		}
	}

	// Fallback: check common locations
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "code", "fleet-commander", "fleet.tmux.conf"),
		filepath.Join(home, ".config", "fleet", "fleet.tmux.conf"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}

	return ""
}

// CreateSession creates a new tmux session for an agent.
// command is the command and arguments to run in the session.
// If nil/empty, defaults to running "claude".
func (m *Manager) CreateSession(agentName, worktreePath string, command []string, stateFilePath string) error {
	sessionName := m.SessionName(agentName)

	// Check if claude is available when using default command
	if len(command) == 0 {
		if _, err := exec.LookPath("claude"); err != nil {
			return fmt.Errorf("claude command not found in PATH")
		}
	}

	// Build tmux command: new-session -d -s <name> -c <path> <command>
	args := []string{
		"new-session",
		"-d", // detached
		"-s", sessionName,
		"-c", worktreePath,
	}

	// Add environment variables if stateFilePath is provided
	if stateFilePath != "" {
		args = append(args, "-e", fmt.Sprintf("FLEET_AGENT_NAME=%s", agentName))
		args = append(args, "-e", fmt.Sprintf("FLEET_STATE_FILE=%s", stateFilePath))
	}

	if len(command) > 0 {
		args = append(args, command...)
	} else {
		args = append(args, "claude")
	}

	cmd := exec.Command("tmux", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create tmux session: %w", err)
	}

	// Source the fleet tmux config if available
	confPath := findFleetTmuxConf()
	if confPath != "" {
		sourceCmd := exec.Command("tmux", "source-file", confPath)
		_ = sourceCmd.Run() // best-effort, don't fail if config has issues
	}

	return nil
}



// Attach attaches to a tmux session
func (m *Manager) Attach(agentName string) error {
	sessionName := m.SessionName(agentName)
	
	// Check if session exists
	if !m.SessionExists(agentName) {
		return fmt.Errorf("session %s does not exist", sessionName)
	}
	
	// Attach to session
	cmd := exec.Command("tmux", "attach", "-t", sessionName)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	return cmd.Run()
}

// Detach detaches from the current tmux session
func (m *Manager) Detach() error {
	cmd := exec.Command("tmux", "detach")
	return cmd.Run()
}

// KillSession kills a tmux session
func (m *Manager) KillSession(agentName string) error {
	sessionName := m.SessionName(agentName)
	cmd := exec.Command("tmux", "kill-session", "-t", sessionName)
	return cmd.Run()
}

// ListSessions lists all fleet tmux sessions
func (m *Manager) ListSessions() ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.Output()
	if err != nil {
		// No sessions is not an error
		if strings.Contains(err.Error(), "no server running") {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	
	var sessions []string
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" && strings.HasPrefix(line, m.SessionPrefix+"-") {
			sessions = append(sessions, line)
		}
	}
	
	return sessions, nil
}

// SendKeys sends keystrokes to a tmux session
func (m *Manager) SendKeys(agentName string, keys string) error {
	sessionName := m.SessionName(agentName)
	cmd := exec.Command("tmux", "send-keys", "-t", sessionName, keys)
	return cmd.Run()
}

// CapturePane captures the content of a tmux pane
func (m *Manager) CapturePane(agentName string) (string, error) {
	sessionName := m.SessionName(agentName)
	cmd := exec.Command("tmux", "capture-pane", "-t", sessionName, "-p")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to capture pane: %w", err)
	}
	return string(output), nil
}

// GetPID returns the PID of the process running in the tmux session
func (m *Manager) GetPID(agentName string) (int, error) {
	sessionName := m.SessionName(agentName)
	cmd := exec.Command("tmux", "list-panes", "-t", sessionName, "-F", "#{pane_pid}")
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get PID: %w", err)
	}
	
	var pid int
	_, err = fmt.Sscanf(string(output), "%d", &pid)
	if err != nil {
		return 0, fmt.Errorf("failed to parse PID: %w", err)
	}
	
	return pid, nil
}

// IsInsideTmux returns true if currently inside a tmux session
func IsInsideTmux() bool {
	return os.Getenv("TMUX") != ""
}

// SwitchClient switches to a different tmux session (from within tmux)
func (m *Manager) SwitchClient(agentName string) error {
	sessionName := m.SessionName(agentName)
	cmd := exec.Command("tmux", "switch-client", "-t", sessionName)
	return cmd.Run()
}
