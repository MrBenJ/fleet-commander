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

func TestBuildFightModeSuffix_ContainsLabel(t *testing.T) {
	got := squadron.BuildFightModeSuffix("Overconfident Engineer")

	mustContain := []string{
		"Fight Mode",
		"Start some fights",
		"Make fun of them",
		"'Overconfident Engineer'",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("fight-mode suffix missing %q\n---\n%s", s, got)
		}
	}
}

func TestBuildFightModeSuffix_PreservesAgentNameWhenNoPersona(t *testing.T) {
	// When the caller passes the agent name (no persona), it should appear verbatim.
	got := squadron.BuildFightModeSuffix("api-refactor")
	if !strings.Contains(got, "'api-refactor'") {
		t.Errorf("expected agent name in suffix, got: %s", got)
	}
}

func TestBuildFightModeSuffix_EmptyLabel(t *testing.T) {
	// Edge case: empty label should not panic and should still produce the
	// header/body — only the trailing personaLabel is empty.
	got := squadron.BuildFightModeSuffix("")
	if !strings.Contains(got, "Fight Mode") {
		t.Error("fight-mode suffix should still contain header for empty label")
	}
}

func TestRunHeadless_FightModeUsesPersonaDisplayName(t *testing.T) {
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
		Consensus:  "none",
		BaseBranch: "main",
		AutoMerge:  false,
		Agents: []squadron.SquadronAgent{
			{
				Name:      "fighter",
				Branch:    "squadron/alpha/fighter",
				Prompt:    "do work",
				Persona:   "overconfident-engineer",
				FightMode: true,
			},
			{Name: "buddy", Branch: "squadron/alpha/buddy", Prompt: "do other work"},
		},
	}

	if _, err := squadron.RunHeadless(f, data); err != nil {
		t.Fatalf("RunHeadless: %v", err)
	}

	prompt, err := os.ReadFile(filepath.Join(f.FleetDir, "prompts", "fighter.txt"))
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	body := string(prompt)

	if !strings.Contains(body, "Fight Mode") {
		t.Error("fight-mode block missing from prompt")
	}
	// When persona is set, the persona DisplayName is used (not the agent name).
	if !strings.Contains(body, "'Overconfident Engineer'") {
		t.Errorf("expected persona DisplayName in fight-mode tail, got prompt:\n%s", body)
	}
	// And agent name should not be the fight-mode label here.
	if strings.Contains(body, "'fighter'") {
		t.Error("fight-mode label should be persona DisplayName, not agent name")
	}
}

func TestRunHeadless_FightModeFallsBackToAgentName(t *testing.T) {
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

	// No persona, but FightMode enabled — agent name is used as the label.
	data := &squadron.SquadronData{
		Name:       "alpha",
		Consensus:  "none",
		BaseBranch: "main",
		AutoMerge:  false,
		Agents: []squadron.SquadronAgent{
			{
				Name:      "loose-cannon",
				Branch:    "squadron/alpha/loose-cannon",
				Prompt:    "do work",
				FightMode: true,
			},
			{Name: "second", Branch: "squadron/alpha/second", Prompt: "support"},
		},
	}

	if _, err := squadron.RunHeadless(f, data); err != nil {
		t.Fatalf("RunHeadless: %v", err)
	}

	prompt, err := os.ReadFile(filepath.Join(f.FleetDir, "prompts", "loose-cannon.txt"))
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	if !strings.Contains(string(prompt), "'loose-cannon'") {
		t.Errorf("expected agent name in fight-mode label when no persona set; got:\n%s", string(prompt))
	}
}

func TestRunHeadless_NoAgentsReturnsError(t *testing.T) {
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
		Name:       "empty",
		Consensus:  "none",
		BaseBranch: "main",
		Agents:     nil,
	}
	if _, err := squadron.RunHeadless(f, data); err == nil {
		t.Fatal("expected error for empty agents, got nil")
	}
}

