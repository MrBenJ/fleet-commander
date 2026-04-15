package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
)

// createTestFleet sets up a temp dir that looks like a fleet repo:
// a git repo with .fleet/config.json containing the given agents.
func createTestFleet(t *testing.T, agents []agentFixture) (repoPath, fleetDir string) {
	t.Helper()

	dir := t.TempDir()

	// Init a real git repo so fleet.Load and git commands work.
	run(t, dir, "git", "init")
	run(t, dir, "git", "config", "user.email", "test@test.com")
	run(t, dir, "git", "config", "user.name", "Test")
	// Need at least one commit for branch commands to work.
	dummyFile := filepath.Join(dir, "README.md")
	os.WriteFile(dummyFile, []byte("test"), 0644)
	run(t, dir, "git", "add", ".")
	run(t, dir, "git", "commit", "-m", "init")

	fleetDir = filepath.Join(dir, ".fleet")
	os.MkdirAll(fleetDir, 0755)

	config := map[string]interface{}{
		"repo_path": dir,
		"fleet_dir": fleetDir,
		"agents":    agents,
	}
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(filepath.Join(fleetDir, "config.json"), data, 0644)

	return dir, fleetDir
}

type agentFixture struct {
	Name         string `json:"name"`
	Branch       string `json:"branch"`
	WorktreePath string `json:"worktree_path"`
	Status       string `json:"status"`
	PID          int    `json:"pid"`
	Driver       string `json:"driver,omitempty"`
	HooksOK      bool   `json:"hooks_ok"`
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
}

// --- HandleGetPersonas (existing, kept) ---

