package squadron_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/squadron"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestRunHeadless_WritesPromptsWithSuffixes(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "commit", "--allow-empty", "-m", "init")

	f, err := fleet.Init(dir, "")
	if err != nil {
		t.Fatalf("fleet.Init: %v", err)
	}

	data := &squadron.SquadronData{
		Name:       "alpha",
		Consensus:  "universal",
		BaseBranch: "main",
		AutoMerge:  true,
		Agents: []squadron.SquadronAgent{
			{Name: "aaa", Branch: "squadron/alpha/aaa", Prompt: "do aaa"},
			{Name: "bbb", Branch: "squadron/alpha/bbb", Prompt: "do bbb"},
		},
	}

	mergeMaster, _ := squadron.RunHeadless(f, data)

	// With AutoMerge enabled, a merge master must be selected.
	if mergeMaster != "aaa" && mergeMaster != "bbb" {
		t.Errorf("expected mergeMaster to be one of the agents, got %q", mergeMaster)
	}

	for _, name := range []string{"aaa", "bbb"} {
		path := filepath.Join(f.FleetDir, "prompts", name+".txt")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("prompt file %s not written: %v", path, err)
			continue
		}
		content := string(b)
		if !strings.Contains(content, "Squadron Consensus Protocol (UNIVERSAL)") {
			t.Errorf("prompt %s missing consensus suffix", name)
		}
		if !strings.Contains(content, "do "+name) {
			t.Errorf("prompt %s missing original text", name)
		}
	}
}
