package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/teknal/fleet-commander/internal/fleet"
)

// helper to create a LaunchModel with minimal state for state-machine tests.
func testLaunchModel(yoloMode, skipConfirm bool) LaunchModel {
	m := LaunchModel{
		fleet:           &fleet.Fleet{},
		mode:            launchModeInput,
		yoloMode:        yoloMode,
		skipYoloConfirm: skipConfirm,
	}
	m.inputArea = newTestTextarea("Fix the login bug")
	return m
}

func newTestTextarea(value string) textarea.Model {
	ta := textarea.New()
	ta.SetValue(value)
	return ta
}

func TestYoloModeCtrlDShowsConfirmation(t *testing.T) {
	m := testLaunchModel(true, false)

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeYoloConfirm {
		t.Errorf("expected mode launchModeYoloConfirm (%d), got %d", launchModeYoloConfirm, updated.mode)
	}
	if updated.pendingYoloInput == "" {
		t.Error("expected pendingYoloInput to be set, got empty string")
	}
}

func TestYoloModeSkipConfirmGoesDirectToGenerating(t *testing.T) {
	m := testLaunchModel(true, true)

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeGenerating {
		t.Errorf("expected mode launchModeGenerating (%d), got %d", launchModeGenerating, updated.mode)
	}
}

func TestNonYoloModeCtrlDGoesDirectToGenerating(t *testing.T) {
	m := testLaunchModel(false, false)

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeGenerating {
		t.Errorf("expected mode launchModeGenerating (%d), got %d", launchModeGenerating, updated.mode)
	}
}

func TestYoloConfirmEscReturnsToInput(t *testing.T) {
	m := testLaunchModel(true, false)
	m.mode = launchModeYoloConfirm
	m.pendingYoloInput = "Fix the login bug"

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeInput {
		t.Errorf("expected mode launchModeInput (%d), got %d", launchModeInput, updated.mode)
	}
}

func TestYoloConfirmCtrlDProceeds(t *testing.T) {
	m := testLaunchModel(true, false)
	m.mode = launchModeYoloConfirm
	m.pendingYoloInput = "Fix the login bug"

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeGenerating {
		t.Errorf("expected mode launchModeGenerating (%d), got %d", launchModeGenerating, updated.mode)
	}
}

func TestYoloConfirmIgnoresOtherKeys(t *testing.T) {
	m := testLaunchModel(true, false)
	m.mode = launchModeYoloConfirm
	m.pendingYoloInput = "Fix the login bug"

	for _, key := range []string{"a", "enter", "l", "y"} {
		msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
		result, _ := m.Update(msg)
		updated := result.(LaunchModel)

		if updated.mode != launchModeYoloConfirm {
			t.Errorf("key %q changed mode to %d, expected to stay at launchModeYoloConfirm (%d)", key, updated.mode, launchModeYoloConfirm)
		}
	}
}

func TestEmptyInputCtrlDDoesNotProceed(t *testing.T) {
	m := LaunchModel{
		mode:     launchModeInput,
		yoloMode: true,
	}
	m.inputArea = newTestTextarea("")

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeInput {
		t.Errorf("expected to stay in launchModeInput (%d), got %d", launchModeInput, updated.mode)
	}
	if updated.statusMsg == "" {
		t.Error("expected a status message about empty input")
	}
}

// --- Review and Edit mode tests ---

func testLaunchModelInReview() LaunchModel {
	m := LaunchModel{
		fleet: &fleet.Fleet{},
		mode:  launchModeReview,
		prompts: []LaunchItem{
			{Prompt: "Fix login bug", AgentName: "fix-login", Branch: "fleet/fix-login"},
			{Prompt: "Add OAuth", AgentName: "add-oauth", Branch: "fleet/add-oauth"},
		},
		currentIdx:  0,
		nameInput:   textinput.New(),
		branchInput: textinput.New(),
		promptEdit:  textarea.New(),
	}
	return m
}

// Review mode tests

func TestReview_S_SkipsAndAdvances(t *testing.T) {
	m := testLaunchModelInReview()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.skipped != 1 {
		t.Errorf("expected skipped=1, got %d", updated.skipped)
	}
	if updated.currentIdx != 1 {
		t.Errorf("expected currentIdx=1, got %d", updated.currentIdx)
	}
}

