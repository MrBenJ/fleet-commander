package squadron_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
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

func TestRunHeadless_DisplayNameAndPersonaFraming(t *testing.T) {
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
			{Name: "alex-slug", DisplayName: "Alex", Branch: "squadron/alpha/alex-slug", Prompt: "do work", Persona: "peter-molyneux"},
			{Name: "second", Branch: "squadron/alpha/second", Prompt: "support"},
		},
	}

	if _, err := squadron.RunHeadless(f, data); err != nil {
		t.Fatalf("RunHeadless: %v", err)
	}

	body, err := os.ReadFile(filepath.Join(f.FleetDir, "prompts", "alex-slug.txt"))
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	content := string(body)

	if !strings.Contains(content, "You are: Alex (coordination handle: alex-slug") {
		t.Errorf("prompt should headline the display name with the slug as handle, got:\n%s", content)
	}
	if !strings.Contains(content, "Your name is Alex") {
		t.Error("prompt missing identity framing naming the agent")
	}
	if !strings.Contains(content, "Never say your name is Peter Molyneux") {
		t.Error("prompt should forbid claiming the persona's name")
	}
	if !strings.Contains(content, "| alex-slug |") {
		t.Error("agent table should still address the slug, not the display name")
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

// Regression: CreateChannel used to auto-rename 2-member channels to
// dm-[a]-[b], leaving 2-agent squadron prompts pointing at a channel that
// didn't exist. Explicit names are now always honored, so a 2-agent squadron
// gets squadron-<name> exactly like larger squadrons.
func TestRunHeadless_TwoAgentSquadron_PromptsReferenceActualChannel(t *testing.T) {
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

	mergerName := "duo-a"
	data := &squadron.SquadronData{
		Name:        "duo",
		Consensus:   "universal",
		BaseBranch:  "main",
		AutoMerge:   true,
		AutoPR:      true,
		MergeMaster: &mergerName,
		Agents: []squadron.SquadronAgent{
			{Name: "duo-a", Branch: "squadron/duo/duo-a", Prompt: "do a"},
			{Name: "duo-b", Branch: "squadron/duo/duo-b", Prompt: "do b"},
		},
	}

	if _, err := squadron.RunHeadless(f, data); err != nil {
		t.Fatalf("RunHeadless: %v", err)
	}

	// 2-agent squadrons get squadron-<name>, exactly like larger squadrons.
	wantChannel := "squadron-duo"
	ctx, err := fleetctx.Load(f.FleetDir)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if _, ok := ctx.Channels[wantChannel]; !ok {
		t.Fatalf("expected channel %q to exist, channels: %v", wantChannel, ctx.Channels)
	}
	if _, ok := ctx.Channels["dm-[duo-a]-[duo-b]"]; ok {
		t.Fatalf("dm-style channel should not exist for a 2-agent squadron")
	}

	for _, name := range []string{"duo-a", "duo-b"} {
		b, err := os.ReadFile(filepath.Join(f.FleetDir, "prompts", name+".txt"))
		if err != nil {
			t.Fatalf("read prompt %s: %v", name, err)
		}
		content := string(b)
		if !strings.Contains(content, "channel-send "+wantChannel) {
			t.Errorf("prompt %s does not send to the channel that exists (%s)\n---\n%s", name, wantChannel, content)
		}
		if !strings.Contains(content, "channel-read "+wantChannel) {
			t.Errorf("prompt %s does not read the channel that exists (%s)", name, wantChannel)
		}
		if strings.Contains(content, "dm-[") {
			t.Errorf("prompt %s references a retired dm- channel name\n---\n%s", name, content)
		}
	}
}

// 3+ agent squadrons keep the squadron-<name> channel; no regression from the
// 2-agent fix.
func TestRunHeadless_ThreeAgentSquadron_UsesSquadronChannel(t *testing.T) {
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
		Name:       "trio",
		Consensus:  "universal",
		BaseBranch: "main",
		AutoMerge:  false,
		Agents: []squadron.SquadronAgent{
			{Name: "tri-a", Branch: "squadron/trio/tri-a", Prompt: "do a"},
			{Name: "tri-b", Branch: "squadron/trio/tri-b", Prompt: "do b"},
			{Name: "tri-c", Branch: "squadron/trio/tri-c", Prompt: "do c"},
		},
	}

	if _, err := squadron.RunHeadless(f, data); err != nil {
		t.Fatalf("RunHeadless: %v", err)
	}

	ctx, err := fleetctx.Load(f.FleetDir)
	if err != nil {
		t.Fatalf("load context: %v", err)
	}
	if _, ok := ctx.Channels["squadron-trio"]; !ok {
		t.Fatalf("expected channel squadron-trio to exist, channels: %v", ctx.Channels)
	}

	for _, name := range []string{"tri-a", "tri-b", "tri-c"} {
		b, err := os.ReadFile(filepath.Join(f.FleetDir, "prompts", name+".txt"))
		if err != nil {
			t.Fatalf("read prompt %s: %v", name, err)
		}
		content := string(b)
		if !strings.Contains(content, "channel-send squadron-trio") {
			t.Errorf("prompt %s should reference squadron-trio channel\n---\n%s", name, content)
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

// Regression: a non-default driver on a SquadronAgent must propagate to the
// generated launcher script and to the persisted fleet config. Before the fix,
// the agent was added with an empty Driver field, GetForAgent() fell back to
// claude-code, and the launcher was written with `exec claude` even when the
// user had selected a different harness in the wizard.
func TestRunHeadless_NonDefaultDriverPropagates(t *testing.T) {
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
		Name:       "kimi-test",
		Consensus:  "none",
		BaseBranch: "main",
		AutoMerge:  false,
		Agents: []squadron.SquadronAgent{
			{Name: "kimi1", Branch: "squadron/kimi-test/kimi1", Prompt: "do kimi work", Driver: "kimi-code"},
			{Name: "kimi2", Branch: "squadron/kimi-test/kimi2", Prompt: "do other kimi work", Driver: "kimi-code"},
		},
	}

	if _, err := squadron.RunHeadless(f, data); err != nil {
		t.Fatalf("RunHeadless: %v", err)
	}

	launcherPath := filepath.Join(f.FleetDir, "prompts", "kimi1.sh")
	b, err := os.ReadFile(launcherPath)
	if err != nil {
		t.Fatalf("read launcher %s: %v", launcherPath, err)
	}
	script := string(b)
	if !strings.Contains(script, "exec kimi") {
		t.Errorf("launcher should invoke kimi, got: %q", script)
	}
	if strings.Contains(script, "exec claude") {
		t.Errorf("launcher should NOT invoke claude for a kimi-code agent, got: %q", script)
	}

	agent, err := f.GetAgent("kimi1")
	if err != nil {
		t.Fatalf("GetAgent: %v", err)
	}
	if agent.Driver != "kimi-code" {
		t.Errorf("expected persisted driver 'kimi-code', got %q", agent.Driver)
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
