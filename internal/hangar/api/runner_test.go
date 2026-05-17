package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/execx"
)

type apiRunner struct {
	outputs  map[string][]byte
	lookErr  error
	commands [][]string
}

func (r *apiRunner) Run(context.Context, execx.Options) error {
	return nil
}

func (r *apiRunner) Output(_ context.Context, opts execx.Options) ([]byte, error) {
	r.commands = append(r.commands, append([]string{opts.Name}, opts.Args...))
	key := opts.Name
	for _, arg := range opts.Args {
		key += " " + arg
	}
	out, ok := r.outputs[key]
	if !ok {
		return nil, errors.New("missing fake output")
	}
	return out, nil
}

func (r *apiRunner) CombinedOutput(context.Context, execx.Options) ([]byte, error) {
	return nil, nil
}

func (r *apiRunner) LookPath(file string) (string, error) {
	if r.lookErr != nil {
		return "", r.lookErr
	}
	return "/bin/" + file, nil
}

func TestHandleGetBranchesUsesRunnerAndFiltersLinkedWorktrees(t *testing.T) {
	runner := &apiRunner{outputs: map[string][]byte{
		"git worktree list --porcelain":        []byte("worktree /repo\nHEAD abc\nbranch refs/heads/main\n\nworktree /repo/.fleet/worktrees/agent\nHEAD def\nbranch refs/heads/feature/agent\n\n"),
		"git branch --format=%(refname:short)": []byte("main\nfeature/agent\nfeature/free\n"),
	}}
	h := NewHandlersWithRunner("/repo", "/repo/.fleet", runner)

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
	if want := []string{"main", "feature/free"}; !reflect.DeepEqual(branches, want) {
		t.Fatalf("branches = %#v, want %#v", branches, want)
	}
	if len(runner.commands) != 2 {
		t.Fatalf("expected 2 runner commands, got %d", len(runner.commands))
	}
}

func TestHandleGetFleetUsesRunnerForGHAvailability(t *testing.T) {
	repoPath, fleetDir := createTestFleet(t, []agentFixture{})
	h := NewHandlersWithRunner(repoPath, fleetDir, &apiRunner{lookErr: errors.New("no gh")})

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
	if resp.GHAvailable {
		t.Fatal("expected GHAvailable=false when runner cannot find gh")
	}
}
