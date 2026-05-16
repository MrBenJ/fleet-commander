package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/MrBenJ/fleet-commander/internal/driver"
	"github.com/MrBenJ/fleet-commander/internal/execx"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/squadron"
)

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

	_, ghErr := h.runner.LookPath("gh")

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

	worktreeBranches := make(map[string]bool)
	wtOut, err := h.runner.Output(context.Background(), execx.Options{
		Name: "git",
		Args: []string{"worktree", "list", "--porcelain"},
		Dir:  h.repoPath,
	})
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

	out, err := h.runner.Output(context.Background(), execx.Options{
		Name: "git",
		Args: []string{"branch", "--format=%(refname:short)"},
		Dir:  h.repoPath,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to list branches: %v", err))
		return
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		name := strings.TrimSpace(line)
		if name == "" || worktreeBranches[name] {
			continue
		}
		branches = append(branches, name)
	}

	if branches == nil {
		branches = []string{}
	}

	writeJSON(w, http.StatusOK, branches)
}
