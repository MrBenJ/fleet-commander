package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
)

// helper to create a LaunchModel with minimal state for state-machine tests.
func testLaunchModel(yoloMode, skipConfirm bool) LaunchModel {
	m := LaunchModel{
		fleet:           &fleet.Fleet{},
		mode:            launchModeInput,
		yoloMode:        yoloMode,
		skipYoloConfirm: skipConfirm,
	}
	m.input.area = newTestTextarea("Fix the login bug")
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
	if updated.input.pendingYoloInput == "" {
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
	m.input.pendingYoloInput = "Fix the login bug"

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
	m.input.pendingYoloInput = "Fix the login bug"

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
	m.input.pendingYoloInput = "Fix the login bug"

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
	m.input.area = newTestTextarea("")

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
		currentIdx: 0,
	}
	m.review.nameInput = textinput.New()
	m.review.branchInput = textinput.New()
	m.review.promptEdit = textarea.New()
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
	m.review.nameInput = textinput.New()
	m.review.nameInput.SetValue("new-name")
	m.review.branchInput = textinput.New()

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
	m.review.nameInput = textinput.New()
	m.review.nameInput.SetValue("")

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
	m.review.nameInput = textinput.New()

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
	m.review.branchInput = textinput.New()
	m.review.branchInput.SetValue("fleet/new-branch")
	m.review.promptEdit = textarea.New()

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
	m.review.branchInput = textinput.New()
	m.review.branchInput.SetValue("")

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
	m.review.promptEdit = textarea.New()
	m.review.promptEdit.SetValue("Updated prompt text")

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
	m.review.promptEdit = textarea.New()
	m.review.promptEdit.SetValue("   ")

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

func TestBuildFullPrompt_AllParts(t *testing.T) {
	systemPrompt := "# Fleet System Prompt\nYou are a fleet agent."
	allItems := []LaunchItem{
		{AgentName: "auth-agent", Branch: "fleet/auth-agent", Prompt: "Fix login bug"},
		{AgentName: "api-agent", Branch: "fleet/api-agent", Prompt: "Add OAuth"},
	}
	current := allItems[0]

	result := buildFullPrompt(systemPrompt, allItems, current)

	// System prompt comes first
	if !strings.Contains(result, "# Fleet System Prompt") {
		t.Error("missing system prompt")
	}

	// Identity line
	if !strings.Contains(result, "You are: auth-agent (branch: fleet/auth-agent)") {
		t.Error("missing identity line")
	}

	// Roster table
	if !strings.Contains(result, "| auth-agent") {
		t.Error("missing auth-agent in roster")
	}
	if !strings.Contains(result, "| api-agent") {
		t.Error("missing api-agent in roster")
	}

	// Original task at end
	if !strings.HasSuffix(strings.TrimSpace(result), "Fix login bug") {
		t.Error("task prompt should be at the end")
	}
}

func TestBuildFullPrompt_EmptySystemPrompt(t *testing.T) {
	allItems := []LaunchItem{
		{AgentName: "solo", Branch: "fleet/solo", Prompt: "Do the thing"},
	}
	current := allItems[0]

	result := buildFullPrompt("", allItems, current)

	// Should NOT start with blank lines when system prompt is empty
	if strings.HasPrefix(result, "\n") {
		t.Error("prompt should not start with blank lines when system prompt is empty")
	}

	// Roster should still be present
	if !strings.Contains(result, "## Active Fleet Agents") {
		t.Error("missing roster section")
	}

	// Task should still be present
	if !strings.Contains(result, "Do the thing") {
		t.Error("missing task prompt")
	}
}

func TestLaunchModel_SystemPromptField(t *testing.T) {
	m := LaunchModel{}

	// Default state
	if m.systemPromptLoaded {
		t.Error("systemPromptLoaded should be false by default")
	}
	if m.systemPrompt != "" {
		t.Error("systemPrompt should be empty by default")
	}

	// After setting
	m.systemPrompt = "test prompt"
	m.systemPromptLoaded = true
	if m.systemPrompt != "test prompt" {
		t.Errorf("systemPrompt = %q, want %q", m.systemPrompt, "test prompt")
	}
}

func TestParseStructuredMarkdown_FiveAgents(t *testing.T) {
	input := `## Prompt 1 — Stripe Checkout

### 🤖 Your Agent Identity
| Field | Value |
|---|---|
| **Agent Name** | Stripe Checkout MoneyBot |
| **Agent ID** | ` + "`stripe-checkout`" + ` |
| **Git Branch** | ` + "`feature/stripe-checkout`" + ` |

Do the stripe work.

---

## Prompt 2 — E2E Tests

### 🤖 Your Agent Identity
| Field | Value |
|---|---|
| **Agent Name** | End-to-end Testing QABot |
| **Agent ID** | ` + "`e2e-testing`" + ` |
| **Git Branch** | ` + "`feature/e2e-testing`" + ` |

Do the e2e work.

---

## Prompt 3 — Discord

### 🤖 Your Agent Identity
| Field | Value |
|---|---|
| **Agent Name** | Discord CommsBot |
| **Agent ID** | ` + "`discord-integration`" + ` |
| **Git Branch** | ` + "`feature/discord-integration`" + ` |

Do the discord work.

---

## Prompt 4 — Email

### 🤖 Your Agent Identity
| Field | Value |
|---|---|
| **Agent Name** | Email NotifyBot |
| **Agent ID** | ` + "`email-integration`" + ` |
| **Git Branch** | ` + "`feature/email-integration`" + ` |

Do the email work.

---

## Prompt 5 — Docker

### 🤖 Your Agent Identity
| Field | Value |
|---|---|
| **Agent Name** | Docker DevExBot |
| **Agent ID** | ` + "`docker-dev-environment`" + ` |
| **Git Branch** | ` + "`dx-ops/docker-dev-environment`" + ` |

Do the docker work.
`
	log := &LaunchLogger{} // no-op logger
	items := parseStructuredMarkdown(input, log)

	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(items))
	}

	expected := []struct {
		name   string
		branch string
	}{
		{"stripe-checkout", "feature/stripe-checkout"},
		{"e2e-testing", "feature/e2e-testing"},
		{"discord-integration", "feature/discord-integration"},
		{"email-integration", "feature/email-integration"},
		{"docker-dev-environment", "dx-ops/docker-dev-environment"},
	}

	for i, exp := range expected {
		if items[i].AgentName != exp.name {
			t.Errorf("item[%d] agent_name=%q, want %q", i, items[i].AgentName, exp.name)
		}
		if items[i].Branch != exp.branch {
			t.Errorf("item[%d] branch=%q, want %q", i, items[i].Branch, exp.branch)
		}
		if items[i].Prompt == "" {
			t.Errorf("item[%d] prompt is empty", i)
		}
	}
}

