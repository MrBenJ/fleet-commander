package worktree

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/execx"
)

type recordingRunner struct {
	runErrs []error
	calls   []execx.Options
}

func (r *recordingRunner) Run(_ context.Context, opts execx.Options) error {
	r.calls = append(r.calls, opts)
	if len(r.runErrs) == 0 {
		return nil
	}
	err := r.runErrs[0]
	r.runErrs = r.runErrs[1:]
	return err
}

func (r *recordingRunner) Output(context.Context, execx.Options) ([]byte, error) {
	return nil, nil
}

func (r *recordingRunner) CombinedOutput(context.Context, execx.Options) ([]byte, error) {
	return nil, nil
}

func (r *recordingRunner) LookPath(file string) (string, error) {
	return file, nil
}

func TestCreateUsesRunnerForHeadCheckInitialCommitAndWorktree(t *testing.T) {
	runner := &recordingRunner{runErrs: []error{errors.New("no head"), nil, nil}}
	repoDir := t.TempDir()
	manager := NewManagerWithRunner(repoDir, runner)

	worktreePath := repoDir + "/.fleet/worktrees/alpha"
	if err := manager.Create(worktreePath, "feature/alpha"); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	var got [][]string
	for _, call := range runner.calls {
		got = append(got, append([]string{call.Name}, call.Args...))
	}
	want := [][]string{
		{"git", "rev-parse", "--verify", "HEAD"},
		{"git", "commit", "--allow-empty", "-m", "Initial commit"},
		{"git", "worktree", "add", "-b", "feature/alpha", worktreePath},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
}

func TestRemoveFallsBackToForceThenRemoveAll(t *testing.T) {
	dir := t.TempDir()
	runner := &recordingRunner{runErrs: []error{errors.New("normal failed"), nil}}
	manager := NewManagerWithRunner("/repo", runner)

	if err := manager.Remove(dir); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if len(runner.calls) != 2 {
		t.Fatalf("expected 2 git calls, got %d", len(runner.calls))
	}
	if got := runner.calls[1].Args; !reflect.DeepEqual(got, []string{"worktree", "remove", "--force", dir}) {
		t.Fatalf("force args = %#v", got)
	}
}
