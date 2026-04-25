package squadron_test

import (
	"strings"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/squadron"
)

func TestBuildConsensusSuffix_None(t *testing.T) {
	got := squadron.BuildConsensusSuffix("none", "alpha", []string{"a", "b"}, "", "main")
	if got != "" {
		t.Fatalf("expected empty suffix for 'none' consensus, got %q", got)
	}
}

func TestBuildConsensusSuffix_Universal(t *testing.T) {
	got := squadron.BuildConsensusSuffix(
		"universal",
		"alpha",
		[]string{"api-refactor", "db-migration", "ui-polish"},
		"",
		"main",
	)

	mustContain := []string{
		"Squadron Consensus Protocol (UNIVERSAL)",
		`squadron "alpha"`,
		"squadron-alpha",
		"git diff main...<their-branch>",
		"Squadron members: api-refactor, db-migration, ui-polish",
		`"COMPLETED:`,
		`"APPROVED:`,
		`"CHANGES_REQUESTED:`,
		`"REVISED:`,
		"CRITICAL: Do NOT stop or exit after completing your review",
		"continue polling the squadron channel every 30 seconds",
		"MERGE_COMPLETE or MERGE_FAILED",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("suffix missing %q\n---\n%s", s, got)
		}
	}
}

func TestBuildConsensusSuffix_ReviewMaster_Reviewer(t *testing.T) {
	got := squadron.BuildReviewMasterReviewerSuffix(
		"alpha",
		[]string{"a", "b", "c"},
		"main",
	)
	mustContain := []string{
		"Squadron Consensus Protocol (REVIEW MASTER)",
		"You are the REVIEW MASTER",
		`squadron "alpha"`,
		"squadron-alpha",
		"git diff main...<their-branch>",
		`"ALL_APPROVED:`,
		"Squadron members: a, b, c",
		"CRITICAL: After posting ALL_APPROVED",
		"MERGE_COMPLETE or MERGE_FAILED",
		"Do NOT exit your session prematurely",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("reviewer suffix missing %q\n---\n%s", s, got)
		}
	}
}

func TestBuildConsensusSuffix_ReviewMaster_NonReviewer(t *testing.T) {
	got := squadron.BuildConsensusSuffix(
		"review_master",
		"alpha",
		[]string{"a", "b", "c"},
		"b", // review master
		"main",
	)
	mustContain := []string{
		"Squadron Consensus Protocol (REVIEW MASTER)",
		"You are a member of squadron",
		`Agent "b" is the designated review master`,
		"squadron-alpha",
		"Review master: b",
		"Squadron members: a, b, c",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("non-reviewer suffix missing %q\n---\n%s", s, got)
		}
	}
	// Non-reviewer suffix must NOT contain reviewer-only phrases
	forbidden := []string{"You are the REVIEW MASTER", `"ALL_APPROVED:`}
	for _, s := range forbidden {
		if strings.Contains(got, s) {
			t.Errorf("non-reviewer suffix should not contain %q", s)
		}
	}

	// Must contain CRITICAL polling instructions
	pollingChecks := []string{
		"CRITICAL: Do NOT stop after receiving approval",
		"MERGE_COMPLETE or MERGE_FAILED",
		"continue polling the squadron channel every 30 seconds",
	}
	for _, s := range pollingChecks {
		if !strings.Contains(got, s) {
			t.Errorf("non-reviewer suffix missing polling instruction %q\n---\n%s", s, got)
		}
	}
}

func TestBuildConsensusSuffix_UnknownType(t *testing.T) {
	got := squadron.BuildConsensusSuffix("bogus", "alpha", []string{"a"}, "", "main")
	if got != "" {
		t.Fatalf("expected empty suffix for unknown consensus type, got %q", got)
	}
}

func TestBuildConsensusSuffix_EmptyAgents(t *testing.T) {
	got := squadron.BuildConsensusSuffix("universal", "alpha", []string{}, "", "main")
	if !strings.Contains(got, "Squadron Consensus Protocol (UNIVERSAL)") {
		t.Error("should still produce universal suffix even with empty agents")
	}
	if !strings.Contains(got, "Squadron members: ") {
		t.Error("should contain 'Squadron members:' with empty list")
	}
}

func TestBuildConsensusSuffix_SpecialCharsInNames(t *testing.T) {
	got := squadron.BuildConsensusSuffix(
		"universal",
		"my-squad_123",
		[]string{"agent-one", "agent_two"},
		"",
		"develop",
	)
	if !strings.Contains(got, "squadron-my-squad_123") {
		t.Error("squadron channel name should preserve hyphens/underscores")
	}
	if !strings.Contains(got, "git diff develop") {
		t.Error("base branch should be 'develop'")
	}
	if !strings.Contains(got, "agent-one, agent_two") {
		t.Error("agent names should be preserved verbatim")
	}
}

