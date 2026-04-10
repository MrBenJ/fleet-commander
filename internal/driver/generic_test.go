package driver

import (
	"regexp"
	"strings"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/fleet"
)

func TestGenericBuildCommand_Positional(t *testing.T) {
	d := NewGenericDriver(GenericConfig{
		Command: "my-agent",
		Args:    []string{"--no-gui"},
	})
	script := d.BuildCommand(LaunchOpts{PromptFile: "/tmp/prompt.txt"})

	if !strings.Contains(script, "#!/usr/bin/env bash") {
		t.Error("missing shebang")
	}
	if !strings.Contains(script, `prompt=$(cat "/tmp/prompt.txt")`) {
		t.Errorf("expected prompt read from file, got:\n%s", script)
	}
	if !strings.Contains(script, `exec my-agent --no-gui "$prompt"`) {
		t.Errorf("expected positional prompt arg, got:\n%s", script)
	}
}

func TestGenericBuildCommand_WithFlag(t *testing.T) {
	d := NewGenericDriver(GenericConfig{
		Command:    "cursor-cli",
		Args:       []string{"--no-gui"},
		PromptFlag: "--message",
	})
	script := d.BuildCommand(LaunchOpts{PromptFile: "/tmp/prompt.txt"})

	if !strings.Contains(script, `exec cursor-cli --no-gui --message "$prompt"`) {
		t.Errorf("expected prompt via --message flag, got:\n%s", script)
	}
}

func TestGenericBuildCommand_FromFile(t *testing.T) {
	d := NewGenericDriver(GenericConfig{
		Command:        "file-agent",
		PromptFromFile: true,
	})
	script := d.BuildCommand(LaunchOpts{PromptFile: "/tmp/prompt.txt"})

	// Should NOT read into a variable
	if strings.Contains(script, "prompt=$(cat") {
		t.Errorf("should not read prompt into variable when PromptFromFile is true, got:\n%s", script)
	}
	if !strings.Contains(script, "/tmp/prompt.txt") {
		t.Errorf("expected prompt file path in output, got:\n%s", script)
	}
}

func TestGenericBuildCommand_FromFileWithFlag(t *testing.T) {
	d := NewGenericDriver(GenericConfig{
		Command:        "file-agent",
		PromptFlag:     "--file",
		PromptFromFile: true,
	})
	script := d.BuildCommand(LaunchOpts{PromptFile: "/tmp/prompt.txt"})

	if !strings.Contains(script, "--file") {
		t.Errorf("expected --file flag, got:\n%s", script)
	}
	if !strings.Contains(script, "/tmp/prompt.txt") {
		t.Errorf("expected prompt file path, got:\n%s", script)
	}
}

func TestGenericBuildCommand_YoloArgs(t *testing.T) {
	d := NewGenericDriver(GenericConfig{
		Command:  "my-agent",
		YoloArgs: []string{"--auto-approve", "--no-confirm"},
	})

	// Without yolo
	script := d.BuildCommand(LaunchOpts{PromptFile: "/tmp/prompt.txt"})
	if strings.Contains(script, "--auto-approve") {
		t.Error("yolo args should not appear when YoloMode is false")
	}

	// With yolo
	script = d.BuildCommand(LaunchOpts{PromptFile: "/tmp/prompt.txt", YoloMode: true})
	if !strings.Contains(script, "--auto-approve") || !strings.Contains(script, "--no-confirm") {
		t.Errorf("expected yolo args, got:\n%s", script)
	}
}

func TestGenericDetectState_WaitingPatterns(t *testing.T) {
	d := NewGenericDriver(GenericConfig{
		Command:         "test",
		WaitingPatterns: []*regexp.Regexp{regexp.MustCompile(`\[Y/n\]`), regexp.MustCompile(`\$\s*$`)},
	})

	tests := []struct {
		name  string
		lines []string
		want  AgentState
	}{
		{"yn prompt", []string{"Continue? [Y/n]"}, StateWaiting},
		{"shell prompt", []string{"user@host:~$ "}, StateWaiting},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := d.DetectState(tt.lines, "")
			if state == nil {
				t.Fatal("expected non-nil state")
			}
			if *state != tt.want {
				t.Errorf("got %q, want %q", *state, tt.want)
			}
		})
	}
}

func TestGenericDetectState_WorkingPatterns(t *testing.T) {
	d := NewGenericDriver(GenericConfig{
		Command:         "test",
		WorkingPatterns: []*regexp.Regexp{regexp.MustCompile(`Running\.\.\.`), regexp.MustCompile(`Processing`)},
	})

	state := d.DetectState([]string{"Running..."}, "")
	if state == nil || *state != StateWorking {
		t.Errorf("expected working, got %v", state)
	}

	state = d.DetectState([]string{"Processing data"}, "")
	if state == nil || *state != StateWorking {
		t.Errorf("expected working, got %v", state)
	}
}

func TestGenericDetectState_WaitingTakesPriority(t *testing.T) {
	d := NewGenericDriver(GenericConfig{
		Command:         "test",
		WaitingPatterns: []*regexp.Regexp{regexp.MustCompile(`\?`)},
		WorkingPatterns: []*regexp.Regexp{regexp.MustCompile(`.*`)},
	})

	state := d.DetectState([]string{"Continue?"}, "")
	if state == nil || *state != StateWaiting {
		t.Errorf("waiting should take priority, got %v", state)
	}
}

