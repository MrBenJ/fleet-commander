package tui

import "testing"

func TestSetupPromptViewportUsesMinimumUsableDimensions(t *testing.T) {
	m := LaunchModel{
		mode:       launchModeReview,
		currentIdx: 0,
		width:      10,
		height:     8,
		prompts: []LaunchItem{{
			AgentName: "alpha",
			Prompt:    "Do the thing, then the other thing, and keep going long enough to wrap.",
		}},
	}

	m.setupPromptViewport()

	if m.review.viewport.Width != 20 {
		t.Fatalf("viewport width = %d, want 20 minimum", m.review.viewport.Width)
	}
	if m.review.viewport.Height != 3 {
		t.Fatalf("viewport height = %d, want 3 minimum", m.review.viewport.Height)
	}
	if m.review.viewportIdx != 0 {
		t.Fatalf("viewportIdx = %d, want 0", m.review.viewportIdx)
	}
}
