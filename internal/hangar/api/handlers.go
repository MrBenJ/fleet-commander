package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
	"github.com/MrBenJ/fleet-commander/internal/driver"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/squadron"
	"github.com/MrBenJ/fleet-commander/internal/tmux"
)

// Handlers holds all REST API handler methods.
type Handlers struct {
	repoPath string // repo root — for fleet.Load()
	fleetDir string // .fleet directory — for context/channels
}

// NewHandlers creates a new Handlers instance.
// repoPath is the repo root (for fleet.Load), fleetDir is the .fleet path (for context).
func NewHandlers(repoPath, fleetDir string) *Handlers {
	return &Handlers{repoPath: repoPath, fleetDir: fleetDir}
}

// personaNames is the ordered list of built-in persona keys.
var personaNames = []string{
	"overconfident-engineer",
	"zen-master",
	"paranoid-perfectionist",
	"raging-jerk",
	"peter-molyneux",
}

// HandleGetFleet handles GET /api/fleet — loads and returns fleet info.
func (h *Handlers) HandleGetFleet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	f, err := fleet.Load(h.repoPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load fleet: %v", err))
		return
	}

	branch, err := f.CurrentBranch()
	if err != nil {
		branch = ""
	}

	agents := make([]AgentResponse, 0, len(f.Agents))
	for _, a := range f.Agents {
		agents = append(agents, AgentResponse{
			Name:          a.Name,
			Branch:        a.Branch,
			Status:        a.Status,
			Driver:        a.Driver,
			HooksOK:       a.HooksOK,
			StateFilePath: a.StateFilePath,
		})
	}

	_, ghErr := exec.LookPath("gh")

	writeJSON(w, http.StatusOK, FleetResponse{
		RepoPath:      f.RepoPath,
		CurrentBranch: branch,
		GHAvailable:   ghErr == nil,
		Agents:        agents,
	})
}

// HandleGetPersonas handles GET /api/fleet/personas — returns all built-in personas.
func (h *Handlers) HandleGetPersonas(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	personas := make([]PersonaResponse, 0, len(personaNames))
	for _, name := range personaNames {
		p, ok := squadron.LookupPersona(name)
		if !ok {
			continue
		}
		personas = append(personas, PersonaResponse{
			Name:        p.Name,
			DisplayName: p.DisplayName,
			Preamble:    p.Preamble,
		})
	}

	writeJSON(w, http.StatusOK, personas)
}

// HandleGetDrivers handles GET /api/fleet/drivers — returns available driver names.
func (h *Handlers) HandleGetDrivers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	names := driver.Available()
	drivers := make([]DriverResponse, 0, len(names))
	for _, name := range names {
		drivers = append(drivers, DriverResponse{Name: name})
	}

	writeJSON(w, http.StatusOK, drivers)
}

// HandleGetBranches handles GET /api/fleet/branches — returns available git branches,
// excluding branches currently checked out in worktrees.
func (h *Handlers) HandleGetBranches(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Get worktree branches to exclude (skip the main worktree so its
	// branch — typically "main" — stays available as a base branch).
	worktreeBranches := make(map[string]bool)
	wtCmd := exec.Command("git", "worktree", "list", "--porcelain")
	wtCmd.Dir = h.repoPath
	wtOut, err := wtCmd.Output()
	if err == nil {
		isMainWorktree := true
		for _, line := range strings.Split(string(wtOut), "\n") {
			if line == "" {
				isMainWorktree = false
				continue
			}
			if !isMainWorktree && strings.HasPrefix(line, "branch ") {
				ref := strings.TrimPrefix(line, "branch ")
				branch := strings.TrimPrefix(ref, "refs/heads/")
				worktreeBranches[branch] = true
			}
		}
	}

	// Get all local branches.
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = h.repoPath
	out, err := cmd.Output()
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list branches: %v", err))
		return
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name := strings.TrimSpace(line)
		if name == "" {
			continue
		}
		if worktreeBranches[name] {
			continue
		}
		branches = append(branches, name)
	}

	if branches == nil {
		branches = []string{}
	}

	writeJSON(w, http.StatusOK, branches)
}

