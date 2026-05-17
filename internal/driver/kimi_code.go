package driver

import (
	"fmt"
	"os/exec"
	"strings"
)

// KimiCodeDriver implements Driver for Moonshot's Kimi CLI (kimi).
// See https://www.kimi.com/code/docs/en/ for the CLI surface.
type KimiCodeDriver struct{}

func (d *KimiCodeDriver) Name() string { return "kimi-code" }

func (d *KimiCodeDriver) InteractiveCommand() []string {
	return []string{"kimi"}
}

// PlanCommand uses --quiet (which is --print --output-format text
// --final-message-only) so kimi prints just the final assistant message.
// Plain --print emits a verbose event stream (TurnBegin / ThinkPart /
// TextPart / StatusUpdate / TurnEnd) that's unparseable for our purposes.
// --quiet still appends a "To resume this session: ..." trailer, which the
// caller's JSON extractor (extractJSON in handlers.go) tolerates.
func (d *KimiCodeDriver) PlanCommand(prompt string) ([]byte, error) {
	return exec.Command("kimi", "--quiet", "-p", prompt).CombinedOutput()
}

func (d *KimiCodeDriver) BuildCommand(opts LaunchOpts) string {
	yoloFlag := ""
	if opts.YoloMode {
		yoloFlag = " --yolo"
	}
	return fmt.Sprintf("#!/usr/bin/env bash\nprompt=$(cat %q)\nexec kimi%s -p \"$prompt\"\n", opts.PromptFile, yoloFlag)
}

// DetectState analyzes tmux pane content to determine Kimi agent state.
// Patterns are grounded in real `tmux capture-pane` output from kimi 1.39.0.
// Note: kimi panes can show both an active spinner AND an approval prompt
// simultaneously (a subagent is working while another asks for approval), so
// waiting patterns must be checked BEFORE working patterns.
func (d *KimiCodeDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
	if strings.TrimSpace(fullContent) == "" {
		s := StateStarting
		return &s
	}

	bottomText := strings.Join(bottomLines, "\n")

	// ── WAITING PATTERNS (checked first) ──

	// Kimi's approval modal: "Shell is requesting approval to run command:" /
	// "is requesting approval" covers shell, file-write, and other variants.
	if strings.Contains(bottomText, "requesting approval") {
		s := StateWaiting
		return &s
	}

	// Approval option text inside the modal box.
	if strings.Contains(bottomText, "Approve once") ||
		strings.Contains(bottomText, "Approve for this session") ||
		strings.Contains(bottomText, "Reject, tell the model") {
		s := StateWaiting
		return &s
	}

	// Kimi's input-area divider — appears when the input field is the active
	// element on screen (idle, waiting for the user to type a new task).
	if strings.Contains(bottomText, "── input ──") {
		s := StateWaiting
		return &s
	}

	// Bottom-bar help lines. `ctrl-j: newline` is shown while editing the
	// input; `ctrl-x: toggle mode` / `shift-tab: plan mode` show when idle
	// at the input prompt with no active subagent.
	if strings.Contains(bottomText, "ctrl-j: newline") ||
		strings.Contains(bottomText, "ctrl-x: toggle mode") ||
		strings.Contains(bottomText, "shift-tab: plan mode") {
		s := StateWaiting
		return &s
	}

	if strings.Contains(bottomText, "[Y/n]") || strings.Contains(bottomText, "[y/N]") {
		s := StateWaiting
		return &s
	}

	for _, line := range bottomLines {
		if strings.TrimSpace(line) == "❯" {
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

	// Subagent dispatch: "⠏ Using Agent (Review Go code quality)".
	if strings.Contains(bottomText, "Using Agent (") {
		s := StateWorking
		return &s
	}

	// Plan-mode prompt glyph (📋) appears when kimi is mid-planning.
	if strings.Contains(bottomText, "📋") {
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

	return nil
}

// Hook injection is intentionally a no-op until Kimi publishes a stable hook
// config schema. When that lands, mirror the Claude Code implementation in
// internal/hooks. Tracked: see TODO.md (M8 of TECH_DEBT_PLAN.md).
func (d *KimiCodeDriver) InjectHooks(worktreePath string) error { return nil }
func (d *KimiCodeDriver) RemoveHooks(worktreePath string) error { return nil }

func (d *KimiCodeDriver) CheckAvailable() error {
	if _, err := exec.LookPath("kimi"); err != nil {
		return fmt.Errorf("kimi command not found in PATH (install: see https://www.kimi.com/code/docs/en/)")
	}
	return nil
}
