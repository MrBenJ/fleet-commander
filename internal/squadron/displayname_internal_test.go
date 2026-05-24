package squadron

import "testing"

func TestResolveDisplayName(t *testing.T) {
	tests := []struct {
		name  string
		agent SquadronAgent
		want  string
	}{
		{"explicit display name", SquadronAgent{Name: "alex-slug", DisplayName: "Alex"}, "Alex"},
		{"empty falls back to slug", SquadronAgent{Name: "alex-slug"}, "alex-slug"},
		{"whitespace falls back to slug", SquadronAgent{Name: "alex-slug", DisplayName: "   "}, "alex-slug"},
		{"trims surrounding space", SquadronAgent{Name: "alex-slug", DisplayName: "  Alex  "}, "Alex"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveDisplayName(tt.agent); got != tt.want {
				t.Errorf("resolveDisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}
