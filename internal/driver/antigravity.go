package driver

import (
	"fmt"
	"os/exec"
	"strings"
)

// AntigravityDriver implements Driver for Google's Antigravity CLI (agy).
// See https://antigravity.google/docs for the CLI surface.
//
// Antigravity mirrors the Codex driver shape: a CLI-backed agent with no
// Claude-Code-style hook injection, so state is determined by pane scraping.
type AntigravityDriver struct{}

func (d *AntigravityDriver) Name() string { return "antigravity" }

func (d *AntigravityDriver) InteractiveCommand() []string {
	return []string{"agy"}
}

// PlanCommand runs agy headlessly with -p, which emits the model's answer as
// plain text — directly consumable by the caller's JSON extractor. We avoid
// --output-format json, which would wrap the answer in an envelope the
// extractor does not expect.
func (d *AntigravityDriver) PlanCommand(prompt string) ([]byte, error) {
	return exec.Command("agy", "-p", prompt).CombinedOutput()
}

// BuildCommand seeds an interactive agy session with the prompt so the user can
// watch and intervene in the tmux pane.
//
// YoloMode is an INTENTIONAL no-op, not an oversight: as of 2026-05, agy exposes
// no permission-bypass flag. This was verified against the official Antigravity
// CLI docs and confirmed by an open feature request on Google's AI Developers
// Forum ("True YOLO Mode — Bypass ALL Accept/Reject Confirmations"), where Google
// staff treat the capability as pending, not shipped. A Claude-Code-style
// `--dangerously-skip-permissions` does NOT exist for agy; passing one would make
// agy reject the unknown flag and fail to launch. agy's only autonomy lever is
// the in-TUI /goal slash command, which cannot be passed as a launch argument.
// So squadron "yolo" runs fall back to agy's human-in-the-loop approval prompts,
// which the monitor surfaces as "needs input" (the hangar warns about this before
// launch). If Antigravity ships a real bypass flag, wire it in here.
func (d *AntigravityDriver) BuildCommand(opts LaunchOpts) string {
	return fmt.Sprintf("#!/usr/bin/env bash\nprompt=$(cat %q)\nexec agy \"$prompt\"\n", opts.PromptFile)
}

// DetectState analyzes tmux pane content to determine the agy agent state.
//
// NOTE: These patterns are based on Antigravity's documented TUI and are not
// yet empirically tuned. Run agy in a tmux session and inspect output via
// `tmux capture-pane` to discover and refine patterns.
func (d *AntigravityDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
	if strings.TrimSpace(fullContent) == "" {
		s := StateStarting
		return &s
	}

	bottomText := strings.Join(bottomLines, "\n")

	// ── WAITING PATTERNS (checked first) ──

	if strings.Contains(bottomText, "[Y/n]") || strings.Contains(bottomText, "[y/N]") {
		s := StateWaiting
		return &s
	}
	if strings.Contains(bottomText, "(y/n)") {
		s := StateWaiting
		return &s
	}
	if strings.Contains(bottomText, "requesting approval") || strings.Contains(bottomText, "Approve") {
		s := StateWaiting
		return &s
	}
	for _, line := range bottomLines {
		if strings.TrimSpace(line) == ">" {
			s := StateWaiting
			return &s
		}
	}
	veryBottom := lastN(bottomLines, 3)
	for _, line := range veryBottom {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, "?") && len(trimmed) > 10 {
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

	// No pattern matched — return nil to let the caller fall back.
	return nil
}

// InjectHooks is a no-op for Antigravity (no Claude-style hook system).
func (d *AntigravityDriver) InjectHooks(worktreePath string) error { return nil }

// RemoveHooks is a no-op for Antigravity (no Claude-style hook system).
func (d *AntigravityDriver) RemoveHooks(worktreePath string) error { return nil }

func (d *AntigravityDriver) CheckAvailable() error {
	if _, err := exec.LookPath("agy"); err != nil {
		return fmt.Errorf("agy command not found in PATH (install: curl -fsSL https://antigravity.google/cli/install.sh | bash)")
	}
	return nil
}