func TestParseStructuredMarkdown_FallsBackForUnstructured(t *testing.T) {
	input := "Fix the login bug\nAdd OAuth support\nRefactor database"
	log := &LaunchLogger{}
	items := parseStructuredMarkdown(input, log)

	if items != nil {
		t.Errorf("expected nil for unstructured input, got %d items", len(items))
	}
}

func TestBuildFullPrompt_SingleAgent(t *testing.T) {
	systemPrompt := "# Prompt"
	allItems := []LaunchItem{
		{AgentName: "only-one", Branch: "fleet/only-one", Prompt: "Solo task"},
	}
	current := allItems[0]

	result := buildFullPrompt(systemPrompt, allItems, current)

	if !strings.Contains(result, "You are: only-one") {
		t.Error("missing identity for single agent")
	}
	if !strings.Contains(result, "| only-one") {
		t.Error("roster should show the single agent")
	}
}

// --- parseClaudeResponse tests ---

func TestParseClaudeResponse_ValidJSON(t *testing.T) {
	raw := `[{"prompt":"Fix the bug","agent_name":"fix-bug","branch":"fleet/fix-bug"}]`
	items, err := parseClaudeResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].AgentName != "fix-bug" {
		t.Errorf("agent_name=%q, want %q", items[0].AgentName, "fix-bug")
	}
	if items[0].Branch != "fleet/fix-bug" {
		t.Errorf("branch=%q, want %q", items[0].Branch, "fleet/fix-bug")
	}
	if items[0].Prompt != "Fix the bug" {
		t.Errorf("prompt=%q, want %q", items[0].Prompt, "Fix the bug")
	}
}

