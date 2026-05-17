package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
	"github.com/MrBenJ/fleet-commander/internal/driver"
	"github.com/MrBenJ/fleet-commander/internal/execx"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/squadron"
	"github.com/MrBenJ/fleet-commander/internal/tmux"
)

// Handlers holds all REST API handler methods.
type Handlers struct {
	repoPath string       // repo root — for fleet.Load()
	fleetDir string       // .fleet directory — for context/channels
	runner   execx.Runner // shared subprocess runner with timeouts
	logger   *log.Logger  // for write-failure diagnostics
}

// Test seams: package-level vars that production handlers route through so
// unit tests can swap them out without standing up a fake runner.
var (
	driverGet      = driver.Get
	execLookPath   = exec.LookPath
	driverBinaries = map[string]string{
		"codex": "codex",
	}
)

// NewHandlers creates a new Handlers instance with the default Runner and
// the stdlib default logger.
func NewHandlers(repoPath, fleetDir string) *Handlers {
	return NewHandlersWithRunner(repoPath, fleetDir, execx.DefaultRunner())
}

// NewHandlersWithRunner creates a Handlers with an injected Runner. Used by
// tests to provide a fake. Logger falls back to the stdlib default.
func NewHandlersWithRunner(repoPath, fleetDir string, runner execx.Runner) *Handlers {
	return NewHandlersFull(repoPath, fleetDir, runner, log.Default())
}

// NewHandlersWithLogger creates a Handlers that emits write-failure
// diagnostics to the supplied logger. A nil logger falls back to the
// stdlib default. Runner is the default.
func NewHandlersWithLogger(repoPath, fleetDir string, logger *log.Logger) *Handlers {
	return NewHandlersFull(repoPath, fleetDir, execx.DefaultRunner(), logger)
}

// NewHandlersFull is the full constructor — allows injecting both a Runner
// and a logger. Nil for either falls back to its default.
func NewHandlersFull(repoPath, fleetDir string, runner execx.Runner, logger *log.Logger) *Handlers {
	if runner == nil {
		runner = execx.DefaultRunner()
	}
	if logger == nil {
		logger = log.Default()
	}
	return &Handlers{repoPath: repoPath, fleetDir: fleetDir, runner: runner, logger: logger}
}

// HandleAvailableDrivers handles GET /api/drivers/available — returns runtime CLI availability
// for each driver the wizard can present, so the UI can disable options
// whose binary isn't installed.
func (h *Handlers) HandleAvailableDrivers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	drivers := []AvailableDriverResponse{
		{Name: "claude-code", Available: true},
		{Name: "codex", Available: isDriverBinaryAvailable("codex")},
	}

	writeJSON(w, http.StatusOK, drivers)
}

// isDriverBinaryAvailable reports whether the binary required by a driver
// is on PATH. Unknown drivers are treated as available (the driver layer
// owns the actual lookup) so unknown-name failures surface elsewhere.
func isDriverBinaryAvailable(name string) bool {
	binary, ok := driverBinaries[name]
	if !ok {
		return true
	}
	_, err := execLookPath(binary)
	return err == nil
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
		writeJSONWithLogger(w, http.StatusBadRequest, ErrorResponse{
			Error:   "validation failed",
			Details: details,
		}, h.logger)
		return
	}

	// Pre-flight: autoPR requires the gh CLI.
	if data.AutoPR {
		if _, err := h.runner.LookPath("gh"); err != nil {
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

// writeJSON encodes v as JSON and writes it with the given status code.
// Write failures are logged through the stdlib default logger rather than
// silently swallowed.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	writeJSONWithLogger(w, status, v, log.Default())
}

// writeError writes an ErrorResponse as JSON with the given status code.
// Write failures are logged through the stdlib default logger.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSONWithLogger(w, status, ErrorResponse{Error: msg}, log.Default())
}

// writeJSONWithLogger is the shared implementation that captures any
// encode/write error and forwards it to the supplied logger.
func writeJSONWithLogger(w http.ResponseWriter, status int, v interface{}, logger *log.Logger) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		if logger == nil {
			logger = log.Default()
		}
		logger.Printf("hangar/api: writeJSON: %v", err)
	}
}