func TestBuildReviewMasterReviewerSuffix_EmptyAgents(t *testing.T) {
	got := squadron.BuildReviewMasterReviewerSuffix("alpha", []string{}, "main")
	if !strings.Contains(got, "You are the REVIEW MASTER") {
		t.Error("should still produce reviewer suffix with empty agents")
	}
	if !strings.Contains(got, "Squadron members: ") {
		t.Error("should contain 'Squadron members:' with empty list")
	}
}

func TestBuildConsensusSuffix_ReviewMasterSingleAgent(t *testing.T) {
	got := squadron.BuildConsensusSuffix(
		"review_master",
		"solo",
		[]string{"only-one"},
		"only-one",
		"main",
	)
	if !strings.Contains(got, `Agent "only-one" is the designated review master`) {
		t.Error("review master name should appear in non-reviewer suffix")
	}
	if !strings.Contains(got, "Review master: only-one") {
		t.Error("review master footer should be present")
	}
}

func TestBuildMergerSuffix_EmptyAgents(t *testing.T) {
	got := squadron.BuildMergerSuffix("alpha", "main", nil, false)
	if !strings.Contains(got, "Squadron Merge Duties") {
		t.Error("should still produce merger suffix even with no agents")
	}
	if !strings.Contains(got, "MERGE MASTER") {
		t.Error("should contain MERGE MASTER header")
	}
}

func TestBuildMergerSuffix_SingleAgent(t *testing.T) {
	agents := []squadron.AgentBranch{
		{Name: "solo", Branch: "squadron/alpha/solo"},
	}
	got := squadron.BuildMergerSuffix("alpha", "develop", agents, false)
	if !strings.Contains(got, "git worktree add -b squadron/alpha-merged ../alpha-merged develop") {
		t.Error("base branch 'develop' should appear as the start point of the integration worktree")
	}
	if !strings.Contains(got, "solo -> squadron/alpha/solo") {
		t.Error("single agent branch should appear")
	}
}

func TestBuildMergerSuffix(t *testing.T) {
	agents := []squadron.AgentBranch{
		{Name: "a", Branch: "squadron/alpha/a"},
		{Name: "b", Branch: "squadron/alpha/b"},
		{Name: "c", Branch: "squadron/alpha/c"},
	}
	got := squadron.BuildMergerSuffix("alpha", "main", agents, false)

	mustContain := []string{
		"Squadron Merge Duties",
		"MERGE MASTER",
		`squadron "alpha"`,
		"git worktree add -b squadron/alpha-merged ../alpha-merged main",
		"cd ../alpha-merged",
		"git merge --no-ff",
		"a -> squadron/alpha/a",
		"b -> squadron/alpha/b",
		"c -> squadron/alpha/c",
		`"MERGE_COMPLETE: squadron/alpha-merged"`,
		`"MERGE_FAILED:`,
		"CRITICAL: Before starting the merge, verify ALL agents have reached consensus",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("merger suffix missing %q\n---\n%s", s, got)
		}
	}

	// Ordering: all three agents present in array order (including merger's own — caller decides who's merger)
	aIdx := strings.Index(got, "a -> squadron/alpha/a")
	bIdx := strings.Index(got, "b -> squadron/alpha/b")
	cIdx := strings.Index(got, "c -> squadron/alpha/c")
	if !(aIdx < bIdx && bIdx < cIdx) {
		t.Errorf("merge order not preserved: a=%d b=%d c=%d", aIdx, bIdx, cIdx)
	}
}

func TestBuildNoConsensusAutoMergeSuffix(t *testing.T) {
	got := squadron.BuildNoConsensusAutoMergeSuffix("alpha")

	mustContain := []string{
		"Squadron Merge Monitoring",
		`squadron "alpha"`,
		"squadron-alpha",
		"CRITICAL: Do NOT stop after completing your work",
		"MERGE_COMPLETE or MERGE_FAILED",
		"continue polling the squadron channel every 30 seconds",
		`fleet context channel-read squadron-alpha`,
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("no-consensus auto-merge suffix missing %q\n---\n%s", s, got)
		}
	}
}

func TestBuildConsensusSuffix_NoneStillEmpty(t *testing.T) {
	// Verify that "none" consensus still returns empty string —
	// the auto-merge polling is handled separately by the caller.
	got := squadron.BuildConsensusSuffix("none", "alpha", []string{"a", "b"}, "", "main")
	if got != "" {
		t.Fatalf("expected empty suffix for 'none' consensus, got %q", got)
	}
}