func TestGenericDetectState_NoPatterns(t *testing.T) {
	d := NewGenericDriver(GenericConfig{Command: "test"})

	state := d.DetectState([]string{"anything here"}, "full content")
	if state != nil {
		t.Errorf("expected nil when no patterns configured, got %v", *state)
	}
}

func TestGenericDetectState_NoMatch(t *testing.T) {
	d := NewGenericDriver(GenericConfig{
		Command:         "test",
		WaitingPatterns: []*regexp.Regexp{regexp.MustCompile(`SPECIFIC_PATTERN`)},
	})

	state := d.DetectState([]string{"nothing matching here"}, "")
	if state != nil {
		t.Errorf("expected nil when no pattern matches, got %v", *state)
	}
}

func TestParseGenericConfig(t *testing.T) {
	dc := &fleet.DriverConfig{
		Command:         "my-agent",
		Args:            []string{"--flag"},
		YoloArgs:        []string{"--yolo"},
		PromptFlag:      "--message",
		PromptFromFile:  true,
		WaitingPatterns: []string{`\[Y/n\]`, `\$\s*$`},
		WorkingPatterns: []string{`Running\.\.\.`},
	}

	config, err := ParseGenericConfig(dc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Command != "my-agent" {
		t.Errorf("command: got %q, want %q", config.Command, "my-agent")
	}
	if len(config.WaitingPatterns) != 2 {
		t.Errorf("waiting patterns: got %d, want 2", len(config.WaitingPatterns))
	}
	if len(config.WorkingPatterns) != 1 {
		t.Errorf("working patterns: got %d, want 1", len(config.WorkingPatterns))
	}
	if !config.PromptFromFile {
		t.Error("PromptFromFile should be true")
	}
}

func TestParseGenericConfig_InvalidRegex(t *testing.T) {
	dc := &fleet.DriverConfig{
		Command:         "test",
		WaitingPatterns: []string{`[invalid`},
	}

	_, err := ParseGenericConfig(dc)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !strings.Contains(err.Error(), "invalid waiting pattern") {
		t.Errorf("error should mention invalid waiting pattern, got: %v", err)
	}
}

func TestParseGenericConfig_InvalidWorkingRegex(t *testing.T) {
	dc := &fleet.DriverConfig{
		Command:         "test",
		WorkingPatterns: []string{`[bad`},
	}

	_, err := ParseGenericConfig(dc)
	if err == nil {
		t.Fatal("expected error for invalid regex")
	}
	if !strings.Contains(err.Error(), "invalid working pattern") {
		t.Errorf("error should mention invalid working pattern, got: %v", err)
	}
}

func TestGenericCheckAvailable_NoCommand(t *testing.T) {
	d := NewGenericDriver(GenericConfig{})

	err := d.CheckAvailable()
	if err == nil {
		t.Fatal("expected error when command is empty")
	}
	if !strings.Contains(err.Error(), "requires a 'command'") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenericCheckAvailable_NotFound(t *testing.T) {
	d := NewGenericDriver(GenericConfig{Command: "definitely-not-a-real-command-12345"})

	err := d.CheckAvailable()
	if err == nil {
		t.Fatal("expected error when command not found")
	}
	if !strings.Contains(err.Error(), "not found in PATH") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestGenericCheckAvailable_Found(t *testing.T) {
	// "go" should be available in the test environment
	d := NewGenericDriver(GenericConfig{Command: "go"})

	if err := d.CheckAvailable(); err != nil {
		t.Errorf("expected no error for 'go' command: %v", err)
	}
}

func TestGenericName(t *testing.T) {
	d := NewGenericDriver(GenericConfig{})
	if d.Name() != "generic" {
		t.Errorf("Name() = %q, want %q", d.Name(), "generic")
	}
}

func TestGenericInteractiveCommand(t *testing.T) {
	d := NewGenericDriver(GenericConfig{
		Command: "my-agent",
		Args:    []string{"--interactive", "--no-gui"},
	})

	cmd := d.InteractiveCommand()
	if len(cmd) != 3 || cmd[0] != "my-agent" || cmd[1] != "--interactive" || cmd[2] != "--no-gui" {
		t.Errorf("InteractiveCommand() = %v, want [my-agent --interactive --no-gui]", cmd)
	}
}

func TestGenericPlanCommand(t *testing.T) {
	d := NewGenericDriver(GenericConfig{Command: "test"})
	_, err := d.PlanCommand("test prompt")
	if err == nil {
		t.Fatal("expected error from PlanCommand")
	}
}

func TestGenericHooksNoOp(t *testing.T) {
	d := NewGenericDriver(GenericConfig{Command: "test"})

	if err := d.InjectHooks("/tmp/test"); err != nil {
		t.Errorf("InjectHooks should be no-op, got: %v", err)
	}
	if err := d.RemoveHooks("/tmp/test"); err != nil {
		t.Errorf("RemoveHooks should be no-op, got: %v", err)
	}
}
