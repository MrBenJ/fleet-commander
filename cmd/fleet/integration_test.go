//go:build integration

package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temp git repo and builds the fleet binary.
// Returns the repo path and the path to the compiled fleet binary.
func setupTestRepo(t *testing.T) (repoPath, binaryPath string) {
	t.Helper()

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not in PATH")
	}

	// Create and initialize a temp git repo
	repoPath = t.TempDir()
	run(t, repoPath, "git", "init")
	run(t, repoPath, "git", "config", "user.email", "test@test.com")
	run(t, repoPath, "git", "config", "user.name", "Test")
	run(t, repoPath, "git", "commit", "--allow-empty", "-m", "init")

	// Build the fleet binary — find module root first
	modRoot := findModRoot(t)
	binaryDir := t.TempDir()
	binaryPath = filepath.Join(binaryDir, "fleet")
	run(t, modRoot, "go", "build", "-o", binaryPath, "./cmd/fleet/")

	return
}

// findModRoot walks up from cwd to find go.mod.
func findModRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// run executes a command in dir, failing the test on error.
func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("command %q %v in %q failed: %v\noutput: %s", name, args, dir, err, out)
	}
	return string(out)
}

// runFleet runs the fleet binary with the given args in repoPath.
func runFleet(t *testing.T, binaryPath, repoPath string, args ...string) string {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	cmd.Dir = repoPath
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fleet %v failed: %v\noutput: %s", args, err, out)
	}
	return string(out)
}

func TestInitCreatesConfig(t *testing.T) {
	repoPath, binaryPath := setupTestRepo(t)

	runFleet(t, binaryPath, repoPath, "init", repoPath)

	configPath := filepath.Join(repoPath, ".fleet", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("config.json not created: %v", err)
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("config.json is not valid JSON: %v\ncontent: %s", err, data)
	}

	if config["repo_path"] == "" {
		t.Error("config.json missing repo_path field")
	}
}

func TestAddCreatesWorktreeAndConfig(t *testing.T) {
	repoPath, binaryPath := setupTestRepo(t)

	runFleet(t, binaryPath, repoPath, "init", repoPath)
	runFleet(t, binaryPath, repoPath, "add", "myagent", "feature/my-agent")

	// Worktree directory must exist
	worktreePath := filepath.Join(repoPath, ".fleet", "worktrees", "myagent")
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Errorf("worktree directory %q was not created", worktreePath)
	}

	// Config must contain the agent
	data, _ := os.ReadFile(filepath.Join(repoPath, ".fleet", "config.json"))
	if !strings.Contains(string(data), `"myagent"`) {
		t.Errorf("config.json does not contain agent name 'myagent'\ncontent: %s", data)
	}
	if !strings.Contains(string(data), `"feature/my-agent"`) {
		t.Errorf("config.json does not contain branch 'feature/my-agent'\ncontent: %s", data)
	}
}

func TestListShowsAgents(t *testing.T) {
	repoPath, binaryPath := setupTestRepo(t)

	runFleet(t, binaryPath, repoPath, "init", repoPath)
	runFleet(t, binaryPath, repoPath, "add", "myagent", "feature/my-agent")

	out := runFleet(t, binaryPath, repoPath, "list")

	if !strings.Contains(out, "myagent") {
		t.Errorf("fleet list output does not contain agent name 'myagent'\noutput: %s", out)
	}
	if !strings.Contains(out, "feature/my-agent") {
		t.Errorf("fleet list output does not contain branch 'feature/my-agent'\noutput: %s", out)
	}
}

func TestListAgentListFlag(t *testing.T) {
	repoPath, binaryPath := setupTestRepo(t)

	runFleet(t, binaryPath, repoPath, "init", repoPath)
	runFleet(t, binaryPath, repoPath, "add", "alpha", "feature/alpha")
	runFleet(t, binaryPath, repoPath, "add", "bravo", "feature/bravo")

	out := runFleet(t, binaryPath, repoPath, "list", "--agent-list")

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %q", len(lines), out)
	}
	if lines[0] != "alpha" {
		t.Errorf("expected first line to be 'alpha', got %q", lines[0])
	}
	if lines[1] != "bravo" {
		t.Errorf("expected second line to be 'bravo', got %q", lines[1])
	}

	// Should not contain table headers
	if strings.Contains(out, "AGENT") || strings.Contains(out, "BRANCH") {
		t.Errorf("--agent-list output should not contain table headers\noutput: %s", out)
	}
}

func TestStopCleansUp(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not in PATH")
	}
	if err := exec.Command("tmux", "list-sessions").Run(); err != nil {
		t.Skip("no running tmux server")
	}
	if _, err := exec.LookPath("claude"); err != nil {
		t.Skip("claude not in PATH — fleet start requires Claude Code CLI")
	}

	repoPath, binaryPath := setupTestRepo(t)

	runFleet(t, binaryPath, repoPath, "init", repoPath)
	runFleet(t, binaryPath, repoPath, "add", "myagent", "feature/my-agent")
	runFleet(t, binaryPath, repoPath, "start", "myagent")
	runFleet(t, binaryPath, repoPath, "stop", "myagent")

	// State file must be gone
	stateFile := filepath.Join(repoPath, ".fleet", "states", "myagent.json")
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Errorf("state file %q still exists after stop", stateFile)
	}

	// .claude/settings.json in the worktree must have no _fleet hooks
	settingsPath := filepath.Join(repoPath, ".fleet", "worktrees", "myagent", ".claude", "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if strings.Contains(string(data), `"_fleet"`) {
			t.Errorf("settings.json still contains _fleet hooks after stop\ncontent: %s", data)
		}
	}
}
