package cost

import "testing"

func TestDriverSource(t *testing.T) {
	tests := []struct {
		driver     string
		wantSource string
		wantOK     bool
	}{
		{"claude-code", "claude", true},
		{"codex", "codex", true},
		{"kimi-code", "kimi", true}, // reconcile exact name in Task 2
		{"aider", "", false},
		{"generic", "", false},
		{"", "claude", true}, // empty == default == claude-code
	}
	for _, tt := range tests {
		t.Run(tt.driver, func(t *testing.T) {
			gotSource, gotOK := driverSource(tt.driver)
			if gotSource != tt.wantSource || gotOK != tt.wantOK {
				t.Errorf("driverSource(%q) = (%q, %v), want (%q, %v)",
					tt.driver, gotSource, gotOK, tt.wantSource, tt.wantOK)
			}
		})
	}
}
