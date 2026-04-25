package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
)

func TestHandleSquadronInfo_WrongMethod(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/test/info", nil)
	w := httptest.NewRecorder()

	h.HandleSquadronInfo(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestHandleSquadronInfo_EmptyName(t *testing.T) {
	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodGet, "/api/squadron//info", nil)
	w := httptest.NewRecorder()

	h.HandleSquadronInfo(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty name, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSquadronInfo_NotFound(t *testing.T) {
	repoPath, fleetDir := createTestFleet(t, []agentFixture{})

	// Empty context — no channels.
	ctx := &fleetctx.Context{
		Agents:   map[string]string{},
		Channels: map[string]*fleetctx.Channel{},
	}
	data, _ := json.MarshalIndent(ctx, "", "  ")
	os.WriteFile(filepath.Join(fleetDir, "context.json"), data, 0644)

	h := NewHandlers(repoPath, fleetDir)
	req := httptest.NewRequest(http.MethodGet, "/api/squadron/ghost/info", nil)
	w := httptest.NewRecorder()

	h.HandleSquadronInfo(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing channel, got %d: %s", w.Code, w.Body.String())
	}
}

func TestHandleSquadronInfo_Success(t *testing.T) {
	repoPath, fleetDir := createTestFleet(t, []agentFixture{
		{Name: "alpha", Branch: "feat-alpha", Status: "working", Driver: "claude-code"},
		{Name: "bravo", Branch: "feat-bravo", Status: "working", Driver: "aider"},
	})

	// Write a context with the squadron channel.
	ctx := &fleetctx.Context{
		Agents: map[string]string{},
		Channels: map[string]*fleetctx.Channel{
			"squadron-myteam": {
				Name:    "squadron-myteam",
				Members: []string{"alpha", "bravo"},
			},
		},
	}
	data, _ := json.MarshalIndent(ctx, "", "  ")
	if err := os.WriteFile(filepath.Join(fleetDir, "context.json"), data, 0644); err != nil {
		t.Fatalf("write context.json: %v", err)
	}

	// Write prompt files for each agent so the handler can pick them up.
	promptsDir := filepath.Join(fleetDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		t.Fatalf("mkdir prompts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "alpha.txt"), []byte("alpha prompt"), 0644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(promptsDir, "bravo.txt"), []byte("bravo prompt"), 0644); err != nil {
		t.Fatalf("write prompt: %v", err)
	}

	h := NewHandlers(repoPath, fleetDir)
	req := httptest.NewRequest(http.MethodGet, "/api/squadron/myteam/info", nil)
	w := httptest.NewRecorder()

	h.HandleSquadronInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp SquadronInfoResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	if resp.Name != "myteam" {
		t.Errorf("Name = %q, want %q", resp.Name, "myteam")
	}
	if len(resp.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(resp.Members))
	}
	if len(resp.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(resp.Agents))
	}

	byName := map[string]SquadronAgentInfo{}
	for _, a := range resp.Agents {
		byName[a.Name] = a
	}
	if a := byName["alpha"]; a.Branch != "feat-alpha" || a.Driver != "claude-code" || a.Prompt != "alpha prompt" {
		t.Errorf("alpha: %+v", a)
	}
	if a := byName["bravo"]; a.Branch != "feat-bravo" || a.Driver != "aider" || a.Prompt != "bravo prompt" {
		t.Errorf("bravo: %+v", a)
	}
}

func TestHandleSquadronInfo_AgentNotInFleet(t *testing.T) {
	// A channel member that isn't in fleet config is reported by name only —
	// the handler should not 500.
	repoPath, fleetDir := createTestFleet(t, []agentFixture{})

	ctx := &fleetctx.Context{
		Agents: map[string]string{},
		Channels: map[string]*fleetctx.Channel{
			"squadron-orphan": {
				Name:    "squadron-orphan",
				Members: []string{"ghost-agent"},
			},
		},
	}
	data, _ := json.MarshalIndent(ctx, "", "  ")
	os.WriteFile(filepath.Join(fleetDir, "context.json"), data, 0644)

	h := NewHandlers(repoPath, fleetDir)
	req := httptest.NewRequest(http.MethodGet, "/api/squadron/orphan/info", nil)
	w := httptest.NewRecorder()

	h.HandleSquadronInfo(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 even when agent missing from fleet, got %d: %s", w.Code, w.Body.String())
	}

	var resp SquadronInfoResponse
	json.Unmarshal(w.Body.Bytes(), &resp)
	if len(resp.Agents) != 1 {
		t.Fatalf("expected 1 agent entry, got %d", len(resp.Agents))
	}
	if resp.Agents[0].Name != "ghost-agent" {
		t.Errorf("expected ghost-agent name preserved, got %q", resp.Agents[0].Name)
	}
	if resp.Agents[0].Branch != "" {
		t.Errorf("expected empty branch for unknown agent, got %q", resp.Agents[0].Branch)
	}
}
