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

func TestApplyPersona_FramesIdentityAbovePreamble(t *testing.T) {
	p := squadron.Persona{
		Name:        "test",
		DisplayName: "Test Persona",
		Preamble:    "Persona preamble body.",
	}
	got := squadron.ApplyPersona(p, "Alex", "ORIGINAL PROMPT")

	if !strings.HasPrefix(got, "## Your Identity") {
		t.Errorf("identity block should be at the very top, got: %q", got[:40])
	}
	if !strings.Contains(got, "Your name is Alex") {
		t.Error("identity block should name the agent")
	}
	if !strings.Contains(got, "Never say your name is Test Persona") {
		t.Error("identity block should forbid claiming the persona's name")
	}
	if !strings.Contains(got, "Persona preamble body.") {
		t.Error("persona preamble should be preserved")
	}
	if !strings.Contains(got, "ORIGINAL PROMPT") {
		t.Error("original prompt should be preserved")
	}
	if strings.Index(got, "## Your Identity") > strings.Index(got, "Persona preamble body.") {
		t.Error("identity block should come before the persona preamble")
	}
	if strings.Index(got, "Persona preamble body.") > strings.Index(got, "ORIGINAL PROMPT") {
		t.Error("persona preamble should come before the original prompt")
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
