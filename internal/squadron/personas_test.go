package squadron_test

import (
	"strings"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/squadron"
)

func TestLookupPersona_Unknown(t *testing.T) {
	_, ok := squadron.LookupPersona("no-such-thing")
	if ok {
		t.Fatal("expected unknown persona to return ok=false")
	}
}

func TestApplyPersona_PrependsPreamble(t *testing.T) {
	p := squadron.Persona{
		Name:        "test",
		DisplayName: "Test",
		Preamble:    "You are Test.",
	}
	got := squadron.ApplyPersona(p, "ORIGINAL PROMPT")

	if !strings.HasPrefix(got, "You are Test.") {
		t.Errorf("persona preamble should be at the top, got: %q", got[:30])
	}
	if !strings.Contains(got, "ORIGINAL PROMPT") {
		t.Error("original prompt should be preserved")
	}
	if !strings.Contains(got, "\n---\n") {
		t.Error("preamble and prompt should be separated by a --- divider")
	}
	if strings.Index(got, "You are Test.") > strings.Index(got, "ORIGINAL PROMPT") {
		t.Error("preamble should come before the original prompt")
	}
}