func TestParseClaudeResponse_MarkdownFences(t *testing.T) {
	raw := "```json\n" +
		`[{"prompt":"Add OAuth","agent_name":"add-oauth","branch":"fleet/add-oauth"}]` +
		"\n```"
	items, err := parseClaudeResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
	if items[0].AgentName != "add-oauth" {
		t.Errorf("agent_name=%q, want %q", items[0].AgentName, "add-oauth")
	}
}

func TestParseClaudeResponse_ExtraTextAroundJSON(t *testing.T) {
	raw := "Here are the tasks:\n\n" +
		`[{"prompt":"Task one","agent_name":"task-one","branch":"fleet/task-one"}]` +
		"\n\nLet me know if you need changes."
	items, err := parseClaudeResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(items))
	}
}

func TestParseClaudeResponse_NoJSON(t *testing.T) {
	raw := "I'm sorry, I can't do that."
	_, err := parseClaudeResponse(raw)
	if err == nil {
		t.Error("expected error for input with no JSON array")
	}
}

func TestParseClaudeResponse_EmptyArray(t *testing.T) {
	raw := "[]"
	_, err := parseClaudeResponse(raw)
	if err == nil {
		t.Error("expected error for empty array")
	}
}

func TestParseClaudeResponse_MultipleItems(t *testing.T) {
	raw := `[
		{"prompt":"Fix login","agent_name":"fix-login","branch":"fleet/fix-login"},
		{"prompt":"Add OAuth","agent_name":"add-oauth","branch":"fleet/add-oauth"},
		{"prompt":"Refactor DB","agent_name":"refactor-db","branch":"fleet/refactor-db"}
	]`
	items, err := parseClaudeResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
}

func TestNewSquadronLaunchModel_Defaults(t *testing.T) {
	f := &fleet.Fleet{}
	m := newSquadronLaunchModel(f, false)

	if !m.squadronMode {
		t.Error("squadronMode should be true")
	}
	if !m.yoloMode {
		t.Error("squadron mode implies yoloMode=true")
	}
	if !m.noAutoMerge {
		t.Error("squadron mode implies noAutoMerge=true (per-agent auto-merge off)")
	}
	if !m.skipYoloConfirm {
		t.Error("squadron mode implies skipYoloConfirm=true")
	}
	if m.mode != launchModeSquadronConsensus {
		t.Errorf("initial mode = %v, want squadronConsensus", m.mode)
	}
	if !m.autoMerge {
		t.Error("autoMerge (squadron-level) should default to true")
	}
}

func TestSquadronConsensus_Navigation(t *testing.T) {
	f := &fleet.Fleet{}
	m := newSquadronLaunchModel(f, false)

	if m.squadron.consensusCursor != 0 {
		t.Fatalf("cursor start = %d, want 0", m.squadron.consensusCursor)
	}

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(LaunchModel)
	if m.squadron.consensusCursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", m.squadron.consensusCursor)
	}

	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(LaunchModel)
	if m.consensusType != "review_master" {
		t.Errorf("consensusType = %q, want review_master", m.consensusType)
	}
	if m.mode != launchModeSquadronName {
		t.Errorf("mode after enter = %v, want squadronName", m.mode)
	}
}

