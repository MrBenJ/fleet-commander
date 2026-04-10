package driver

import (
	"fmt"
	"os/exec"
	"strings"
)

// AiderDriver implements Driver for Aider (aider-chat).
type AiderDriver struct{}

func (d *AiderDriver) Name() string { return "aider" }

func (d *AiderDriver) InteractiveCommand() []string {
	return []string{"aider", "--no-auto-commits"}
}

func (d *AiderDriver) PlanCommand(prompt string) ([]byte, error) {
	return exec.Command("aider", "--no-auto-commits", "--message", prompt).CombinedOutput()
}

func (d *AiderDriver) BuildCommand(opts LaunchOpts) string {
	flags := "--no-auto-commits"
	if opts.YoloMode {
		flags += " --yes"
	}
	return fmt.Sprintf("#!/usr/bin/env bash\nprompt=$(cat %q)\nexec aider %s --message \"$prompt\"\n", opts.PromptFile, flags)
}

func (d *AiderDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
	bottomText := strings.Join(bottomLines, "\n")

	// ── WAITING PATTERNS (checked first) ──

	for _, line := range bottomLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "aider>" || strings.HasPrefix(trimmed, "aider>") {
			s := StateWaiting
			return &s
		}
	}

	if strings.Contains(bottomText, "Add") && strings.Contains(bottomText, "to the chat?") {
		s := StateWaiting
		return &s
	}

	if strings.Contains(bottomText, "[Y/n]") || strings.Contains(bottomText, "[y/n]") {
		s := StateWaiting
		return &s
	}

	if strings.Contains(bottomText, "Run shell command?") {
		s := StateWaiting
		return &s
	}

	if strings.Contains(bottomText, "Apply edit?") {
		s := StateWaiting
		return &s
	}

	// "Edit <file> ?" pattern
	for _, line := range bottomLines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Edit ") && strings.HasSuffix(trimmed, "?") {
			s := StateWaiting
			return &s
		}
	}

	// ── WORKING PATTERNS ──

	if strings.Contains(bottomText, "Tokens:") {
		s := StateWorking
		return &s
	}

	if strings.Contains(bottomText, "Editing") || strings.Contains(bottomText, "Committing") {
		s := StateWorking
		return &s
	}

	if strings.Contains(bottomText, "Applying edit to") {
		s := StateWorking
		return &s
	}

	// Spinner characters (same as Claude Code driver)
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for _, sp := range spinners {
		if strings.Contains(bottomText, sp) {
			s := StateWorking
			return &s
		}
	}

	// NOTE: These patterns may need empirical tuning by observing actual
	// aider output via `tmux capture-pane -p`.
	return nil
}

func (d *AiderDriver) InjectHooks(worktreePath string) error  { return nil }
func (d *AiderDriver) RemoveHooks(worktreePath string) error { return nil }

func (d *AiderDriver) CheckAvailable() error {
	if _, err := exec.LookPath("aider"); err != nil {
		return fmt.Errorf("aider command not found in PATH (install: pip install aider-chat)")
	}
	return nil
}
