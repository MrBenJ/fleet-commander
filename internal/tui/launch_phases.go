package tui

import (
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
)

// InputPhase holds state for launchModeInput and launchModeYoloConfirm.
// The textarea captures the user's task descriptions; pendingYoloInput
// saves the text between the first Ctrl+D and the YOLO confirmation.
type InputPhase struct {
	area             textarea.Model
	pendingYoloInput string
}

// GeneratingPhase holds state for launchModeGenerating.
// Just a spinner — Claude CLI is doing the real work.
type GeneratingPhase struct {
	spinner spinner.Model
}

// ReviewPhase holds state for launchModeReview, launchModeEditName,
// launchModeEditBranch, and launchModeEditPrompt.
// The viewport displays the current prompt, and the edit inputs allow
// inline modification of agent name, branch, and prompt text.
type ReviewPhase struct {
	nameInput   textinput.Model
	branchInput textinput.Model
	promptEdit  textarea.Model
	viewport    viewport.Model
	viewportIdx int // tracks which prompt index the viewport was built for
}

// SquadronPhase holds state for launchModeSquadronConsensus and
// launchModeSquadronName. The consensus cursor navigates the selector
// list, and nameInput captures the squadron name.
type SquadronPhase struct {
	consensusCursor int
	nameInput       textinput.Model
}