func TestRunHeadless_NoConsensusButAutoMergeAddsPollerToNonMergers(t *testing.T) {
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

	mergerName := "merger"
	mergerPtr := mergerName
	data := &squadron.SquadronData{
		Name:        "alpha",
		Consensus:   "none",
		BaseBranch:  "main",
		AutoMerge:   true,
		MergeMaster: &mergerPtr,
		Agents: []squadron.SquadronAgent{
			{Name: "merger", Branch: "squadron/alpha/merger", Prompt: "do merger work"},
			{Name: "worker", Branch: "squadron/alpha/worker", Prompt: "do worker work"},
		},
	}

	gotMerger, err := squadron.RunHeadless(f, data)
	if err != nil {
		t.Fatalf("RunHeadless: %v", err)
	}
	if gotMerger != mergerName {
		t.Fatalf("expected merger %q, got %q", mergerName, gotMerger)
	}

	// Merger gets the merger suffix, NOT the no-consensus auto-merge poller.
	mergerPrompt, err := os.ReadFile(filepath.Join(f.FleetDir, "prompts", "merger.txt"))
	if err != nil {
		t.Fatalf("read merger prompt: %v", err)
	}
	if !strings.Contains(string(mergerPrompt), "Squadron Merge Duties") {
		t.Error("merger prompt should contain merger duties block")
	}
	if strings.Contains(string(mergerPrompt), "Squadron Merge Monitoring") {
		t.Error("merger prompt should NOT contain the merge-monitoring poller — only non-mergers get that")
	}

	// Worker (non-merger) should get the merge monitoring poller, NOT the merger duties.
	workerPrompt, err := os.ReadFile(filepath.Join(f.FleetDir, "prompts", "worker.txt"))
	if err != nil {
		t.Fatalf("read worker prompt: %v", err)
	}
	if !strings.Contains(string(workerPrompt), "Squadron Merge Monitoring") {
		t.Errorf("non-merger prompt missing merge-monitoring poller block. content:\n%s", string(workerPrompt))
	}
	if strings.Contains(string(workerPrompt), "Squadron Merge Duties") {
		t.Error("non-merger prompt should not contain merger duties")
	}
}

func TestRunHeadless_PromptIncludesPersonaPreamble(t *testing.T) {
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
		Consensus:  "none",
		BaseBranch: "main",
		AutoMerge:  false,
		Agents: []squadron.SquadronAgent{
			{
				Name:    "engineer",
				Branch:  "squadron/alpha/engineer",
				Prompt:  "implement features",
				Persona: "overconfident-engineer",
			},
			{Name: "second", Branch: "squadron/alpha/second", Prompt: "support"},
		},
	}

	if _, err := squadron.RunHeadless(f, data); err != nil {
		t.Fatalf("RunHeadless: %v", err)
	}

	prompt, err := os.ReadFile(filepath.Join(f.FleetDir, "prompts", "engineer.txt"))
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	body := string(prompt)
	if !strings.Contains(body, "You are the Overconfident Engineer") {
		t.Error("expected persona preamble at top of prompt")
	}
	if !strings.Contains(body, "implement features") {
		t.Error("expected original task prompt preserved")
	}
	// Persona preamble comes before the original prompt.
	if strings.Index(body, "You are the Overconfident Engineer") > strings.Index(body, "implement features") {
		t.Error("persona preamble should appear before original prompt")
	}
}

func TestRunHeadless_UseJumpShAddsToSystemPrompt(t *testing.T) {
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
		Consensus:  "none",
		BaseBranch: "main",
		AutoMerge:  false,
		UseJumpSh:  true,
		Agents: []squadron.SquadronAgent{
			{Name: "agent", Branch: "squadron/alpha/agent", Prompt: "task"},
			{Name: "second", Branch: "squadron/alpha/second", Prompt: "support"},
		},
	}

	if _, err := squadron.RunHeadless(f, data); err != nil {
		t.Fatalf("RunHeadless: %v", err)
	}

	prompt, err := os.ReadFile(filepath.Join(f.FleetDir, "prompts", "agent.txt"))
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	if !strings.Contains(string(prompt), "jump.sh") {
		t.Errorf("UseJumpSh should inject jump.sh hint into system prompt; got:\n%s", string(prompt))
	}
}
