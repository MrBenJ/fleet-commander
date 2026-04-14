package hangar

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTUIZenView(t *testing.T) {
	m := NewTUIModel("http://localhost:4242")
	view := m.View()

	if !strings.Contains(view, "Hangar is running") {
		t.Error("zen view should contain 'Hanger is running'")
	}
	if !strings.Contains(view, "http://localhost:4242") {
		t.Error("zen view should contain the URL")
	}
}

func TestTUIToggleLogs(t *testing.T) {
	m := NewTUIModel("http://localhost:4242")

	if m.mode != viewZen {
		t.Fatal("should start in zen mode")
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(TUIModel)
	if m.mode != viewLogs {
		t.Fatal("should switch to log mode after pressing L")
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(TUIModel)
	if m.mode != viewZen {
		t.Fatal("should switch back to zen mode")
	}
}

func TestTUIQuit(t *testing.T) {
	m := NewTUIModel("http://localhost:4242")

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m = updated.(TUIModel)

	if !m.quitting {
		t.Fatal("should be quitting")
	}
	if cmd == nil {
		t.Fatal("should return quit command")
	}
}

func TestTUILogMessages(t *testing.T) {
	m := NewTUIModel("http://localhost:4242")

	updated, _ := m.Update(LogMsg{Message: "Server started"})
	m = updated.(TUIModel)

	if len(m.logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(m.logs))
	}
	if m.logs[0].message != "Server started" {
		t.Fatalf("wrong message: %s", m.logs[0].message)
	}

	// Switch to log view and check rendering
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	m = updated.(TUIModel)
	view := m.View()

	if !strings.Contains(view, "Server started") {
		t.Error("log view should show log messages")
	}
}
