package squadron_test

import (
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/squadron"
)

func TestBuildConsensusSuffix_None(t *testing.T) {
	got := squadron.BuildConsensusSuffix("none", "alpha", []string{"a", "b"}, "", "main")
	if got != "" {
		t.Fatalf("expected empty suffix for 'none' consensus, got %q", got)
	}
}
