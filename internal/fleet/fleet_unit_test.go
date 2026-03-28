//go:build !windows

package fleet

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
		{"git", "-C", dir, "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v failed: %s", args, out)
		}
	}
	return dir
}

func TestInit_CreatesFleetDir(t *testing.T) {
	dir := setupTestRepo(t)

	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// .fleet directory exists
	fleetDir := filepath.Join(dir, ".fleet")
	if info, err := os.Stat(fleetDir); err != nil || !info.IsDir() {
		t.Fatalf(".fleet directory not created")
	}

	// config.json exists
	configPath := filepath.Join(fleetDir, "config.json")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("config.json not created: %v", err)
	}

	// worktrees directory exists
	worktreesDir := filepath.Join(fleetDir, "worktrees")
	if info, err := os.Stat(worktreesDir); err != nil || !info.IsDir() {
		t.Fatalf("worktrees directory not created")
	}

	// .fleet is in .gitignore
	gitignore, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatalf("failed to read .gitignore: %v", err)
	}
	if !strings.Contains(string(gitignore), ".fleet") {
		t.Fatalf(".fleet not found in .gitignore")
	}

	// Fleet struct is sane
	if f.RepoPath != dir {
		t.Errorf("RepoPath = %q, want %q", f.RepoPath, dir)
	}
	if len(f.Agents) != 0 {
		t.Errorf("expected 0 agents, got %d", len(f.Agents))
	}
}