// HandleLaunchSquadron handles POST /api/squadron/launch — validates and launches a squadron.
func (h *Handlers) HandleLaunchSquadron(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req LaunchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	// Convert LaunchRequest to SquadronData JSON for ParseAndValidate.
	squadronData := squadron.SquadronData{
		Name:         req.Name,
		Consensus:    req.Consensus,
		ReviewMaster: req.ReviewMaster,
		BaseBranch:   req.BaseBranch,
		AutoMerge:    req.AutoMerge,
		AutoPR:       req.AutoPR,
		MergeMaster:  req.MergeMaster,
		UseJumpSh:    req.UseJumpSh,
	}
	for _, a := range req.Agents {
		squadronData.Agents = append(squadronData.Agents, squadron.SquadronAgent{
			Name:      a.Name,
			Branch:    a.Branch,
			Prompt:    a.Prompt,
			Driver:    a.Driver,
			Persona:   a.Persona,
			FightMode: a.FightMode,
		})
	}

	jsonBytes, err := json.Marshal(squadronData)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to marshal request: %v", err))
		return
	}

	data, errs := squadron.ParseAndValidate(jsonBytes)
	if len(errs) > 0 {
		details := make([]string, 0, len(errs))
		for _, e := range errs {
			details = append(details, e.Error())
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(ErrorResponse{
			Error:   "validation failed",
			Details: details,
		})
		return
	}

	// Pre-flight: autoPR requires the gh CLI.
	if data.AutoPR {
		if _, err := exec.LookPath("gh"); err != nil {
			writeError(w, http.StatusBadRequest,
				"autoPR requires the gh CLI (https://cli.github.com) — install it and run `gh auth login`")
			return
		}
	}

	f, err := fleet.Load(h.repoPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load fleet: %v", err))
		return
	}

	mergeMaster, err := squadron.RunHeadless(f, data)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("launch failed: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, LaunchResponse{MergeMaster: mergeMaster})
}

// HandleStopAgent handles POST /api/agent/{name}/stop — stops a named agent.
func (h *Handlers) HandleStopAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract agent name from path: /api/agent/{name}/stop
	path := strings.TrimPrefix(r.URL.Path, "/api/agent/")
	path = strings.TrimSuffix(path, "/stop")
	name := strings.TrimSpace(path)

	if name == "" {
		writeError(w, http.StatusBadRequest, "agent name is required")
		return
	}

	f, err := fleet.Load(h.repoPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load fleet: %v", err))
		return
	}

	tm := tmux.NewManager(f.TmuxPrefix())
	if tm.SessionExists(name) {
		if err := tm.KillSession(name); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to stop session: %v", err))
			return
		}
	}

	agent, err := f.GetAgent(name)
	if err == nil && agent.StateFilePath != "" {
		os.Remove(agent.StateFilePath)
		f.UpdateAgentStateFile(name, "")
	}
	if err == nil {
		drv, _ := driver.GetForAgent(agent)
		if drv != nil {
			drv.RemoveHooks(agent.WorktreePath)
		}
		f.UpdateAgentHooks(name, false)
	}
	f.UpdateAgent(name, "stopped", 0)

	w.WriteHeader(http.StatusNoContent)
}

// ChannelStatusResponse is the response type for squadron status.
type ChannelStatusResponse struct {
	Name    string                  `json:"name"`
	Members []string                `json:"members"`
	Log     []ChannelLogEntryOutput `json:"log"`
}

// ChannelLogEntryOutput is a single log entry for the response.
type ChannelLogEntryOutput struct {
	Agent     string `json:"agent"`
	Timestamp string `json:"timestamp"`
	Message   string `json:"message"`
}

// HandleSquadronStatus handles GET /api/squadron/{name}/status — returns squadron channel log.
func (h *Handlers) HandleSquadronStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Extract squadron name from path: /api/squadron/{name}/status
	path := strings.TrimPrefix(r.URL.Path, "/api/squadron/")
	path = strings.TrimSuffix(path, "/status")
	name := strings.TrimSpace(path)

	if name == "" {
		writeError(w, http.StatusBadRequest, "squadron name is required")
		return
	}

	channelName := "squadron-" + name

	ctx, err := fleetctx.Load(h.fleetDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load context: %v", err))
		return
	}

	ch, ok := ctx.Channels[channelName]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("channel %q not found", channelName))
		return
	}

	entries := make([]ChannelLogEntryOutput, 0, len(ch.Log))
	for _, entry := range ch.Log {
		entries = append(entries, ChannelLogEntryOutput{
			Agent:     entry.Agent,
			Timestamp: entry.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			Message:   entry.Message,
		})
	}

	writeJSON(w, http.StatusOK, ChannelStatusResponse{
		Name:    ch.Name,
		Members: ch.Members,
		Log:     entries,
	})
}