func TestHandleGetPersonas(t *testing.T) {
	h := NewHandlers("/tmp/fake-repo", "/tmp/fake-repo/.fleet")

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/personas", nil)
	w := httptest.NewRecorder()

	h.HandleGetPersonas(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var personas []PersonaResponse
	if err := json.Unmarshal(w.Body.Bytes(), &personas); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if len(personas) < 5 {
		t.Fatalf("expected at least 5 personas, got %d", len(personas))
	}

	if personas[0].Name == "" || personas[0].DisplayName == "" {
		t.Error("persona missing name or displayName")
	}
}

// --- HandleGetDrivers (existing, kept) ---

func TestHandleGetDrivers(t *testing.T) {
	h := NewHandlers("/tmp/fake-repo", "/tmp/fake-repo/.fleet")

	req := httptest.NewRequest(http.MethodGet, "/api/fleet/drivers", nil)
	w := httptest.NewRecorder()

	h.HandleGetDrivers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var drivers []DriverResponse
	if err := json.Unmarshal(w.Body.Bytes(), &drivers); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if len(drivers) < 3 {
		t.Fatalf("expected at least 3 drivers, got %d", len(drivers))
	}
}

func TestHandleGetPersonasWrongMethod(t *testing.T) {
	h := NewHandlers("/tmp/fake-repo", "/tmp/fake-repo/.fleet")

	req := httptest.NewRequest(http.MethodPost, "/api/fleet/personas", nil)
	w := httptest.NewRecorder()

	h.HandleGetPersonas(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// --- HandleGetFleet ---

func TestHandleGetFleet_Success(t *testing.T) {
	agents := []agentFixture{
		{Name: "alpha", Branch: "feat-alpha", Status: "working", Driver: "claude-code", HooksOK: true},
		{Name: "bravo", Branch: "feat-bravo", Status: "stopped", Driver: "aider"},
	}
	repoPath, fleetDir := createTestFleet(t, agents)

	h := NewHandlers(repoPath, fleetDir)
	req := httptest.NewRequest(http.MethodGet, "/api/fleet", nil)
	w := httptest.NewRecorder()

	h.HandleGetFleet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp FleetResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if resp.RepoPath != repoPath {
		t.Errorf("expected repoPath %q, got %q", repoPath, resp.RepoPath)
	}
	if len(resp.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(resp.Agents))
	}
	if resp.Agents[0].Name != "alpha" {
		t.Errorf("expected agent name 'alpha', got %q", resp.Agents[0].Name)
	}
	if resp.Agents[1].Status != "stopped" {
		t.Errorf("expected agent status 'stopped', got %q", resp.Agents[1].Status)
	}
}

func TestHandleGetFleet_EmptyAgents(t *testing.T) {
	repoPath, fleetDir := createTestFleet(t, []agentFixture{})

	h := NewHandlers(repoPath, fleetDir)
	req := httptest.NewRequest(http.MethodGet, "/api/fleet", nil)
	w := httptest.NewRecorder()

	h.HandleGetFleet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp FleetResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Agents) != 0 {
		t.Fatalf("expected 0 agents, got %d", len(resp.Agents))
	}
}

func TestHandleGetFleet_NoFleet(t *testing.T) {
	h := NewHandlers("/tmp/definitely-not-a-real-fleet-path-ever", "/tmp/nope/.fleet")
	req := httptest.NewRequest(http.MethodGet, "/api/fleet", nil)
	w := httptest.NewRecorder()

	h.HandleGetFleet(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error == "" {
		t.Error("expected error message in response")
	}
}

func TestHandleGetFleet_WrongMethod(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/fleet", nil)
	w := httptest.NewRecorder()

	h.HandleGetFleet(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// --- HandleGetBranches ---

func TestHandleGetBranches_Success(t *testing.T) {
	repoPath, fleetDir := createTestFleet(t, []agentFixture{})

	// Create a couple of branches so there's something to list.
	run(t, repoPath, "git", "branch", "feature-one")
	run(t, repoPath, "git", "branch", "feature-two")

	h := NewHandlers(repoPath, fleetDir)
	req := httptest.NewRequest(http.MethodGet, "/api/fleet/branches", nil)
	w := httptest.NewRecorder()

	h.HandleGetBranches(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var branches []string
	if err := json.Unmarshal(w.Body.Bytes(), &branches); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	// Should include main/master + the two branches we created.
	if len(branches) < 2 {
		t.Fatalf("expected at least 2 branches, got %d: %v", len(branches), branches)
	}

	found := map[string]bool{}
	for _, b := range branches {
		found[b] = true
	}
	if !found["feature-one"] || !found["feature-two"] {
		t.Errorf("expected feature-one and feature-two in branches, got %v", branches)
	}
}

func TestHandleGetBranches_ExcludesWorktreeBranches(t *testing.T) {
	repoPath, fleetDir := createTestFleet(t, []agentFixture{})

	run(t, repoPath, "git", "branch", "keep-this")
	run(t, repoPath, "git", "branch", "worktree-branch")

	// Create a worktree for "worktree-branch" — it should be excluded.
	wtPath := filepath.Join(t.TempDir(), "wt")
	run(t, repoPath, "git", "worktree", "add", wtPath, "worktree-branch")
	t.Cleanup(func() {
		exec.Command("git", "-C", repoPath, "worktree", "remove", "--force", wtPath).Run()
	})

	h := NewHandlers(repoPath, fleetDir)
	req := httptest.NewRequest(http.MethodGet, "/api/fleet/branches", nil)
	w := httptest.NewRecorder()

	h.HandleGetBranches(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var branches []string
	json.Unmarshal(w.Body.Bytes(), &branches)

	for _, b := range branches {
		if b == "worktree-branch" {
			t.Error("worktree-branch should have been excluded from the list")
		}
	}

	found := false
	for _, b := range branches {
		if b == "keep-this" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'keep-this' in branches, got %v", branches)
	}
}

func TestHandleGetBranches_WrongMethod(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/fleet/branches", nil)
	w := httptest.NewRecorder()

	h.HandleGetBranches(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

// --- HandleLaunchSquadron ---

func TestHandleLaunchSquadron_WrongMethod(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodGet, "/api/squadron/launch", nil)
	w := httptest.NewRecorder()

	h.HandleLaunchSquadron(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleLaunchSquadron_BadJSON(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/launch", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleLaunchSquadron(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleLaunchSquadron_ValidationError_NoName(t *testing.T) {
	repoPath, fleetDir := createTestFleet(t, []agentFixture{})

	h := NewHandlers(repoPath, fleetDir)
	body := `{"name":"","consensus":"none","agents":[{"name":"a","prompt":"do stuff","driver":"claude-code"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/launch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleLaunchSquadron(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty squadron name, got %d: %s", w.Code, w.Body.String())
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "validation failed" {
		t.Errorf("expected 'validation failed' error, got %q", errResp.Error)
	}
	if len(errResp.Details) == 0 {
		t.Error("expected validation details")
	}
}

func TestHandleLaunchSquadron_ValidationError_NoAgents(t *testing.T) {
	repoPath, fleetDir := createTestFleet(t, []agentFixture{})

	h := NewHandlers(repoPath, fleetDir)
	body := `{"name":"test-squad","consensus":"none","agents":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/launch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleLaunchSquadron(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for no agents, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleLaunchSquadron_EmptyBody(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/launch", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleLaunchSquadron(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d: %s", w.Code, w.Body.String())
	}
}

// --- HandleStopAgent ---

func TestHandleStopAgent_WrongMethod(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodGet, "/api/agent/myagent/stop", nil)
	w := httptest.NewRecorder()

	h.HandleStopAgent(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleStopAgent_EmptyName(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/agent//stop", nil)
	w := httptest.NewRecorder()

	h.HandleStopAgent(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty agent name, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleStopAgent_NoFleet(t *testing.T) {
	h := NewHandlers("/tmp/definitely-not-a-fleet-path", "/tmp/nope/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/agent/test-agent/stop", nil)
	w := httptest.NewRecorder()

	h.HandleStopAgent(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500 when fleet doesn't exist, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleStopAgent_AgentNotInFleet(t *testing.T) {
	// Agent name doesn't exist in the fleet — should still succeed (204)
	// because the handler tolerates missing agents gracefully.
	repoPath, fleetDir := createTestFleet(t, []agentFixture{
		{Name: "alpha", Branch: "feat-alpha", Status: "working"},
	})

	h := NewHandlers(repoPath, fleetDir)
	req := httptest.NewRequest(http.MethodPost, "/api/agent/nonexistent/stop", nil)
	w := httptest.NewRecorder()

	h.HandleStopAgent(w, req)

	// The handler calls f.UpdateAgent which writes config —
	// it won't error on a missing agent name, it just skips.
	// So we expect 204 (no tmux session exists, no state file).
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleStopAgent_NameParsing(t *testing.T) {
	// Verify the path parsing extracts the agent name correctly.
	repoPath, fleetDir := createTestFleet(t, []agentFixture{
		{Name: "my-cool-agent", Branch: "feat", Status: "working"},
	})

	h := NewHandlers(repoPath, fleetDir)
	req := httptest.NewRequest(http.MethodPost, "/api/agent/my-cool-agent/stop", nil)
	w := httptest.NewRecorder()

	h.HandleStopAgent(w, req)

	// No tmux session running, so it should succeed silently.
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}
}

// --- HandleSquadronStatus ---

func TestHandleSquadronStatus_WrongMethod(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/test-squad/status", nil)
	w := httptest.NewRecorder()

	h.HandleSquadronStatus(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleSquadronStatus_EmptyName(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodGet, "/api/squadron//status", nil)
	w := httptest.NewRecorder()

	h.HandleSquadronStatus(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty squadron name, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSquadronStatus_ChannelNotFound(t *testing.T) {
	dir := t.TempDir()
	fleetDir := filepath.Join(dir, ".fleet")
	os.MkdirAll(fleetDir, 0755)

	// Write an empty context.json — no channels.
	ctx := &fleetctx.Context{
		Agents:   map[string]string{},
		Channels: map[string]*fleetctx.Channel{},
	}
	data, _ := json.MarshalIndent(ctx, "", "  ")
	os.WriteFile(filepath.Join(fleetDir, "context.json"), data, 0644)

	h := NewHandlers(dir, fleetDir)
	req := httptest.NewRequest(http.MethodGet, "/api/squadron/ghost-squad/status", nil)
	w := httptest.NewRecorder()

	h.HandleSquadronStatus(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSquadronStatus_Success(t *testing.T) {
	dir := t.TempDir()
	fleetDir := filepath.Join(dir, ".fleet")
	os.MkdirAll(fleetDir, 0755)

	ts := time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

	ctx := &fleetctx.Context{
		Agents: map[string]string{},
		Channels: map[string]*fleetctx.Channel{
			"squadron-alpha": {
				Name:    "squadron-alpha",
				Members: []string{"agent-a", "agent-b"},
				Log: []fleetctx.LogEntry{
					{Agent: "agent-a", Timestamp: ts, Message: "COMPLETED: did the thing"},
					{Agent: "agent-b", Timestamp: ts.Add(time.Minute), Message: "APPROVED: agent-a"},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(ctx, "", "  ")
	os.WriteFile(filepath.Join(fleetDir, "context.json"), data, 0644)

	h := NewHandlers(dir, fleetDir)
	req := httptest.NewRequest(http.MethodGet, "/api/squadron/alpha/status", nil)
	w := httptest.NewRecorder()

	h.HandleSquadronStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp ChannelStatusResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if resp.Name != "squadron-alpha" {
		t.Errorf("expected channel name 'squadron-alpha', got %q", resp.Name)
	}
	if len(resp.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(resp.Members))
	}
	if len(resp.Log) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(resp.Log))
	}
	if resp.Log[0].Agent != "agent-a" {
		t.Errorf("expected first log agent 'agent-a', got %q", resp.Log[0].Agent)
	}
	if resp.Log[0].Message != "COMPLETED: did the thing" {
		t.Errorf("expected first log message, got %q", resp.Log[0].Message)
	}
	if resp.Log[0].Timestamp == "" {
		t.Error("expected timestamp to be set")
	}
}

func TestHandleSquadronStatus_EmptyLog(t *testing.T) {
	dir := t.TempDir()
	fleetDir := filepath.Join(dir, ".fleet")
	os.MkdirAll(fleetDir, 0755)

	ctx := &fleetctx.Context{
		Agents: map[string]string{},
		Channels: map[string]*fleetctx.Channel{
			"squadron-empty": {
				Name:    "squadron-empty",
				Members: []string{"x", "y"},
				Log:     nil,
			},
		},
	}
	data, _ := json.MarshalIndent(ctx, "", "  ")
	os.WriteFile(filepath.Join(fleetDir, "context.json"), data, 0644)

	h := NewHandlers(dir, fleetDir)
	req := httptest.NewRequest(http.MethodGet, "/api/squadron/empty/status", nil)
	w := httptest.NewRecorder()

	h.HandleSquadronStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp ChannelStatusResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Log) != 0 {
		t.Fatalf("expected 0 log entries, got %d", len(resp.Log))
	}
}

// --- HandleGenerate ---

func TestHandleGenerate_WrongMethod(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodGet, "/api/squadron/generate", nil)
	w := httptest.NewRecorder()

	h.HandleGenerate(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleGenerate_BadJSON(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/generate", strings.NewReader("not json at all"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleGenerate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleGenerate_EmptyDescription(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/generate", strings.NewReader(`{"description":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleGenerate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty description, got %d: %s", w.Code, w.Body.String())
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if !strings.Contains(errResp.Error, "description is required") {
		t.Errorf("expected 'description is required' error, got %q", errResp.Error)
	}
}

func TestHandleGenerate_EmptyBody(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/generate", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleGenerate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d: %s", w.Code, w.Body.String())
	}
}

// --- extractJSON helper ---

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNil  bool
		wantJSON string
	}{
		{
			name:     "clean array",
			input:    `[{"name":"a"}]`,
			wantJSON: `[{"name":"a"}]`,
		},
		{
			name:     "prose before array",
			input:    `Here are the agents:\n[{"name":"a"},{"name":"b"}]\nDone.`,
			wantJSON: `[{"name":"a"},{"name":"b"}]`,
		},
		{
			name:     "nested arrays",
			input:    `[[1,2],[3,4]]`,
			wantJSON: `[[1,2],[3,4]]`,
		},
		{
			name:    "no array",
			input:   `just some text`,
			wantNil: true,
		},
		{
			name:    "empty string",
			input:   ``,
			wantNil: true,
		},
		{
			name:    "unclosed bracket",
			input:   `[{"name":"a"`,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractJSON([]byte(tt.input))
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %q", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected result, got nil")
			}
			if string(result) != tt.wantJSON {
				t.Errorf("expected %q, got %q", tt.wantJSON, string(result))
			}
		})
	}
}

// --- Method enforcement for all handlers (405 sweep) ---

func TestAllHandlers_MethodEnforcement(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")

	tests := []struct {
		name    string
		handler func(http.ResponseWriter, *http.Request)
		bad     string
	}{
		{"GetFleet", h.HandleGetFleet, http.MethodPost},
		{"GetFleet-PUT", h.HandleGetFleet, http.MethodPut},
		{"GetFleet-DELETE", h.HandleGetFleet, http.MethodDelete},
		{"GetPersonas", h.HandleGetPersonas, http.MethodPost},
		{"GetDrivers", h.HandleGetDrivers, http.MethodDelete},
		{"GetBranches", h.HandleGetBranches, http.MethodPost},
		{"LaunchSquadron", h.HandleLaunchSquadron, http.MethodGet},
		{"LaunchSquadron-PUT", h.HandleLaunchSquadron, http.MethodPut},
		{"StopAgent", h.HandleStopAgent, http.MethodGet},
		{"StopAgent-DELETE", h.HandleStopAgent, http.MethodDelete},
		{"SquadronStatus", h.HandleSquadronStatus, http.MethodPost},
		{"Generate", h.HandleGenerate, http.MethodGet},
		{"Generate-PUT", h.HandleGenerate, http.MethodPut},
	}

	for _, tt := range tests {
		t.Run(tt.name+"_"+tt.bad, func(t *testing.T) {
			req := httptest.NewRequest(tt.bad, "/test", nil)
			w := httptest.NewRecorder()
			tt.handler(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("expected 405 for %s %s, got %d", tt.bad, tt.name, w.Code)
			}
		})
	}
}

// --- writeJSON / writeError helpers ---

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	writeJSON(w, http.StatusOK, map[string]string{"hello": "world"})

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}

	var result map[string]string
	json.Unmarshal(w.Body.Bytes(), &result)
	if result["hello"] != "world" {
		t.Errorf("expected hello=world, got %v", result)
	}
}

func TestWriteError(t *testing.T) {
	w := httptest.NewRecorder()
	writeError(w, http.StatusBadRequest, "you messed up")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if errResp.Error != "you messed up" {
		t.Errorf("expected 'you messed up', got %q", errResp.Error)
	}
}

// --- HandleLaunchSquadron autoPR gh check ---

func TestHandleLaunchSquadron_AutoPR_NoGH(t *testing.T) {
	repoPath, fleetDir := createTestFleet(t, []agentFixture{})

	// Override PATH so gh is not found.
	t.Setenv("PATH", t.TempDir())

	h := NewHandlers(repoPath, fleetDir)
	body := `{"name":"test","consensus":"none","autoMerge":true,"autoPR":true,"agents":[{"name":"a","branch":"squadron/test/a","prompt":"do stuff","driver":"claude-code"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/launch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleLaunchSquadron(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 when gh is missing and autoPR=true, got %d: %s", w.Code, w.Body.String())
	}

	var errResp ErrorResponse
	json.Unmarshal(w.Body.Bytes(), &errResp)
	if !strings.Contains(errResp.Error, "gh") {
		t.Errorf("error should mention gh CLI, got %q", errResp.Error)
	}
}

func TestHandleGetFleet_GHAvailable(t *testing.T) {
	repoPath, fleetDir := createTestFleet(t, []agentFixture{})

	h := NewHandlers(repoPath, fleetDir)
	req := httptest.NewRequest(http.MethodGet, "/api/fleet", nil)
	w := httptest.NewRecorder()

	h.HandleGetFleet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// In CI or dev environments, gh may or may not be installed.
	// We just verify the field is present in the JSON (not its value).
	raw := make(map[string]interface{})
	json.Unmarshal(w.Body.Bytes(), &raw)
	if _, ok := raw["ghAvailable"]; !ok {
		t.Error("expected 'ghAvailable' field in fleet response")
	}
}

func TestHandleGetFleet_GHUnavailable(t *testing.T) {
	repoPath, fleetDir := createTestFleet(t, []agentFixture{})

	// Override PATH so gh is not found.
	t.Setenv("PATH", t.TempDir())

	h := NewHandlers(repoPath, fleetDir)
	req := httptest.NewRequest(http.MethodGet, "/api/fleet", nil)
	w := httptest.NewRecorder()

	h.HandleGetFleet(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp FleetResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp.GHAvailable {
		t.Error("expected ghAvailable=false when gh is not in PATH")
	}
}

func TestHandleLaunchSquadron_AutoPR_False_NoGHCheck(t *testing.T) {
	repoPath, fleetDir := createTestFleet(t, []agentFixture{})

	// Override PATH so gh is not found — should not matter when autoPR is false.
	t.Setenv("PATH", t.TempDir())

	h := NewHandlers(repoPath, fleetDir)
	body := `{"name":"test","consensus":"none","autoMerge":true,"agents":[{"name":"a","branch":"squadron/test/a","prompt":"do stuff","driver":"claude-code"}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/launch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleLaunchSquadron(w, req)

	// Should NOT get 400 for missing gh — it should proceed past the gh check.
	// It will fail later (fleet.Load on a temp dir won't have worktrees), but not with 400.
	if w.Code == http.StatusBadRequest {
		var errResp ErrorResponse
		json.Unmarshal(w.Body.Bytes(), &errResp)
		if strings.Contains(errResp.Error, "gh") {
			t.Fatalf("should not check for gh when autoPR is false, got: %s", errResp.Error)
		}
	}
}
