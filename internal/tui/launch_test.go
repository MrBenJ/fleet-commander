package tui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
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