func TestInit_NotAGitRepo(t *testing.T) {
	dir := t.TempDir() // plain directory, no git init
	_, err := Init(dir)
	if err == nil {
		t.Fatal("expected error for non-git directory, got nil")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLoad_FindsConfig(t *testing.T) {
	dir := setupTestRepo(t)
	_, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.RepoPath != dir {
		t.Errorf("RepoPath = %q, want %q", loaded.RepoPath, dir)
	}
}

func TestLoad_WalksUp(t *testing.T) {
	dir := setupTestRepo(t)
	_, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	subdir := filepath.Join(dir, "some", "nested", "dir")
	if err := os.MkdirAll(subdir, 0755); err != nil {
		t.Fatalf("failed to create subdir: %v", err)
	}

	loaded, err := Load(subdir)
	if err != nil {
		t.Fatalf("Load from subdir failed: %v", err)
	}
	if loaded.RepoPath != dir {
		t.Errorf("RepoPath = %q, want %q", loaded.RepoPath, dir)
	}
}

func TestLoad_NoFleet(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when no fleet exists, got nil")
	}
	if !strings.Contains(err.Error(), "no fleet found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAddAgent_CreatesWorktreeAndConfig(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	agent, err := f.AddAgent("alice", "feature/alice")
	if err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	if agent.Name != "alice" {
		t.Errorf("Name = %q, want %q", agent.Name, "alice")
	}
	if agent.Branch != "feature/alice" {
		t.Errorf("Branch = %q, want %q", agent.Branch, "feature/alice")
	}
	if agent.Status != "ready" {
		t.Errorf("Status = %q, want %q", agent.Status, "ready")
	}

	// Worktree directory should exist
	if _, err := os.Stat(agent.WorktreePath); err != nil {
		t.Errorf("worktree path does not exist: %v", err)
	}

	// Verify persistence by reloading
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded.Agents) != 1 {
		t.Fatalf("expected 1 agent after reload, got %d", len(loaded.Agents))
	}
	if loaded.Agents[0].Name != "alice" {
		t.Errorf("reloaded agent name = %q, want %q", loaded.Agents[0].Name, "alice")
	}
}

func TestAddAgent_DuplicateNameFails(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if _, err := f.AddAgent("bob", "feature/bob"); err != nil {
		t.Fatalf("first AddAgent failed: %v", err)
	}

	_, err = f.AddAgent("bob", "feature/bob2")
	if err == nil {
		t.Fatal("expected error for duplicate agent name, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestGetAgent_Found(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if _, err := f.AddAgent("charlie", "feature/charlie"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	agent, err := f.GetAgent("charlie")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if agent.Name != "charlie" {
		t.Errorf("Name = %q, want %q", agent.Name, "charlie")
	}
}

func TestGetAgent_NotFound(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	_, err = f.GetAgent("nonexistent")
	if err == nil {
		t.Fatal("expected error for missing agent, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRemoveAgent(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if _, err := f.AddAgent("dave", "feature/dave"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}
	if _, err := f.AddAgent("eve", "feature/eve"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	if err := f.RemoveAgent("dave"); err != nil {
		t.Fatalf("RemoveAgent failed: %v", err)
	}

	if len(f.Agents) != 1 {
		t.Errorf("expected 1 agent, got %d", len(f.Agents))
	}

	// Verify persistence
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded.Agents) != 1 {
		t.Errorf("expected 1 agent after reload, got %d", len(loaded.Agents))
	}
	if loaded.Agents[0].Name != "eve" {
		t.Errorf("remaining agent = %q, want %q", loaded.Agents[0].Name, "eve")
	}
}

func TestRemoveAgent_NotFound(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err = f.RemoveAgent("ghost")
	if err == nil {
		t.Fatal("expected error for missing agent, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestUpdateAgent_PersistsStatusAndPID(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if _, err := f.AddAgent("frank", "feature/frank"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	if err := f.UpdateAgent("frank", "running", 12345); err != nil {
		t.Fatalf("UpdateAgent failed: %v", err)
	}

	// Verify persistence
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	agent, err := loaded.GetAgent("frank")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if agent.Status != "running" {
		t.Errorf("Status = %q, want %q", agent.Status, "running")
	}
	if agent.PID != 12345 {
		t.Errorf("PID = %d, want %d", agent.PID, 12345)
	}
}

func TestUpdateAgentHooks(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if _, err := f.AddAgent("grace", "feature/grace"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	if err := f.UpdateAgentHooks("grace", true); err != nil {
		t.Fatalf("UpdateAgentHooks failed: %v", err)
	}

	// Verify persistence
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	agent, err := loaded.GetAgent("grace")
	if err != nil {
		t.Fatalf("GetAgent failed: %v", err)
	}
	if !agent.HooksOK {
		t.Error("HooksOK = false, want true")
	}
}

func TestRenameAgent(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if _, err := f.AddAgent("heidi", "feature/heidi"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	if err := f.RenameAgent("heidi", "helen"); err != nil {
		t.Fatalf("RenameAgent failed: %v", err)
	}

	// Old name should be gone
	if _, err := f.GetAgent("heidi"); err == nil {
		t.Error("expected old name 'heidi' to be gone")
	}

	// New name should exist
	agent, err := f.GetAgent("helen")
	if err != nil {
		t.Fatalf("GetAgent('helen') failed: %v", err)
	}

	// Worktree path should be updated
	expectedPath := filepath.Join(f.FleetDir, "worktrees", "helen")
	if agent.WorktreePath != expectedPath {
		t.Errorf("WorktreePath = %q, want %q", agent.WorktreePath, expectedPath)
	}

	// New worktree should exist on disk
	if _, err := os.Stat(expectedPath); err != nil {
		t.Errorf("new worktree path does not exist: %v", err)
	}

	// Verify persistence
	loaded, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if _, err := loaded.GetAgent("helen"); err != nil {
		t.Error("renamed agent not found after reload")
	}
	if _, err := loaded.GetAgent("heidi"); err == nil {
		t.Error("old agent name still exists after reload")
	}
}

func TestRenameAgent_SameName(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if _, err := f.AddAgent("ivan", "feature/ivan"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	err = f.RenameAgent("ivan", "ivan")
	if err == nil {
		t.Fatal("expected error renaming to same name, got nil")
	}
	if !strings.Contains(err.Error(), "same") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRenameAgent_TargetExists(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if _, err := f.AddAgent("judy", "feature/judy"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}
	if _, err := f.AddAgent("kate", "feature/kate"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	err = f.RenameAgent("judy", "kate")
	if err == nil {
		t.Fatal("expected error renaming to existing name, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConfigPersistence_RoundTrip(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir)
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	if _, err := f.AddAgent("lima", "feature/lima"); err != nil {
		t.Fatalf("AddAgent failed: %v", err)
	}

	if err := f.UpdateAgent("lima", "running", 9999); err != nil {
		t.Fatalf("UpdateAgent failed: %v", err)
	}

	if err := f.UpdateAgentHooks("lima", true); err != nil {
		t.Fatalf("UpdateAgentHooks failed: %v", err)
	}

	// Read raw JSON and verify structure
	data, err := os.ReadFile(filepath.Join(dir, ".fleet", "config.json"))
	if err != nil {
		t.Fatalf("failed to read config.json: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to parse raw JSON: %v", err)
	}

	// Should have repo_path, fleet_dir, agents keys
	for _, key := range []string{"repo_path", "fleet_dir", "agents"} {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing key %q in config JSON", key)
		}
	}

	// Parse agents array
	var agents []map[string]interface{}
	if err := json.Unmarshal(raw["agents"], &agents); err != nil {
		t.Fatalf("failed to parse agents: %v", err)
	}

	if len(agents) != 1 {
		t.Fatalf("expected 1 agent, got %d", len(agents))
	}

	a := agents[0]
	if a["name"] != "lima" {
		t.Errorf("name = %v, want %q", a["name"], "lima")
	}
	if a["status"] != "running" {
		t.Errorf("status = %v, want %q", a["status"], "running")
	}
	if pid, ok := a["pid"].(float64); !ok || int(pid) != 9999 {
		t.Errorf("pid = %v, want 9999", a["pid"])
	}
	if a["hooks_ok"] != true {
		t.Errorf("hooks_ok = %v, want true", a["hooks_ok"])
	}
}