func TestSquadronName_ValidatesAndAdvances(t *testing.T) {
	f := &fleet.Fleet{}
	m := newSquadronLaunchModel(f, false)
	m.consensusType = "none"
	m.mode = launchModeSquadronName
	m.squadron.nameInput.Focus()

	m.squadron.nameInput.SetValue("!!!")
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(LaunchModel)
	if m.mode != launchModeSquadronName {
		t.Errorf("invalid name should stay on name screen, mode = %v", m.mode)
	}
	if m.statusMsg == "" {
		t.Error("expected a validation error in statusMsg")
	}

	m.statusMsg = ""
	m.squadron.nameInput.SetValue("alpha")
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(LaunchModel)
	if m.squadronName != "alpha" {
		t.Errorf("squadronName = %q", m.squadronName)
	}
	if m.mode != launchModeInput {
		t.Errorf("after valid name, mode = %v, want launchModeInput", m.mode)
	}
}

func TestSquadronName_EscGoesBackToConsensus(t *testing.T) {
	f := &fleet.Fleet{}
	m := newSquadronLaunchModel(f, false)
	m.mode = launchModeSquadronName

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(LaunchModel)
	if m.mode != launchModeSquadronConsensus {
		t.Errorf("esc should return to consensus, mode = %v", m.mode)
	}
}

func TestLaunchCurrent_AppendsConsensusSuffix(t *testing.T) {
	f := &fleet.Fleet{}
	m := newSquadronLaunchModel(f, false)
	m.squadronName = "alpha"
	m.consensusType = "universal"
	m.baseBranch = "main"
	m.prompts = []LaunchItem{
		{AgentName: "a", Branch: "squadron/alpha/a", Prompt: "do a"},
		{AgentName: "b", Branch: "squadron/alpha/b", Prompt: "do b"},
	}

	got := m.applySquadronSuffixes("a", "ORIGINAL")

	if !strings.Contains(got, "ORIGINAL") {
		t.Error("original prompt should be preserved")
	}
	if !strings.Contains(got, "Squadron Consensus Protocol (UNIVERSAL)") {
		t.Error("universal suffix missing")
	}
	if !strings.Contains(got, "squadron-alpha") {
		t.Error("channel name missing")
	}
}

func TestLaunchCurrent_MergerGetsMergerSuffix(t *testing.T) {
	f := &fleet.Fleet{}
	m := newSquadronLaunchModel(f, false)
	m.squadronName = "alpha"
	m.consensusType = "none"
	m.baseBranch = "main"
	m.mergeMaster = "b"
	m.prompts = []LaunchItem{
		{AgentName: "a", Branch: "squadron/alpha/a", Prompt: "do a"},
		{AgentName: "b", Branch: "squadron/alpha/b", Prompt: "do b"},
	}

	aPrompt := m.applySquadronSuffixes("a", "A-ORIG")
	bPrompt := m.applySquadronSuffixes("b", "B-ORIG")

	if strings.Contains(aPrompt, "Squadron Merge Duties") {
		t.Error("non-merger should not get merge duties")
	}
	if !strings.Contains(bPrompt, "Squadron Merge Duties") {
		t.Error("merger should get merge duties")
	}
	if !strings.Contains(bPrompt, "a -> squadron/alpha/a") {
		t.Error("merger suffix should list all agents")
	}
}

func TestLaunchCurrent_PersonaPrepended(t *testing.T) {
	f := &fleet.Fleet{}
	m := newSquadronLaunchModel(f, false)
	m.squadronName = "alpha"
	m.consensusType = "none"
	m.baseBranch = "main"
	m.personas = map[string]string{"a": "overconfident-engineer"}
	m.prompts = []LaunchItem{
		{AgentName: "a", Branch: "squadron/alpha/a", Prompt: "do a"},
	}

	got := m.applySquadronSuffixes("a", "ORIGINAL")

	if !strings.HasPrefix(got, "You are the Overconfident Engineer") {
		t.Errorf("persona should be prepended above everything, got prefix: %q", got[:60])
	}
}
