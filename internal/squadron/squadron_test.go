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

func TestBuildMergerSuffix(t *testing.T) {
	agents := []squadron.AgentBranch{
		{Name: "a", Branch: "squadron/alpha/a"},
		{Name: "b", Branch: "squadron/alpha/b"},
		{Name: "c", Branch: "squadron/alpha/c"},
	}
	got := squadron.BuildMergerSuffix("alpha", "main", agents)

	mustContain := []string{
		"Squadron Merge Duties",
		"MERGE MASTER",
		`squadron "alpha"`,
		"git checkout main",
		"git checkout -b squadron/alpha",
		"git merge --no-ff",
		"a -> squadron/alpha/a",
		"b -> squadron/alpha/b",
		"c -> squadron/alpha/c",
		`"MERGE_COMPLETE: squadron/alpha"`,
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