// HandleSquadronInfo handles GET /api/squadron/{name}/info — returns squadron
// agent details reconstructed from the channel members and fleet config.
func (h *Handlers) HandleSquadronInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/squadron/")
	path = strings.TrimSuffix(path, "/info")
	name := strings.TrimSpace(path)

	if name == "" {
		writeError(w, http.StatusBadRequest, "squadron name is required")
		return
	}

	channelName := "squadron-" + name

	ctx, err := fleetctx.Load(h.fleetDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load context: %v", err))
		return
	}

	ch, ok := ctx.Channels[channelName]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Sprintf("squadron %q not found", name))
		return
	}

	f, err := fleet.Load(h.repoPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to load fleet: %v", err))
		return
	}

	promptsDir := h.fleetDir + "/prompts"
	agents := make([]SquadronAgentInfo, 0, len(ch.Members))
	for _, memberName := range ch.Members {
		info := SquadronAgentInfo{Name: memberName}
		if agent, err := f.GetAgent(memberName); err == nil {
			info.Branch = agent.Branch
			info.Driver = agent.Driver
		}
		promptFile := promptsDir + "/" + memberName + ".txt"
		if data, err := os.ReadFile(promptFile); err == nil {
			info.Prompt = string(data)
		}
		agents = append(agents, info)
	}

	writeJSON(w, http.StatusOK, SquadronInfoResponse{
		Name:      name,
		Agents:    agents,
		Consensus: "universal",
		AutoMerge: true,
		Members:   ch.Members,
	})
}

// HandleGenerate handles POST /api/squadron/generate — uses claude-code to generate an agent breakdown.
func (h *Handlers) HandleGenerate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid request body: %v", err))
		return
	}

	if req.Description == "" {
		writeError(w, http.StatusBadRequest, "description is required")
		return
	}

	drv, err := driver.Get("claude-code")
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to get driver: %v", err))
		return
	}

	metaprompt := fmt.Sprintf(`You are a task decomposition assistant for Fleet Commander, a tool that manages parallel AI coding agents.

The user wants to accomplish the following:

%s

Break this down into individual agent tasks. Each agent handles exactly one task.

Respond with ONLY a JSON array (no markdown, no explanation, no code fences) where each element has:
- "name": a short kebab-case agent name. MUST be 20 characters or fewer, contain only letters, digits, and hyphens, and start with a letter or digit. Examples: "auth-agent", "test-writer", "db-migrator". Do NOT exceed 20 characters — names like "database-migration-agent" (24 chars) or "frontend-component-builder" (26 chars) will be rejected.
- "prompt": the full detailed task prompt for that agent
- "branch": leave as empty string (will be auto-generated)
- "driver": "claude-code"
- "persona": leave as empty string

Example format:
[{"name":"auth-agent","prompt":"Implement OAuth2 login flow with Google and GitHub providers","branch":"","driver":"claude-code","persona":""}]`, req.Description)

	out, err := drv.PlanCommand(metaprompt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("plan command failed: %v", err))
		return
	}

	jsonData := extractJSON(out)
	if jsonData == nil {
		writeError(w, http.StatusInternalServerError, "no JSON array found in plan output")
		return
	}

	var agents []LaunchAgentInput
	if err := json.Unmarshal(jsonData, &agents); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to parse plan output: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, GenerateResponse{Agents: agents})
}

// writeJSON encodes v as JSON and writes it with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes an ErrorResponse as JSON with the given status code.
func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{Error: msg})
}

// extractJSON finds the first JSON array in data (e.g., claude output that may
// include prose before/after the JSON).
func extractJSON(data []byte) []byte {
	start := bytes.IndexByte(data, '[')
	if start == -1 {
		return nil
	}
	// Find the matching closing bracket.
	depth := 0
	for i := start; i < len(data); i++ {
		switch data[i] {
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				return data[start : i+1]
			}
		}
	}
	return nil
}
