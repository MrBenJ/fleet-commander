package driver

import (
	"fmt"
	"os/exec"
	"strings"
)

// CodexDriver implements Driver for OpenAI's Codex CLI.
type CodexDriver struct{}

func (d *CodexDriver) Name() string { return "codex" }

func (d *CodexDriver) InteractiveCommand() []string {
	return []string{"codex"}
}

func (d *CodexDriver) PlanCommand(prompt string) ([]byte, error) {
	return exec.Command("codex", "exec", "--color", "never", prompt).CombinedOutput()
}

func (d *CodexDriver) BuildCommand(opts LaunchOpts) string {
	args := "--ask-for-approval on-request --sandbox workspace-write"
	if opts.YoloMode {
		args = "--dangerously-bypass-approvals-and-sandbox"
	}
	return fmt.Sprintf("#!/usr/bin/env bash\nprompt=$(cat %q)\nexec codex %s \"$prompt\"\n", opts.PromptFile, args)
}

// DetectState analyzes tmux pane content to determine Codex agent state.
//
// NOTE: These patterns are based on observed Codex CLI output and may need
// empirical tuning as Codex evolves. Run codex in a tmux session and inspect
// output via `tmux capture-pane` to discover new patterns.
func (d *CodexDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
	if strings.TrimSpace(fullContent) == "" {
		s := StateStarting
		return &s
	}

	bottomText := strings.Join(bottomLines, "\n")

	// ── WAITING PATTERNS (checked first) ──

	// Confirmation prompts
	if strings.Contains(bottomText, "[Y/n]") || strings.Contains(bottomText, "[y/N]") {
		s := StateWaiting
		return &s
	}

	// File change acceptance
	if strings.Contains(bottomText, "Accept?") {
		s := StateWaiting
		return &s
	}

	// Sandbox shell prompt
	if strings.Contains(bottomText, "sandbox$") {
		s := StateWaiting
		return &s
	}

	// Input prompt: > on its own line
	for _, line := range bottomLines {
		if strings.TrimSpace(line) == ">" {
			s := StateWaiting
			return &s
		}
	}

	// Question heuristic: lines ending with ? (len > 10) in last 3 lines
	veryBottom := lastN(bottomLines, 3)
	for _, line := range veryBottom {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, "?") && len(trimmed) > 10 {
			s := StateWaiting
			return &s
		}
	}

	// ── WORKING PATTERNS ──

	// Status lines
	for _, pattern := range []string{"Running...", "Executing...", "Reading", "Writing", "Searching"} {
		if strings.Contains(bottomText, pattern) {
			s := StateWorking
			return &s
		}
	}

	// Spinner characters
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for _, sp := range spinners {
		if strings.Contains(bottomText, sp) {
			s := StateWorking
			return &s
		}
	}

	// No pattern matched — return nil to let the caller fall back
	return nil
}

// InjectHooks is a no-op for Codex (no hook system).
func (d *CodexDriver) InjectHooks(worktreePath string) error { return nil }

// RemoveHooks is a no-op for Codex (no hook system).
func (d *CodexDriver) RemoveHooks(worktreePath string) error { return nil }

func (d *CodexDriver) CheckAvailable() error {
	if _, err := exec.LookPath("codex"); err != nil {
		return fmt.Errorf("codex command not found in PATH (install: npm i -g @openai/codex)")
	}
	return nil
}
