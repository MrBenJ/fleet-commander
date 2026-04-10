package driver

import (
	"os/exec"
	"strings"
	"testing"
)

func TestAiderBuildCommand(t *testing.T) {
	d := &AiderDriver{}

	t.Run("without yolo mode", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{
			PromptFile: "/tmp/prompt.txt",
			AgentName:  "test",
		})
		if !strings.Contains(script, "--no-auto-commits") {
			t.Error("expected --no-auto-commits flag")
		}
		if strings.Contains(script, "--yes") {
			t.Error("should not contain --yes without yolo mode")
		}
		if !strings.Contains(script, "--message") {
			t.Error("expected --message flag")
		}
		if !strings.Contains(script, "#!/usr/bin/env bash") {
			t.Error("expected shebang")
		}
		if !strings.Contains(script, "exec aider") {
			t.Error("expected exec aider")
		}
	})

	t.Run("with yolo mode", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{
			PromptFile: "/tmp/prompt.txt",
			YoloMode:   true,
		})
		if !strings.Contains(script, "--no-auto-commits") {
			t.Error("expected --no-auto-commits flag")
		}
		if !strings.Contains(script, "--yes") {
			t.Error("expected --yes flag in yolo mode")
		}
	})
}

func TestAiderBuildCommand_PromptFileEscaping(t *testing.T) {
	d := &AiderDriver{}

	tests := []struct {
		name string
		path string
	}{
		{"spaces", "/tmp/my prompts/file.txt"},
		{"special chars", "/tmp/prompt's \"file\".txt"},
		{"backticks", "/tmp/`prompt`.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := d.BuildCommand(LaunchOpts{PromptFile: tt.path})
			// The %q formatting should properly quote the path
			if !strings.Contains(script, "cat ") {
				t.Error("expected cat command for prompt file")
			}
		})
	}
}

func TestAiderDetectState_Waiting(t *testing.T) {
	d := &AiderDriver{}

	tests := []struct {
		name        string
		bottomLines []string
	}{
		{"aider prompt", []string{"aider>"}},
		{"aider prompt with space", []string{"  aider>  "}},
		{"aider prompt with text", []string{"aider> some text"}},
		{"add to chat", []string{"Add src/main.go to the chat?"}},
		{"Y/n prompt", []string{"Do you want to continue? [Y/n]"}},
		{"y/n prompt", []string{"Apply changes? [y/n]"}},
		{"run shell command", []string{"Run shell command? ls -la"}},
		{"apply edit", []string{"Apply edit?"}},
		{"edit file prompt", []string{"Edit src/main.go ?"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := d.DetectState(tt.bottomLines, strings.Join(tt.bottomLines, "\n"))
			if state == nil {
				t.Fatal("expected non-nil state")
			}
			if *state != StateWaiting {
				t.Errorf("expected StateWaiting, got %s", *state)
			}
		})
	}
}

func TestAiderDetectState_Working(t *testing.T) {
	d := &AiderDriver{}

	tests := []struct {
		name        string
		bottomLines []string
	}{
		{"tokens line", []string{"Tokens: 1,234 sent, 567 received"}},
		{"editing status", []string{"Editing src/main.go"}},
		{"committing status", []string{"Committing changes..."}},
		{"applying edit", []string{"Applying edit to src/main.go"}},
		{"spinner", []string{"⠋ Working..."}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := d.DetectState(tt.bottomLines, strings.Join(tt.bottomLines, "\n"))
			if state == nil {
				t.Fatal("expected non-nil state")
			}
			if *state != StateWorking {
				t.Errorf("expected StateWorking, got %s", *state)
			}
		})
	}
}

func TestAiderDetectState_Unknown(t *testing.T) {
	d := &AiderDriver{}

	tests := []struct {
		name        string
		bottomLines []string
	}{
		{"empty", []string{}},
		{"random text", []string{"hello world", "some output"}},
		{"blank lines", []string{"", "  ", ""}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := d.DetectState(tt.bottomLines, strings.Join(tt.bottomLines, "\n"))
			if state != nil {
				t.Errorf("expected nil state for unknown content, got %s", *state)
			}
		})
	}
}

func TestAiderCheckAvailable(t *testing.T) {
	d := &AiderDriver{}

	// We can't control whether aider is installed in the test environment,
	// so just verify the method doesn't panic and returns a sensible result.
	err := d.CheckAvailable()
	if err != nil {
		// aider not installed — verify error message is helpful
		if !strings.Contains(err.Error(), "pip install aider-chat") {
			t.Errorf("error should contain install hint, got: %s", err.Error())
		}
	} else {
		// aider is installed — verify it's actually in PATH
		if _, lookErr := exec.LookPath("aider"); lookErr != nil {
			t.Error("CheckAvailable returned nil but aider not in PATH")
		}
	}
}

func TestAiderName(t *testing.T) {
	d := &AiderDriver{}
	if d.Name() != "aider" {
		t.Errorf("expected 'aider', got %q", d.Name())
	}
}

func TestAiderHooksAreNoop(t *testing.T) {
	d := &AiderDriver{}
	if err := d.InjectHooks("/tmp/fake"); err != nil {
		t.Errorf("InjectHooks should be no-op, got error: %v", err)
	}
	if err := d.RemoveHooks("/tmp/fake"); err != nil {
		t.Errorf("RemoveHooks should be no-op, got error: %v", err)
	}
}

func TestAiderRegistered(t *testing.T) {
	d, err := Get("aider")
	if err != nil {
		t.Fatalf("aider should be registered: %v", err)
	}
	if d.Name() != "aider" {
		t.Errorf("expected driver name 'aider', got %q", d.Name())
	}
}

func TestAiderInteractiveCommand(t *testing.T) {
	d := &AiderDriver{}
	cmd := d.InteractiveCommand()
	if len(cmd) < 2 {
		t.Fatal("expected at least 2 elements")
	}
	if cmd[0] != "aider" {
		t.Errorf("expected 'aider', got %q", cmd[0])
	}
	found := false
	for _, arg := range cmd {
		if arg == "--no-auto-commits" {
			found = true
		}
	}
	if !found {
		t.Error("expected --no-auto-commits in interactive command")
	}
}
