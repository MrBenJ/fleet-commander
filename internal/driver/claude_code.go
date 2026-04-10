package driver

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/MrBenJ/fleet-commander/internal/hooks"
)

// ClaudeCodeDriver implements Driver for Claude Code.
type ClaudeCodeDriver struct{}

func (d *ClaudeCodeDriver) Name() string {
	return "claude-code"
}

func (d *ClaudeCodeDriver) BuildCommand(opts LaunchOpts) string {
	claudeArgs := ""
	if opts.YoloMode {
		claudeArgs = " --dangerously-skip-permissions"
	}
	return fmt.Sprintf("#!/usr/bin/env bash\nprompt=$(cat %q)\nexec claude%s -- \"$prompt\"\n", opts.PromptFile, claudeArgs)
}

func (d *ClaudeCodeDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
	// Empty content = probably starting up
	if strings.TrimSpace(fullContent) == "" {
		s := StateStarting
		return &s
	}

	bottomText := strings.Join(bottomLines, "\n")

	// ── WAITING PATTERNS (checked first) ──

	if strings.Contains(bottomText, "Esc to cancel") {
		s := StateWaiting
		return &s
	}

	if strings.Contains(bottomText, "Do you want to proceed") {
		s := StateWaiting
		return &s
	}

	if strings.Contains(bottomText, "accept edits") {
		s := StateWaiting
		return &s
	}

	if strings.Contains(bottomText, "shift+tab to cycle") {
		s := StateWaiting
		return &s
	}

	if strings.Contains(bottomText, "❯") && strings.Contains(bottomText, "1.") && strings.Contains(bottomText, "2.") {
		s := StateWaiting
		return &s
	}

	for _, line := range bottomLines {
		if strings.TrimSpace(line) == "❯" {
			s := StateWaiting
			return &s
		}
	}

	// Question heuristic and y/n prompts in the last 3 lines
	veryBottom := lastN(bottomLines, 3)
	for _, line := range veryBottom {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, "?") && len(trimmed) > 10 {
			s := StateWaiting
			return &s
		}
		if strings.Contains(trimmed, "(y/n)") || strings.Contains(trimmed, "[Y/n]") {
			s := StateWaiting
			return &s
		}
	}

	// ── WORKING PATTERNS ──

	if strings.Contains(bottomText, "esc to interrupt") {
		s := StateWorking
		return &s
	}

	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for _, sp := range spinners {
		if strings.Contains(bottomText, sp) {
			s := StateWorking
			return &s
		}
	}

	// No pattern matched — default to working
	s := StateWorking
	return &s
}

func (d *ClaudeCodeDriver) InjectHooks(worktreePath string) error {
	return hooks.Inject(worktreePath)
}

func (d *ClaudeCodeDriver) RemoveHooks(worktreePath string) error {
	return hooks.Remove(worktreePath)
}

func (d *ClaudeCodeDriver) CheckAvailable() error {
	_, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH: %w", err)
	}
	return nil
}

// lastN returns the last n elements of a slice.
func lastN(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
