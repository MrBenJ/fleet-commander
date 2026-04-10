package tmux

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// safeAgentName validates that an agent name contains only safe characters
// before it is used in tmux commands. This is defense-in-depth — the fleet
// layer also validates, but the tmux layer must not trust its callers.
var safeAgentName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

func validateAgentName(name string) error {
	if !safeAgentName.MatchString(name) {
		return fmt.Errorf("unsafe agent name %q: must be alphanumeric with hyphens/underscores", name)
	}
	return nil
}

// CommandRunner abstracts shell command execution so Manager can be tested
// without real tmux.
type CommandRunner interface {
	Run(name string, args ...string) error
	Output(name string, args ...string) ([]byte, error)
	RunInteractive(name string, args ...string) error
	LookPath(name string) (string, error)
}

type execRunner struct{}

func (e *execRunner) Run(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func (e *execRunner) Output(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

func (e *execRunner) RunInteractive(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (e *execRunner) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

// Manager handles tmux session operations
type Manager struct {
	SessionPrefix string
	runner        CommandRunner
}

// NewManager creates a new tmux manager
func NewManager(prefix string) *Manager {
	if prefix == "" {
		prefix = "fleet"
	}
	return &Manager{SessionPrefix: prefix, runner: &execRunner{}}
}

// NewManagerWithRunner creates a new tmux manager with a custom CommandRunner
func NewManagerWithRunner(prefix string, runner CommandRunner) *Manager {
	if prefix == "" {
		prefix = "fleet"
	}
	return &Manager{SessionPrefix: prefix, runner: runner}
}

// IsAvailable checks if tmux is installed
func (m *Manager) IsAvailable() bool {
	_, err := m.runner.LookPath("tmux")
	return err == nil
}

// SessionName returns the full tmux session name for an agent
func (m *Manager) SessionName(agentName string) string {
	return fmt.Sprintf("%s-%s", m.SessionPrefix, agentName)
}

// SessionExists checks if a tmux session exists
func (m *Manager) SessionExists(agentName string) bool {
	if validateAgentName(agentName) != nil {
		return false
	}
	sessionName := m.SessionName(agentName)
	err := m.runner.Run("tmux", "has-session", "-t", sessionName)
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
// If nil/empty, tmux starts the user's default shell.
func (m *Manager) CreateSession(agentName, worktreePath string, command []string, stateFilePath string) error {
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	sessionName := m.SessionName(agentName)

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
	}
	// When no command is given, tmux starts the user's default shell

	// Build redacted args for error messages (redact long prompt args)
	debugArgs := make([]string, len(args))
	copy(debugArgs, args)
	for i, a := range debugArgs {
		if len(a) > 200 {
			debugArgs[i] = fmt.Sprintf("[%d bytes]", len(a))
		}
	}

	if err := m.runner.Run("tmux", args...); err != nil {
		return fmt.Errorf("failed to create tmux session (args=%v): %w", debugArgs, err)
	}

	// Source the fleet tmux config if available
	confPath := findFleetTmuxConf()
	if confPath != "" {
		_ = m.runner.Run("tmux", "source-file", confPath) // best-effort, don't fail if config has issues
	}

	return nil
}



// Attach attaches to a tmux session
func (m *Manager) Attach(agentName string) error {
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	sessionName := m.SessionName(agentName)
	
	// Check if session exists
	if !m.SessionExists(agentName) {
		return fmt.Errorf("session %s does not exist", sessionName)
	}
	
	// Attach to session
	return m.runner.RunInteractive("tmux", "attach", "-t", sessionName)
}

// Detach detaches from the current tmux session
func (m *Manager) Detach() error {
	return m.runner.Run("tmux", "detach")
}

// KillSession kills a tmux session
func (m *Manager) KillSession(agentName string) error {
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	sessionName := m.SessionName(agentName)
	return m.runner.Run("tmux", "kill-session", "-t", sessionName)
}

// ListSessions lists all fleet tmux sessions
func (m *Manager) ListSessions() ([]string, error) {
	output, err := m.runner.Output("tmux", "list-sessions", "-F", "#{session_name}")
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
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	sessionName := m.SessionName(agentName)
	return m.runner.Run("tmux", "send-keys", "-t", sessionName, keys)
}

// CapturePane captures the content of a tmux pane
func (m *Manager) CapturePane(agentName string) (string, error) {
	if err := validateAgentName(agentName); err != nil {
		return "", err
	}
	sessionName := m.SessionName(agentName)
	output, err := m.runner.Output("tmux", "capture-pane", "-t", sessionName, "-p")
	if err != nil {
		return "", fmt.Errorf("failed to capture pane: %w", err)
	}
	return string(output), nil
}

// GetPID returns the PID of the process running in the tmux session
func (m *Manager) GetPID(agentName string) (int, error) {
	if err := validateAgentName(agentName); err != nil {
		return 0, err
	}
	sessionName := m.SessionName(agentName)
	output, err := m.runner.Output("tmux", "list-panes", "-t", sessionName, "-F", "#{pane_pid}")
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
	if err := validateAgentName(agentName); err != nil {
		return err
	}
	sessionName := m.SessionName(agentName)
	return m.runner.Run("tmux", "switch-client", "-t", sessionName)
}
