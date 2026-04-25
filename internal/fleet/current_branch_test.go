//go:build !windows

package fleet

import (
	"os/exec"
	"strings"
	"testing"
)

func TestCurrentBranch_ReturnsHeadBranch(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir, "")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	got, err := f.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	// `git init` defaults to either "main" or "master" depending on the user's
	// git config. Either is fine; we just want to verify we got a non-empty
	// branch name and that it matches what git itself reports.
	if got == "" {
		t.Fatal("expected non-empty current branch")
	}

	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse: %v", err)
	}
	want := strings.TrimSpace(string(out))
	if got != want {
		t.Errorf("CurrentBranch = %q, want %q", got, want)
	}
}

func TestCurrentBranch_TracksCheckout(t *testing.T) {
	dir := setupTestRepo(t)
	f, err := Init(dir, "")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	// Create and check out a new branch.
	for _, args := range [][]string{
		{"git", "-C", dir, "checkout", "-b", "my-feature"},
	} {
		if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
			t.Fatalf("%v: %s", args, out)
		}
	}

	got, err := f.CurrentBranch()
	if err != nil {
		t.Fatalf("CurrentBranch: %v", err)
	}
	if got != "my-feature" {
		t.Errorf("CurrentBranch = %q, want %q", got, "my-feature")
	}
}

func TestCurrentBranch_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	f := &Fleet{RepoPath: dir}

	if _, err := f.CurrentBranch(); err == nil {
		t.Fatal("expected error when not in a git repo, got nil")
	}
}
