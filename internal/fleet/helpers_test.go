//go:build !windows

package fleet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateAgentStateFile(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir, "")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if _, err := f.AddAgent("statetest", "feature/statetest"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	stateFilePath := filepath.Join(f.FleetDir, "states", "statetest.json")
	if err := f.UpdateAgentStateFile("statetest", stateFilePath); err != nil {
		t.Fatalf("UpdateAgentStateFile failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	agent, _ := loaded.GetAgent("statetest")
	if agent.StateFilePath != stateFilePath {
		t.Errorf("StateFilePath = %q, want %q", agent.StateFilePath, stateFilePath)
	}
}

func TestUpdateAgentStateFile_NotFound(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir, "")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err = f.UpdateAgentStateFile("ghost", "/some/path")
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateAgentDriver(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir, "")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if _, err := f.AddAgent("drivertest", "feature/drivertest"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	if err := f.UpdateAgentDriver("drivertest", "aider"); err != nil {
		t.Fatalf("UpdateAgentDriver failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	agent, _ := loaded.GetAgent("drivertest")
	if agent.Driver != "aider" {
		t.Errorf("Driver = %q, want %q", agent.Driver, "aider")
	}
}

func TestUpdateAgentDriver_NotFound(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir, "")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err = f.UpdateAgentDriver("ghost", "claude-code")
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestUpdateAgentDriverConfig(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir, "")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	if _, err := f.AddAgent("cfgtest", "feature/cfgtest"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	cfg := &DriverConfig{
		Command:    "my-agent",
		Args:       []string{"--verbose"},
		PromptFlag: "--prompt",
	}
	if err := f.UpdateAgentDriverConfig("cfgtest", cfg); err != nil {
		t.Fatalf("UpdateAgentDriverConfig failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	agent, _ := loaded.GetAgent("cfgtest")
	if agent.DriverConfig == nil {
		t.Fatal("DriverConfig should not be nil")
	}
	if agent.DriverConfig.Command != "my-agent" {
		t.Errorf("Command = %q, want %q", agent.DriverConfig.Command, "my-agent")
	}
	if agent.DriverConfig.PromptFlag != "--prompt" {
		t.Errorf("PromptFlag = %q, want %q", agent.DriverConfig.PromptFlag, "--prompt")
	}
}

func TestUpdateAgentDriverConfig_NotFound(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir, "")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err = f.UpdateAgentDriverConfig("ghost", &DriverConfig{})
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestUpdateAgent_NotFound(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir, "")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err = f.UpdateAgent("ghost", "running", 1234)
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestUpdateAgentHooks_NotFound(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir, "")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err = f.UpdateAgentHooks("ghost", true)
	if err == nil {
		t.Fatal("expected error for missing agent")
	}
}

func TestAddToGitignore_AlreadyPresent(t *testing.T) {
	dir := t.TempDir()
	gitignorePath := filepath.Join(dir, ".gitignore")
	os.WriteFile(gitignorePath, []byte(".fleet\n"), 0644)

	addToGitignore(dir, ".fleet")

	content, _ := os.ReadFile(gitignorePath)
	count := strings.Count(string(content), ".fleet")
	if count != 1 {
		t.Errorf("entry duplicated: found %d times", count)
	}
}

func TestAddToGitignore_AppendsNewline(t *testing.T) {
	dir := t.TempDir()
	gitignorePath := filepath.Join(dir, ".gitignore")
	os.WriteFile(gitignorePath, []byte("node_modules"), 0644) // no trailing newline

	addToGitignore(dir, ".fleet")

	content, _ := os.ReadFile(gitignorePath)
	if !strings.Contains(string(content), "node_modules\n.fleet\n") {
		t.Errorf("expected newline before new entry, got: %q", content)
	}
}

func TestAddToGitignore_CreatesFile(t *testing.T) {
	dir := t.TempDir()

	addToGitignore(dir, ".fleet")

	content, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("gitignore not created: %v", err)
	}
	if !strings.Contains(string(content), ".fleet") {
		t.Errorf("expected .fleet in gitignore, got: %q", content)
	}
}

func TestInit_WithShortName(t *testing.T) {
	dir := setupTestRepo(t)

	f, err := Init(dir, "myfleet")
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if f.ShortName != "myfleet" {
		t.Errorf("ShortName = %q, want %q", f.ShortName, "myfleet")
	}
}

func TestLoadFromPath_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	fleetDir := filepath.Join(dir, ".fleet")
	os.MkdirAll(fleetDir, 0755)
	os.WriteFile(filepath.Join(fleetDir, "config.json"), []byte("{bad json"), 0644)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for malformed config")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("unexpected error: %v", err)
	}
}
