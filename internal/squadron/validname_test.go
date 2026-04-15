package squadron_test

import (
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/squadron"
)

func TestValidName(t *testing.T) {
	valid := []string{
		"a",
		"abc",
		"agent-1",
		"agent_2",
		"A1",
		"CamelCase",
		"123",
		"a-b_c",
	}
	for _, name := range valid {
		if !squadron.ValidName(name) {
			t.Errorf("ValidName(%q) = false, want true", name)
		}
	}

	invalid := []string{
		"",
		"-starts-with-dash",
		"_starts-with-underscore",
		"has spaces",
		"has.period",
		"has/slash",
		"has@symbol",
		"emoji🎉",
		// 31 chars — exceeds 30 char limit
		"abcdefghijklmnopqrstuvwxyz12345",
	}
	for _, name := range invalid {
		if squadron.ValidName(name) {
			t.Errorf("ValidName(%q) = true, want false", name)
		}
	}
}

func TestValidName_ExactlyThirtyChars(t *testing.T) {
	name := "abcdefghijklmnopqrstuvwxyz1234" // exactly 30
	if !squadron.ValidName(name) {
		t.Errorf("ValidName(%q) should accept exactly 30 chars", name)
	}
}
