package agent

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

// Runner manages agent (Claude Code) processes
type Runner struct {
	WorktreePath string
	AgentName    string
}

// NewRunner creates a new agent runner
func NewRunner(worktreePath, agentName string) *Runner {
	return &Runner{
		WorktreePath: worktreePath,
		AgentName:    agentName,
	}
}

// Start starts a Claude Code session in the worktree
func (r *Runner) Start() (*os.Process, error) {
	// Check if claude is available
	if _, err := exec.LookPath("claude"); err != nil {
		return nil, fmt.Errorf("claude command not found in PATH")
	}
	
	// Start claude code in the worktree
	cmd := exec.Command("claude", "code")
	cmd.Dir = r.WorktreePath
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	
	// Start process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}
	
	return cmd.Process, nil
}

// StartDetached starts Claude Code detached (for background agents)
func (r *Runner) StartDetached() (int, error) {
	if _, err := exec.LookPath("claude"); err != nil {
		return 0, fmt.Errorf("claude command not found in PATH")
	}
	
	cmd := exec.Command("claude", "code")
	cmd.Dir = r.WorktreePath
	
	// Detach from parent
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start claude: %w", err)
	}
	
	return cmd.Process.Pid, nil
}

// IsRunning checks if a process is still running
func IsRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	
	// On Unix, FindProcess always succeeds, so we need to send signal 0
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// Kill terminates a running agent
func Kill(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}
	
	return process.Kill()
}
