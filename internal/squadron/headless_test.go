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

func TestRunHeadless_AutoPR_OnlyMergerGetsPRInstructions(t *testing.T) {
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

	// Pin the merge master so the assertions are deterministic.
	mergerName := "aaa"
	mergerPtr := mergerName
	data := &squadron.SquadronData{
		Name:        "alpha",
		Consensus:   "universal",
		BaseBranch:  "main",
		AutoMerge:   true,
		AutoPR:      true,
		MergeMaster: &mergerPtr,
		Agents: []squadron.SquadronAgent{
			{Name: "aaa", Branch: "squadron/alpha/aaa", Prompt: "do aaa"},
			{Name: "bbb", Branch: "squadron/alpha/bbb", Prompt: "do bbb"},
		},
	}

	gotMerger, _ := squadron.RunHeadless(f, data)
	if gotMerger != mergerName {
		t.Fatalf("expected merge master %q, got %q", mergerName, gotMerger)
	}

	// Merger prompt must include PR creation instructions.
	mergerPrompt, err := os.ReadFile(filepath.Join(f.FleetDir, "prompts", mergerName+".txt"))
	if err != nil {
		t.Fatalf("read merger prompt: %v", err)
	}
	if !strings.Contains(string(mergerPrompt), "gh pr create") {
		t.Errorf("merger prompt missing PR creation instructions")
	}
	if strings.Contains(string(mergerPrompt), "DO NOT create a pull request") {
		t.Errorf("merger prompt should not contain the non-merger no-PR block")
	}

	// Non-merger prompts must explicitly forbid PR creation and must NOT contain
	// the merger's gh pr create instructions.
	nonMergerPrompt, err := os.ReadFile(filepath.Join(f.FleetDir, "prompts", "bbb.txt"))
	if err != nil {
		t.Fatalf("read non-merger prompt: %v", err)
	}
	content := string(nonMergerPrompt)
	if !strings.Contains(content, "DO NOT create a pull request") {
		t.Errorf("non-merger prompt missing no-PR instruction")
	}
	if strings.Contains(content, "gh pr create --title") {
		t.Errorf("non-merger prompt should not contain merger PR creation instructions")
	}
}

func TestRunHeadless_NoAutoPR_NoNonMergerPRBlock(t *testing.T) {
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

	mergerName := "aaa"
	mergerPtr := mergerName
	data := &squadron.SquadronData{
		Name:        "alpha",
		Consensus:   "universal",
		BaseBranch:  "main",
		AutoMerge:   true,
		AutoPR:      false,
		MergeMaster: &mergerPtr,
		Agents: []squadron.SquadronAgent{
			{Name: "aaa", Branch: "squadron/alpha/aaa", Prompt: "do aaa"},
			{Name: "bbb", Branch: "squadron/alpha/bbb", Prompt: "do bbb"},
		},
	}

	if _, err := squadron.RunHeadless(f, data); err != nil {
		t.Fatalf("RunHeadless: %v", err)
	}

	nonMergerPrompt, err := os.ReadFile(filepath.Join(f.FleetDir, "prompts", "bbb.txt"))
	if err != nil {
		t.Fatalf("read non-merger prompt: %v", err)
	}
	if strings.Contains(string(nonMergerPrompt), "DO NOT create a pull request") {
		t.Errorf("non-merger should not get no-PR block when autoPR is disabled")
	}
}
