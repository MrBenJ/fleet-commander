package hangar

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type viewMode int

const (
	viewZen viewMode = iota
	viewLogs
)

type logEntry struct {
	timestamp string
	message   string
}

type TUIModel struct {
	url      string
	mode     viewMode
	logs     []logEntry
	quitting bool
	width    int
	height   int
}

func NewTUIModel(url string) TUIModel {
	return TUIModel{
		url:  url,
		mode: viewZen,
		logs: []logEntry{},
	}
}

type LogMsg struct {
	Message string
}

func (m TUIModel) Init() tea.Cmd {
	return nil
}

func (m TUIModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "Q":
			m.quitting = true
			return m, tea.Quit
		case "l", "L":
			if m.mode == viewZen {
				m.mode = viewLogs
			} else {
				m.mode = viewZen
			}
		case "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case LogMsg:
		m.logs = append(m.logs, logEntry{
			timestamp: time.Now().Format("15:04:05"),
			message:   msg.Message,
		})
		if len(m.logs) > 100 {
			m.logs = m.logs[len(m.logs)-100:]
		}
	}
	return m, nil
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#3fb950"))
	urlStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#58a6ff"))
	mutedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#484f58"))
	keyStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e")).
			Background(lipgloss.Color("#161b22")).
			Padding(0, 1)
)

func (m TUIModel) View() string {
	if m.quitting {
		return ""
	}

	if m.mode == viewZen {
		return m.viewZen()
	}
	return m.viewLogs()
}

func (m TUIModel) viewZen() string {
	s := "\n"
	s += "🛫 " + titleStyle.Render("Hangar is running") + "\n"
	s += urlStyle.Render(m.url) + "\n\n"
	s += mutedStyle.Render("Press ") + keyStyle.Render("L") + mutedStyle.Render(" to show logs · ") +
		keyStyle.Render("Q") + mutedStyle.Render(" to quit")
	return s
}

func (m TUIModel) viewLogs() string {
	s := "🛫 " + titleStyle.Render("Hangar") + "  " + urlStyle.Render(m.url)
	s += "    " + keyStyle.Render("L") + mutedStyle.Render(" hide logs · ") + keyStyle.Render("Q") + mutedStyle.Render(" quit") + "\n"
	s += mutedStyle.Render("─────────────────────────────────────────────────────") + "\n"

	maxLines := m.height - 4
	if maxLines < 5 {
		maxLines = 20
	}
	start := 0
	if len(m.logs) > maxLines {
		start = len(m.logs) - maxLines
	}
	for _, entry := range m.logs[start:] {
		s += mutedStyle.Render(entry.timestamp) + "  " + entry.message + "\n"
	}

	return s
}
