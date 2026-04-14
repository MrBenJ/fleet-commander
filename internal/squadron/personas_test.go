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

func TestBuiltInPersonas(t *testing.T) {
	want := []string{
		"overconfident-engineer",
		"zen-master",
		"paranoid-perfectionist",
		"raging-jerk",
		"peter-molyneux",
	}
	for _, key := range want {
		p, ok := squadron.LookupPersona(key)
		if !ok {
			t.Errorf("persona %q not registered", key)
			continue
		}
		if p.Name != key {
			t.Errorf("persona %q has Name=%q", key, p.Name)
		}
		if p.DisplayName == "" {
			t.Errorf("persona %q missing DisplayName", key)
		}
		if len(p.Preamble) < 100 {
			t.Errorf("persona %q preamble too short (%d bytes)", key, len(p.Preamble))
		}
	}
}
