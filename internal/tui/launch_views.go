package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m LaunchModel) View() string {
	if m.quitting {
		if m.aborted {
			return m.renderSummary("Aborted.")
		}
		return m.renderSummary("Done.")
	}

	if m.width == 0 {
		return "Loading..."
	}

	switch m.mode {
	case launchModeInput:
		return m.viewInput()
	case launchModeYoloConfirm:
		return m.viewYoloConfirm()
	case launchModeGenerating:
		return m.viewGenerating()
	case launchModeReview:
		return m.viewReview()
	case launchModeEditName:
		return m.viewEditName()
	case launchModeEditBranch:
		return m.viewEditBranch()
	case launchModeEditPrompt:
		return m.viewEditPrompt()
	}

	return ""
}

func (m LaunchModel) viewInput() string {
	var b strings.Builder

	if m.yoloMode {
		yoloWarning := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#FF0000")).
			Padding(0, 2).
			Render("⚠  WARNING: ULTRA DANGEROUS YOLO MODE ACTIVATED  ⚠")
		yoloSubtext := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF0000")).
			Render("ALL YOUR CHANGES WILL FIRE OFF WITHOUT ASKING FOR PERMISSION")
		b.WriteString("\n  " + yoloWarning + "\n")
		b.WriteString("  " + yoloSubtext + "\n\n")
	}

	b.WriteString(titleStyle.Render("⚓ Fleet Launch") + "\n\n")
	b.WriteString("  Enter your tasks:\n\n")
	b.WriteString(m.inputArea.View() + "\n\n")

	if m.statusMsg != "" {
		b.WriteString("  " + stoppedStyle.Render("❌ "+m.statusMsg) + "\n")
	}

	b.WriteString(helpStyle.Render("  Ctrl+D: submit • Esc: cancel"))

	return b.String()
}

func (m LaunchModel) viewYoloConfirm() string {
	var b strings.Builder

	warningBox := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#FF0000")).
		Padding(1, 3).
		Render("⚠  ARE YOU ABSOLUTELY SURE THIS IS READY?  ⚠")

	b.WriteString("\n  " + warningBox + "\n\n")

	warnText := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FF6600"))
	b.WriteString("  " + warnText.Render("This will run and you cannot stop it.") + "\n")
	b.WriteString("  " + warnText.Render("Ensure you have enough usage in your account to make it through the end of this.") + "\n")
	b.WriteString("  " + warnText.Render("Please don't destroy humanity.") + "\n")
	b.WriteString("  " + warnText.Render("Please be sober.") + "\n\n")

	b.WriteString(helpStyle.Render("  Ctrl+D: confirm and launch • Esc: go back"))

	return b.String()
}

func (m LaunchModel) viewGenerating() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("⚓ Fleet Launch") + "\n\n")
	b.WriteString(fmt.Sprintf("  %s Generating prompts, agent names, and branches...\n\n", m.spinner.View()))
	b.WriteString(helpStyle.Render("  Esc: cancel"))
	return b.String()
}

func (m LaunchModel) viewReview() string {
	var b strings.Builder
	item := m.prompts[m.currentIdx]

	b.WriteString(titleStyle.Render(
		fmt.Sprintf("⚓ Fleet Launch — Task %d of %d", m.currentIdx+1, len(m.prompts)),
	) + "\n\n")

	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	valueStyle := lipgloss.NewStyle().Bold(true)

	b.WriteString(fmt.Sprintf("  %s  %s\n", labelStyle.Render("Agent: "), valueStyle.Render(item.AgentName)))
	b.WriteString(fmt.Sprintf("  %s %s\n", labelStyle.Render("Branch:"), valueStyle.Render(item.Branch)))

	// Prompt section — scrollable viewport
	b.WriteString(fmt.Sprintf("  %s\n", labelStyle.Render("Prompt:")))
	b.WriteString(m.promptViewport.View() + "\n")

	scrollInfo := ""
	if m.promptViewport.TotalLineCount() > m.promptViewport.VisibleLineCount() {
		scrollInfo = fmt.Sprintf(" (scroll: ↑↓/jk • %.0f%%)", m.promptViewport.ScrollPercent()*100)
	}

	b.WriteString("\n")

	actionStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	b.WriteString(fmt.Sprintf("  %s Launch  %s Edit  %s Skip  %s Abort%s\n",
		actionStyle.Render("[L]"),
		actionStyle.Render("[E]"),
		actionStyle.Render("[S]"),
		actionStyle.Render("[A]"),
		helpStyle.Render(scrollInfo),
	))

	if m.statusMsg != "" {
		b.WriteString("\n  " + stoppedStyle.Render("⚠ "+m.statusMsg))
	}

	// Show launched agents
	if len(m.launched) > 0 {
		b.WriteString("\n")
		successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00"))
		for _, name := range m.launched {
			b.WriteString("  " + successStyle.Render("✓ Launched: "+name) + "\n")
		}
	}

	return b.String()
}

func (m LaunchModel) viewEditName() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("⚓ Edit Agent Name") + "\n\n")
	b.WriteString("  " + selectedItemStyle.Render("> Agent name: ") + m.nameInput.View() + "\n")
	if m.statusMsg != "" {
		b.WriteString("\n  " + stoppedStyle.Render("❌ "+m.statusMsg))
	}
	b.WriteString("\n" + helpStyle.Render("  Enter: next (branch) • Esc: back"))
	return b.String()
}

func (m LaunchModel) viewEditBranch() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("⚓ Edit Branch Name") + "\n\n")
	b.WriteString("  Agent name:  " + m.prompts[m.currentIdx].AgentName + "\n")
	b.WriteString("  " + selectedItemStyle.Render("> Branch name: ") + m.branchInput.View() + "\n")
	if m.statusMsg != "" {
		b.WriteString("\n  " + stoppedStyle.Render("❌ "+m.statusMsg))
	}
	b.WriteString("\n" + helpStyle.Render("  Enter: next (prompt) • Esc: back"))
	return b.String()
}

func (m LaunchModel) viewEditPrompt() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("⚓ Edit Prompt") + "\n\n")
	b.WriteString("  Agent: " + m.prompts[m.currentIdx].AgentName + "\n")
	b.WriteString("  Branch: " + m.prompts[m.currentIdx].Branch + "\n\n")
	b.WriteString(m.promptEdit.View() + "\n")
	if m.statusMsg != "" {
		b.WriteString("\n  " + stoppedStyle.Render("❌ "+m.statusMsg))
	}
	b.WriteString("\n" + helpStyle.Render("  Ctrl+D: confirm • Esc: back"))
	return b.String()
}

func (m LaunchModel) renderSummary(header string) string {
	var b strings.Builder
	b.WriteString(header + "\n")
	if m.statusMsg != "" {
		b.WriteString(fmt.Sprintf("Error: %s\n", m.statusMsg))
	}
	if len(m.launched) > 0 {
		b.WriteString(fmt.Sprintf("Launched %d agent(s): %s\n", len(m.launched), strings.Join(m.launched, ", ")))
	}
	if m.skipped > 0 {
		b.WriteString(fmt.Sprintf("Skipped %d prompt(s)\n", m.skipped))
	}
	if m.log != nil && m.log.Path() != "" {
		b.WriteString(fmt.Sprintf("Debug log: %s\n", m.log.Path()))
	}
	return b.String()
}
