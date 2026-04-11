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
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("suffix missing %q\n---\n%s", s, got)
		}
	}
}
