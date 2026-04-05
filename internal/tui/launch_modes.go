package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m LaunchModel) updateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+d":
			input := m.inputArea.Value()
			if strings.TrimSpace(input) == "" {
				m.statusMsg = "No prompts found. Enter at least one task."
				return m, nil
			}

			// In yolo mode, show confirmation unless --i-know-what-im-doing
			if m.yoloMode && !m.skipYoloConfirm {
				m.pendingYoloInput = input
				m.mode = launchModeYoloConfirm
				m.statusMsg = ""
				return m, nil
			}

			return m.submitInput(input)

		case "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.inputArea, cmd = m.inputArea.Update(msg)
	return m, cmd
}

// submitInput starts the generation phase with the given input.
func (m LaunchModel) submitInput(input string) (tea.Model, tea.Cmd) {
	// Collect existing agent names for deduplication
	var existingNames []string
	for _, a := range m.fleet.Agents {
		existingNames = append(existingNames, a.Name)
	}

	m.mode = launchModeGenerating
	m.statusMsg = ""

	m.log.Log("submitInput: input_len=%d existing_agents=%d", len(input), len(existingNames))

	// Launch async Claude CLI call alongside the spinner
	log := m.log // capture for closure
	claudeCmd := func() tea.Msg {
		items, err := GenerateWithClaude(input, existingNames, log)
		return claudeResultMsg{items: items, err: err}
	}
	return m, tea.Batch(m.spinner.Tick, claudeCmd)
}

func (m LaunchModel) updateYoloConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+d":
			return m.submitInput(m.pendingYoloInput)
		case "esc", "ctrl+c":
			m.mode = launchModeInput
			m.statusMsg = ""
			m.inputArea.Focus()
			return m, nil
		}
	}
	return m, nil
}

func (m LaunchModel) updateGenerating(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// setupPromptViewport initializes or updates the prompt viewport for the current prompt.
func (m *LaunchModel) setupPromptViewport() {
	if m.promptViewportIdx == m.currentIdx && m.promptViewport.Width > 0 {
		return // already set up for this prompt
	}

	// Reserve space: 2 indent + label lines above + actions below
	contentWidth := m.width - 6
	if contentWidth < 20 {
		contentWidth = 20
	}

	// Word-wrap the prompt text to fit
	wrapped := lipgloss.NewStyle().Width(contentWidth).Render(m.prompts[m.currentIdx].Prompt)

	// Calculate viewport height: use remaining space, capped
	// Header(2) + agent(1) + branch(1) + "Prompt:" label(1) + blank(1) + actions(1) + status/launched(~4) = ~11 lines overhead
	vpHeight := m.height - 11
	if vpHeight < 3 {
		vpHeight = 3
	}
	// No upper cap — use all available terminal height

	vp := viewport.New(contentWidth, vpHeight)
	vp.Style = lipgloss.NewStyle().PaddingLeft(4)
	vp.SetContent(wrapped)

	m.promptViewport = vp
	m.promptViewportIdx = m.currentIdx
}

func (m LaunchModel) updateReview(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Ensure viewport is set up for the current prompt
	m.setupPromptViewport()

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "l", "L":
			return m.launchCurrent()

		case "e", "E":
			item := m.prompts[m.currentIdx]
			m.nameInput.SetValue(item.AgentName)
			m.nameInput.Focus()
			m.mode = launchModeEditName
			m.statusMsg = ""
			return m, m.nameInput.Focus()

		case "s", "S":
			m.skipped++
			m.statusMsg = ""
			return m.advance()

		case "a", "A", "esc", "ctrl+c":
			m.aborted = true
			m.quitting = true
			return m, tea.Quit

		// Scroll the prompt viewport
		case "j", "down":
			m.promptViewport.LineDown(1)
			return m, nil
		case "k", "up":
			m.promptViewport.LineUp(1)
			return m, nil
		case "h", "left":
			// no-op for horizontal (content is wrapped)
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.promptViewport, cmd = m.promptViewport.Update(msg)
	return m, cmd
}

func (m LaunchModel) updateEditName(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			val := m.nameInput.Value()
			if val == "" {
				m.statusMsg = "Name cannot be empty"
				return m, nil
			}
			m.prompts[m.currentIdx].AgentName = val
			// Move to branch edit
			m.branchInput.SetValue(m.prompts[m.currentIdx].Branch)
			m.branchInput.Focus()
			m.mode = launchModeEditBranch
			m.statusMsg = ""
			return m, m.branchInput.Focus()

		case "esc":
			m.mode = launchModeReview
			m.statusMsg = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.nameInput, cmd = m.nameInput.Update(msg)
	return m, cmd
}

func (m LaunchModel) updateEditBranch(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			val := m.branchInput.Value()
			if val == "" {
				m.statusMsg = "Branch cannot be empty"
				return m, nil
			}
			m.prompts[m.currentIdx].Branch = val
			// Move to prompt edit
			m.promptEdit.SetValue(m.prompts[m.currentIdx].Prompt)
			m.promptEdit.Focus()
			m.mode = launchModeEditPrompt
			m.statusMsg = ""
			return m, m.promptEdit.Focus()

		case "esc":
			m.mode = launchModeReview
			m.statusMsg = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.branchInput, cmd = m.branchInput.Update(msg)
	return m, cmd
}

func (m LaunchModel) updateEditPrompt(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "ctrl+d":
			val := m.promptEdit.Value()
			if strings.TrimSpace(val) == "" {
				m.statusMsg = "Prompt cannot be empty"
				return m, nil
			}
			m.prompts[m.currentIdx].Prompt = strings.TrimSpace(val)
			m.promptViewportIdx = -1 // force viewport rebuild with new content
			m.mode = launchModeReview
			m.statusMsg = ""
			return m, nil

		case "esc":
			m.mode = launchModeReview
			m.statusMsg = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.promptEdit, cmd = m.promptEdit.Update(msg)
	return m, cmd
}