func TestBuildMergerSuffix_WithAutoPR(t *testing.T) {
	agents := []squadron.AgentBranch{
		{Name: "a", Branch: "squadron/alpha/a"},
		{Name: "b", Branch: "squadron/alpha/b"},
	}
	got := squadron.BuildMergerSuffix("alpha", "main", agents, true)

	// Must contain standard merger content
	if !strings.Contains(got, "Squadron Merge Duties") {
		t.Error("should contain merger duties header")
	}

	// Must contain autoPR-specific instructions
	mustContain := []string{
		"git push -u origin squadron/alpha",
		"gh pr create",
		"Squadron alpha:",
		"--base main",
		"gh pr checks",
		`"PR_READY:`,
		`"PR_BLOCKED:`,
		"squadron-alpha",
		"You are NOT done until the PR exists and CI is passing",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("autoPR merger suffix missing %q\n---\n%s", s, got)
		}
	}
}

func TestBuildMergerSuffix_WithAutoPR_ContainsAuthCheck(t *testing.T) {
	agents := []squadron.AgentBranch{
		{Name: "a", Branch: "squadron/alpha/a"},
	}
	got := squadron.BuildMergerSuffix("alpha", "main", agents, true)

	mustContain := []string{
		"gh auth status",
		"GH_AUTH_FAILED",
		"fleet context channel-send squadron-alpha",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("autoPR merger suffix missing auth check content %q\n---\n%s", s, got)
		}
	}
}

func TestBuildMergerSuffix_WithoutAutoPR(t *testing.T) {
	agents := []squadron.AgentBranch{
		{Name: "a", Branch: "squadron/alpha/a"},
	}
	got := squadron.BuildMergerSuffix("alpha", "main", agents, false)

	// Must NOT contain autoPR-specific instructions
	forbidden := []string{
		"gh pr create",
		"PR_READY:",
		"PR_BLOCKED:",
		"gh pr checks",
	}
	for _, s := range forbidden {
		if strings.Contains(got, s) {
			t.Errorf("non-autoPR merger suffix should not contain %q", s)
		}
	}
}

func TestBuildMergerSuffix_CreatesIntegrationWorktree(t *testing.T) {
	agents := []squadron.AgentBranch{
		{Name: "a", Branch: "squadron/alpha/a"},
		{Name: "b", Branch: "squadron/alpha/b"},
	}
	got := squadron.BuildMergerSuffix("alpha", "main", agents, false)

	mustContain := []string{
		// New behavior: merger creates a dedicated worktree for integration
		"git worktree add",
		"squadron/alpha-merged",
		"../alpha-merged",
		// The base branch is the start point of the new worktree
		"git worktree add -b squadron/alpha-merged ../alpha-merged main",
		// Status messages reference the new branch name
		`"MERGE_COMPLETE: squadron/alpha-merged"`,
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("merger suffix missing %q\n---\n%s", s, got)
		}
	}

	// Old single-branch checkout pattern must be gone.
	forbidden := []string{
		"git checkout -b squadron/alpha\n",
	}
	for _, s := range forbidden {
		if strings.Contains(got, s) {
			t.Errorf("merger suffix still contains old pattern %q", s)
		}
	}
}

func TestBuildMergerSuffix_WorktreeNameInterpolation(t *testing.T) {
	// Squadron name with hyphens and underscores must be substituted verbatim.
	got := squadron.BuildMergerSuffix("my-trio_v2", "develop", nil, false)

	mustContain := []string{
		"squadron/my-trio_v2-merged",
		"../my-trio_v2-merged",
		"git worktree add -b squadron/my-trio_v2-merged ../my-trio_v2-merged develop",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("merger suffix missing %q for squadron 'my-trio_v2'\n---\n%s", s, got)
		}
	}
}

func TestBuildMergerSuffix_AutoPRPushesMergedBranch(t *testing.T) {
	agents := []squadron.AgentBranch{
		{Name: "a", Branch: "squadron/alpha/a"},
	}
	got := squadron.BuildMergerSuffix("alpha", "main", agents, true)

	mustContain := []string{
		// AutoPR pushes the new merged branch, not the old squadron/<name> branch
		"git push -u origin squadron/alpha-merged",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("autoPR merger suffix missing %q\n---\n%s", s, got)
		}
	}
}

func TestBuildNoPRForNonMergerSuffix(t *testing.T) {
	got := squadron.BuildNoPRForNonMergerSuffix("alpha")

	mustContain := []string{
		"DO NOT create a pull request",
		"gh pr create",
		"squadron/alpha",
		"merge master",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("non-merger no-PR suffix missing %q\n---\n%s", s, got)
		}
	}
}