func TestReview_S_LastItemQuits(t *testing.T) {
	m := testLaunchModelInReview()
	m.currentIdx = len(m.prompts) - 1

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if !updated.quitting {
		t.Error("expected quitting=true when skipping last item")
	}
}

func TestReview_E_EntersEditName(t *testing.T) {
	m := testLaunchModelInReview()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeEditName {
		t.Errorf("expected mode launchModeEditName (%d), got %d", launchModeEditName, updated.mode)
	}
}

func TestReview_A_Aborts(t *testing.T) {
	m := testLaunchModelInReview()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if !updated.aborted {
		t.Error("expected aborted=true")
	}
	if !updated.quitting {
		t.Error("expected quitting=true")
	}
}

// Edit name mode tests

func TestEditName_Enter_MovesToBranch(t *testing.T) {
	m := testLaunchModelInReview()
	m.mode = launchModeEditName
	m.nameInput = textinput.New()
	m.nameInput.SetValue("new-name")
	m.branchInput = textinput.New()

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeEditBranch {
		t.Errorf("expected mode launchModeEditBranch (%d), got %d", launchModeEditBranch, updated.mode)
	}
	if updated.prompts[0].AgentName != "new-name" {
		t.Errorf("expected agent name 'new-name', got %q", updated.prompts[0].AgentName)
	}
}

func TestEditName_EmptyRejected(t *testing.T) {
	m := testLaunchModelInReview()
	m.mode = launchModeEditName
	m.nameInput = textinput.New()
	m.nameInput.SetValue("")

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeEditName {
		t.Errorf("expected to stay in launchModeEditName (%d), got %d", launchModeEditName, updated.mode)
	}
	if updated.statusMsg == "" {
		t.Error("expected a status message about empty name")
	}
}

func TestEditName_Esc_GoesBackToReview(t *testing.T) {
	m := testLaunchModelInReview()
	m.mode = launchModeEditName
	m.nameInput = textinput.New()

	msg := tea.KeyMsg{Type: tea.KeyEsc}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeReview {
		t.Errorf("expected mode launchModeReview (%d), got %d", launchModeReview, updated.mode)
	}
}

// Edit branch mode tests

func TestEditBranch_Enter_MovesToPrompt(t *testing.T) {
	m := testLaunchModelInReview()
	m.mode = launchModeEditBranch
	m.branchInput = textinput.New()
	m.branchInput.SetValue("fleet/new-branch")
	m.promptEdit = textarea.New()

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeEditPrompt {
		t.Errorf("expected mode launchModeEditPrompt (%d), got %d", launchModeEditPrompt, updated.mode)
	}
	if updated.prompts[0].Branch != "fleet/new-branch" {
		t.Errorf("expected branch 'fleet/new-branch', got %q", updated.prompts[0].Branch)
	}
}

func TestEditBranch_EmptyRejected(t *testing.T) {
	m := testLaunchModelInReview()
	m.mode = launchModeEditBranch
	m.branchInput = textinput.New()
	m.branchInput.SetValue("")

	msg := tea.KeyMsg{Type: tea.KeyEnter}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeEditBranch {
		t.Errorf("expected to stay in launchModeEditBranch (%d), got %d", launchModeEditBranch, updated.mode)
	}
	if updated.statusMsg == "" {
		t.Error("expected a status message about empty branch")
	}
}

// Edit prompt mode tests

func TestEditPrompt_CtrlD_Confirms(t *testing.T) {
	m := testLaunchModelInReview()
	m.mode = launchModeEditPrompt
	m.promptEdit = textarea.New()
	m.promptEdit.SetValue("Updated prompt text")

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeReview {
		t.Errorf("expected mode launchModeReview (%d), got %d", launchModeReview, updated.mode)
	}
	if updated.prompts[0].Prompt != "Updated prompt text" {
		t.Errorf("expected prompt 'Updated prompt text', got %q", updated.prompts[0].Prompt)
	}
}

func TestEditPrompt_EmptyRejected(t *testing.T) {
	m := testLaunchModelInReview()
	m.mode = launchModeEditPrompt
	m.promptEdit = textarea.New()
	m.promptEdit.SetValue("   ")

	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	result, _ := m.Update(msg)
	updated := result.(LaunchModel)

	if updated.mode != launchModeEditPrompt {
		t.Errorf("expected to stay in launchModeEditPrompt (%d), got %d", launchModeEditPrompt, updated.mode)
	}
	if updated.statusMsg == "" {
		t.Error("expected a status message about empty prompt")
	}
}
