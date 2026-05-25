# Antigravity (`agy`) Driver Integration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add Google Antigravity's `agy` CLI as a first-class Fleet Commander driver — usable everywhere claude-code/codex are (manual add, edit, squadron launch, and the hangar "AI Generate from Description" panel) — with the brand icon, a pre-launch babysitting warning, and a cost-analysis info panel.

**Architecture:** The Antigravity driver is structurally a clone of the Codex driver: a CLI-backed driver with no Claude-style hook injection, state via tmux pane-scraping, and a clean `agy -p` headless mode for planning. Registering it in the driver map makes it flow automatically through every driver dropdown (which read `driver.Available()`). The hangar API gains an availability check and accepts it in the AI-Generate handler. The web wizard gets the icon, the AI-Generate option, and two advisory pre-launch panels.

**Tech Stack:** Go 1.21+ (backend driver, hangar HTTP API), React 18 + TypeScript + Vite (hangar SPA), Vitest + Testing Library (web tests), Go `testing` (backend tests).

---

## Reference: the `agy` CLI surface

| Capability | Command |
|---|---|
| Interactive session | `agy` |
| Headless one-shot prompt (plain-text stdout) | `agy -p "<prompt>"` |
| Install | `curl -fsSL https://antigravity.google/cli/install.sh \| bash` |

- `agy -p` emits the model's answer as plain text — feeds the existing `extractJSON` unchanged. Do **not** use `--output-format json` (it wraps the answer in an envelope `extractJSON` won't match).
- **No documented permission-bypass/yolo flag.** Squadron yolo is best-effort.
- **No Claude-Code-style hook injection.** `InjectHooks`/`RemoveHooks` are no-ops; state is pane-scrape only.
- **No `ccusage` source for `agy`** — cost renders as "—" (like aider/generic). Do not add a `cost.driverSource` mapping.

---

## File structure

**Backend (Go)**
- Create: `internal/driver/antigravity.go` — `AntigravityDriver` implementing `driver.Driver`.
- Create: `internal/driver/antigravity_test.go` — unit tests mirroring `codex_test.go`.
- Modify: `internal/driver/registry.go` — register `"antigravity"`.
- Modify: `internal/hangar/api/handlers.go` — `driverBinaries` + `HandleAvailableDrivers`.
- Modify: `internal/hangar/api/generate_handler.go` — accept + availability-check `antigravity`.
- Modify: `internal/hangar/api/handlers_test.go` — extend available-drivers + generate tests.

**Frontend (web)**
- Create: `web/src/components/icons/antigravityIconData.ts` — generated base64 data-URI constant.
- Create: `web/src/components/icons/AntigravityIcon.tsx` — `<img>` icon component.
- Modify: `web/src/components/mission/AgentPill.tsx` — `DriverIcon` antigravity case.
- Modify: `web/src/components/wizard/review-constants.ts` — driver color maps + `costUnsupportedDrivers`.
- Modify: `web/src/components/wizard/AIGeneratePanel.tsx` — Antigravity AI-Generate option + availability.
- Modify: `web/src/components/wizard/AIGeneratePanel.test.tsx` — option coverage.
- Modify: `web/src/components/wizard/ReviewStep.tsx` — babysitting warning + cost info panel.
- Modify: `web/src/components/wizard/ReviewStep.test.tsx` — panel coverage.

**Docs**
- Modify: `CLAUDE.md`, `AGENTS.md`, `README.md` — driver enumerations.

---

## Task 1: Antigravity driver (`AntigravityDriver`)

**Files:**
- Create: `internal/driver/antigravity.go`
- Test: `internal/driver/antigravity_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/driver/antigravity_test.go` (reuses the `nonEmptyLines` helper already defined in `codex_test.go`, same package):

```go
package driver

import (
	"strings"
	"testing"
)

func TestAntigravityName(t *testing.T) {
	d := &AntigravityDriver{}
	if d.Name() != "antigravity" {
		t.Errorf("expected Name() to return 'antigravity', got %q", d.Name())
	}
}

func TestAntigravityInteractiveCommand(t *testing.T) {
	d := &AntigravityDriver{}
	cmd := d.InteractiveCommand()
	if len(cmd) != 1 || cmd[0] != "agy" {
		t.Errorf("expected [\"agy\"], got %v", cmd)
	}
}

func TestAntigravityBuildCommand(t *testing.T) {
	d := &AntigravityDriver{}

	t.Run("normal mode seeds interactive agy with the prompt", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{PromptFile: "/tmp/prompt.txt", YoloMode: false})
		if !strings.HasPrefix(script, "#!/usr/bin/env bash") {
			t.Errorf("script does not start with shebang: %q", script)
		}
		if !strings.Contains(script, "exec agy") {
			t.Errorf("script does not exec agy: %q", script)
		}
		if !strings.Contains(script, "/tmp/prompt.txt") {
			t.Errorf("script does not contain prompt file path: %q", script)
		}
	})

	t.Run("yolo mode is best-effort: no bypass flag exists", func(t *testing.T) {
		script := d.BuildCommand(LaunchOpts{PromptFile: "/tmp/prompt.txt", YoloMode: true})
		if !strings.Contains(script, "exec agy") {
			t.Errorf("yolo script does not exec agy: %q", script)
		}
		if strings.Contains(script, "--dangerously") || strings.Contains(script, "--yolo") {
			t.Errorf("agy has no documented bypass flag; none should be emitted: %q", script)
		}
	})
}

func TestAntigravityDetectState_Waiting(t *testing.T) {
	d := &AntigravityDriver{}
	cases := []struct {
		name        string
		fullContent string
	}{
		{"Y/n prompt", "Apply these changes? [Y/n]"},
		{"y/N prompt", "Run command [y/N]"},
		{"(y/n) prompt", "Proceed (y/n)"},
		{"requesting approval", "agy is requesting approval to run a command"},
		{"Approve option", "Approve once"},
		{"> prompt on own line", "some output\n>"},
		{"question line", "What would you like me to do next?"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := d.DetectState(nonEmptyLines(tc.fullContent), tc.fullContent)
			if result == nil {
				t.Fatal("DetectState returned nil, expected waiting")
			}
			if *result != StateWaiting {
				t.Errorf("expected waiting, got %q", *result)
			}
		})
	}
}

func TestAntigravityDetectState_Working(t *testing.T) {
	d := &AntigravityDriver{}
	cases := []struct {
		name        string
		fullContent string
	}{
		{"esc to interrupt", "Thinking... (esc to interrupt)"},
		{"spinner", "Working ⠋"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := d.DetectState(nonEmptyLines(tc.fullContent), tc.fullContent)
			if result == nil {
				t.Fatal("DetectState returned nil, expected working")
			}
			if *result != StateWorking {
				t.Errorf("expected working, got %q", *result)
			}
		})
	}
}

func TestAntigravityDetectState_Unknown(t *testing.T) {
	d := &AntigravityDriver{}
	cases := []struct {
		name        string
		fullContent string
	}{
		{"random output", "some random text that matches nothing"},
		{"finished message", "Done. All changes applied."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := d.DetectState(nonEmptyLines(tc.fullContent), tc.fullContent)
			if result != nil {
				t.Errorf("expected nil for unknown content, got %q", *result)
			}
		})
	}
}

func TestAntigravityDetectState_Empty(t *testing.T) {
	d := &AntigravityDriver{}
	result := d.DetectState(nil, "")
	if result == nil {
		t.Fatal("DetectState returned nil for empty content, expected starting")
	}
	if *result != StateStarting {
		t.Errorf("expected starting for empty content, got %q", *result)
	}
}

func TestAntigravityHooksAreNoOps(t *testing.T) {
	d := &AntigravityDriver{}
	if err := d.InjectHooks("/tmp/whatever"); err != nil {
		t.Errorf("InjectHooks should be a no-op, got %v", err)
	}
	if err := d.RemoveHooks("/tmp/whatever"); err != nil {
		t.Errorf("RemoveHooks should be a no-op, got %v", err)
	}
}

func TestAntigravityCheckAvailable(t *testing.T) {
	d := &AntigravityDriver{}
	err := d.CheckAvailable()
	// agy may or may not be installed — just verify a missing agy gives a
	// helpful install hint and nothing panics.
	if err != nil {
		if !strings.Contains(err.Error(), "antigravity.google/cli/install.sh") {
			t.Errorf("error should contain install hint, got: %v", err)
		}
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/driver/ -run TestAntigravity -v`
Expected: FAIL — compile error, `undefined: AntigravityDriver`.

- [ ] **Step 3: Write the driver implementation**

Create `internal/driver/antigravity.go`:

```go
package driver

import (
	"fmt"
	"os/exec"
	"strings"
)

// AntigravityDriver implements Driver for Google's Antigravity CLI (agy).
// See https://antigravity.google/docs for the CLI surface.
//
// Antigravity mirrors the Codex driver shape: a CLI-backed agent with no
// Claude-Code-style hook injection, so state is determined by pane scraping.
type AntigravityDriver struct{}

func (d *AntigravityDriver) Name() string { return "antigravity" }

func (d *AntigravityDriver) InteractiveCommand() []string {
	return []string{"agy"}
}

// PlanCommand runs agy headlessly with -p, which emits the model's answer as
// plain text — directly consumable by the caller's JSON extractor. We avoid
// --output-format json, which would wrap the answer in an envelope the
// extractor does not expect.
func (d *AntigravityDriver) PlanCommand(prompt string) ([]byte, error) {
	return exec.Command("agy", "-p", prompt).CombinedOutput()
}

// BuildCommand seeds an interactive agy session with the prompt so the user can
// watch and intervene in the tmux pane.
//
// YoloMode is best-effort: agy exposes no documented permission-bypass flag
// (its autonomy lives behind the in-TUI /goal slash command, which cannot be
// passed as a launch argument), so yolo currently adds nothing. If Antigravity
// ships a bypass flag, add it here.
func (d *AntigravityDriver) BuildCommand(opts LaunchOpts) string {
	return fmt.Sprintf("#!/usr/bin/env bash\nprompt=$(cat %q)\nexec agy \"$prompt\"\n", opts.PromptFile)
}

// DetectState analyzes tmux pane content to determine the agy agent state.
//
// NOTE: These patterns are based on Antigravity's documented TUI and are not
// yet empirically tuned. Run agy in a tmux session and inspect output via
// `tmux capture-pane` to discover and refine patterns.
func (d *AntigravityDriver) DetectState(bottomLines []string, fullContent string) *AgentState {
	if strings.TrimSpace(fullContent) == "" {
		s := StateStarting
		return &s
	}

	bottomText := strings.Join(bottomLines, "\n")

	// ── WAITING PATTERNS (checked first) ──

	if strings.Contains(bottomText, "[Y/n]") || strings.Contains(bottomText, "[y/N]") {
		s := StateWaiting
		return &s
	}
	if strings.Contains(bottomText, "(y/n)") {
		s := StateWaiting
		return &s
	}
	if strings.Contains(bottomText, "requesting approval") || strings.Contains(bottomText, "Approve") {
		s := StateWaiting
		return &s
	}
	for _, line := range bottomLines {
		if strings.TrimSpace(line) == ">" {
			s := StateWaiting
			return &s
		}
	}
	veryBottom := lastN(bottomLines, 3)
	for _, line := range veryBottom {
		trimmed := strings.TrimSpace(line)
		if strings.HasSuffix(trimmed, "?") && len(trimmed) > 10 {
			s := StateWaiting
			return &s
		}
	}

	// ── WORKING PATTERNS ──

	if strings.Contains(bottomText, "esc to interrupt") {
		s := StateWorking
		return &s
	}
	spinners := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	for _, sp := range spinners {
		if strings.Contains(bottomText, sp) {
			s := StateWorking
			return &s
		}
	}

	// No pattern matched — return nil to let the caller fall back.
	return nil
}

// InjectHooks is a no-op for Antigravity (no Claude-style hook system).
func (d *AntigravityDriver) InjectHooks(worktreePath string) error { return nil }

// RemoveHooks is a no-op for Antigravity (no Claude-style hook system).
func (d *AntigravityDriver) RemoveHooks(worktreePath string) error { return nil }

func (d *AntigravityDriver) CheckAvailable() error {
	if _, err := exec.LookPath("agy"); err != nil {
		return fmt.Errorf("agy command not found in PATH (install: curl -fsSL https://antigravity.google/cli/install.sh | bash)")
	}
	return nil
}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/driver/ -run TestAntigravity -v`
Expected: PASS (all subtests).

- [ ] **Step 5: Commit**

```bash
git add internal/driver/antigravity.go internal/driver/antigravity_test.go
git commit -m "feat(driver): add Antigravity (agy) driver

Mirrors the Codex driver: CLI-backed, no hook injection, pane-scrape
state detection. agy -p powers headless planning; yolo is best-effort
because agy exposes no documented bypass flag.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 2: Register the driver

**Files:**
- Modify: `internal/driver/registry.go`
- Test: `internal/driver/antigravity_test.go` (add registration test)

- [ ] **Step 1: Write the failing test**

Append to `internal/driver/antigravity_test.go`:

```go
func TestAntigravityRegistered(t *testing.T) {
	d, err := Get("antigravity")
	if err != nil {
		t.Fatalf("Get('antigravity') returned error: %v", err)
	}
	if d.Name() != "antigravity" {
		t.Errorf("expected 'antigravity', got %q", d.Name())
	}
}

func TestAntigravityInAvailable(t *testing.T) {
	found := false
	for _, name := range Available() {
		if name == "antigravity" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'antigravity' in Available(), got %v", Available())
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/driver/ -run 'TestAntigravityRegistered|TestAntigravityInAvailable' -v`
Expected: FAIL — `Get('antigravity')` returns `unknown driver "antigravity"`.

- [ ] **Step 3: Register the driver**

In `internal/driver/registry.go`, add the entry to the `drivers` map:

```go
var drivers = map[string]Driver{
	"claude-code": &ClaudeCodeDriver{},
	"codex":       &CodexDriver{},
	"aider":       &AiderDriver{},
	"kimi-code":   &KimiCodeDriver{},
	"antigravity": &AntigravityDriver{},
	// "generic" is not a singleton — it's constructed per-agent via GetForAgent.
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/driver/ -v`
Expected: PASS (whole package, including registry tests).

- [ ] **Step 5: Commit**

```bash
git add internal/driver/registry.go internal/driver/antigravity_test.go
git commit -m "feat(driver): register antigravity in the driver registry

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 3: Hangar API — availability reporting

**Files:**
- Modify: `internal/hangar/api/handlers.go:33-36` (`driverBinaries`), `internal/hangar/api/handlers.go:78-81` (`HandleAvailableDrivers`)
- Test: `internal/hangar/api/handlers_test.go` (extend `TestHandleAvailableDrivers`)

- [ ] **Step 1: Write the failing test additions**

In `internal/hangar/api/handlers_test.go`, update `TestHandleAvailableDrivers` so the fake `execLookPath` makes `agy` available and assert antigravity is reported. Replace the existing function body (lines 142-177) with:

```go
func TestHandleAvailableDrivers(t *testing.T) {
	origLookPath := execLookPath
	execLookPath = func(file string) (string, error) {
		if file == "codex" {
			return "", exec.ErrNotFound
		}
		return "/bin/" + file, nil
	}
	t.Cleanup(func() { execLookPath = origLookPath })

	h := NewHandlers("/tmp/fake-repo", "/tmp/fake-repo/.fleet")
	req := httptest.NewRequest(http.MethodGet, "/api/drivers/available", nil)
	w := httptest.NewRecorder()

	h.HandleAvailableDrivers(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var drivers []AvailableDriverResponse
	if err := json.Unmarshal(w.Body.Bytes(), &drivers); err != nil {
		t.Fatalf("bad json: %v", err)
	}

	availability := map[string]bool{}
	present := map[string]bool{}
	for _, d := range drivers {
		availability[d.Name] = d.Available
		present[d.Name] = true
	}
	if !availability["claude-code"] {
		t.Fatal("expected claude-code to always be available")
	}
	if availability["codex"] {
		t.Fatal("expected codex to be unavailable when binary lookup fails")
	}
	if !present["antigravity"] {
		t.Fatal("expected antigravity to be reported by available-drivers")
	}
	if !availability["antigravity"] {
		t.Fatal("expected antigravity to be available when agy lookup succeeds")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./internal/hangar/api/ -run TestHandleAvailableDrivers -v`
Expected: FAIL — antigravity not present in the response.

- [ ] **Step 3: Add antigravity to the binary map and the response**

In `internal/hangar/api/handlers.go`, add `agy` to `driverBinaries`:

```go
	driverBinaries = map[string]string{
		"codex":       "codex",
		"antigravity": "agy",
	}
```

And add antigravity to the slice returned by `HandleAvailableDrivers`:

```go
	drivers := []AvailableDriverResponse{
		{Name: "claude-code", Available: true},
		{Name: "codex", Available: isDriverBinaryAvailable("codex")},
		{Name: "antigravity", Available: isDriverBinaryAvailable("antigravity")},
	}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `go test ./internal/hangar/api/ -run TestHandleAvailableDrivers -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/hangar/api/handlers.go internal/hangar/api/handlers_test.go
git commit -m "feat(hangar): report antigravity availability via agy binary lookup

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 4: AI-Generate accepts Antigravity

**Files:**
- Modify: `internal/hangar/api/generate_handler.go:32-39`
- Test: `internal/hangar/api/handlers_test.go` (add antigravity generate tests)

- [ ] **Step 1: Write the failing tests**

Append to `internal/hangar/api/handlers_test.go` (the `fakePlannerDriver` type and `driverGet`/`execLookPath` seams already exist and are used by the codex generate tests):

```go
func TestHandleGenerate_UsesAntigravityDriver(t *testing.T) {
	origDriverGet := driverGet
	origLookPath := execLookPath
	planner := &fakePlannerDriver{
		name:   "antigravity",
		output: []byte(`[{"name":"agy-agent","prompt":"Do agy things","branch":"","driver":"antigravity","persona":""}]`),
	}
	driverGet = func(name string) (driver.Driver, error) {
		if name != "antigravity" {
			t.Fatalf("expected antigravity driver lookup, got %q", name)
		}
		return planner, nil
	}
	execLookPath = func(file string) (string, error) {
		if file == "agy" {
			return "/usr/local/bin/agy", nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() {
		driverGet = origDriverGet
		execLookPath = origLookPath
	})

	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/generate", strings.NewReader(`{"description":"split this up","driver":"antigravity"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleGenerate(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(planner.lastPrompt, `"driver": "antigravity"`) {
		t.Fatalf("expected metaprompt to request antigravity driver, got %q", planner.lastPrompt)
	}

	var resp GenerateResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if len(resp.Agents) != 1 || resp.Agents[0].Driver != "antigravity" {
		t.Fatalf("expected antigravity agent response, got %+v", resp.Agents)
	}
}

func TestHandleGenerate_RejectsUnavailableAntigravity(t *testing.T) {
	origLookPath := execLookPath
	execLookPath = func(file string) (string, error) {
		if file == "agy" {
			return "", exec.ErrNotFound
		}
		return "/bin/" + file, nil
	}
	t.Cleanup(func() { execLookPath = origLookPath })

	h := NewHandlers("/tmp/fake", "/tmp/fake/.fleet")
	req := httptest.NewRequest(http.MethodPost, "/api/squadron/generate", strings.NewReader(`{"description":"split this up","driver":"antigravity"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.HandleGenerate(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/hangar/api/ -run 'TestHandleGenerate_UsesAntigravityDriver|TestHandleGenerate_RejectsUnavailableAntigravity' -v`
Expected: FAIL — the handler rejects antigravity with "driver must be claude-code or codex" (400 for the Uses test).

- [ ] **Step 3: Accept antigravity and generalize the availability check**

In `internal/hangar/api/generate_handler.go`, replace the driver-guard block (lines 32-39):

```go
	if selectedDriver != "claude-code" && selectedDriver != "codex" && selectedDriver != "antigravity" {
		writeError(w, http.StatusBadRequest, "driver must be claude-code, codex, or antigravity")
		return
	}
	if !isDriverBinaryAvailable(selectedDriver) {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("%s not installed", selectedDriver))
		return
	}
```

Note: `isDriverBinaryAvailable` returns `true` for drivers without a binary mapping (claude-code), so the generalized check preserves claude-code behavior while covering codex and antigravity from `driverBinaries`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/hangar/api/ -run TestHandleGenerate -v`
Expected: PASS (all generate tests, including the existing codex ones).

- [ ] **Step 5: Commit**

```bash
git add internal/hangar/api/generate_handler.go internal/hangar/api/handlers_test.go
git commit -m "feat(hangar): accept antigravity in AI-Generate handler

Generalizes the driver guard and availability check so AI Generate can
decompose tasks with agy -p.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 6: Run the full backend suite**

Run: `go test ./... && go vet ./...`
Expected: PASS, no vet warnings.

---

## Task 5: Antigravity brand icon

**Files:**
- Create: `web/src/components/icons/antigravityIconData.ts` (generated)
- Create: `web/src/components/icons/AntigravityIcon.tsx`

- [ ] **Step 1: Generate the base64 data-URI module**

The source PNG is at `/Users/bjunya/Desktop/Google-Antigravity-Icon-Full-Color.png`. Downscale to 64×64 (crisp at the 14–16px render size, keeps the inline string small) and emit a TS module. Run from the repo root:

```bash
PNG="/Users/bjunya/Desktop/Google-Antigravity-Icon-Full-Color.png"
sips -z 64 64 "$PNG" --out /tmp/agy64.png >/dev/null
B64=$(base64 -i /tmp/agy64.png | tr -d '\n')
printf 'export const ANTIGRAVITY_ICON_DATA_URI =\n  "data:image/png;base64,%s";\n' "$B64" > web/src/components/icons/antigravityIconData.ts
```

Verify the file was written and is a single export of a data URI:

Run: `head -c 80 web/src/components/icons/antigravityIconData.ts`
Expected: starts with `export const ANTIGRAVITY_ICON_DATA_URI =` followed by `"data:image/png;base64,...`.

- [ ] **Step 2: Write the icon component**

Create `web/src/components/icons/AntigravityIcon.tsx`:

```tsx
import { ANTIGRAVITY_ICON_DATA_URI } from "./antigravityIconData";

export function AntigravityIcon({ size = 14 }: { size?: number }) {
  return (
    <img
      src={ANTIGRAVITY_ICON_DATA_URI}
      width={size}
      height={size}
      alt=""
      aria-hidden="true"
      style={{ display: "inline-block", verticalAlign: "middle" }}
    />
  );
}
```

- [ ] **Step 3: Verify the web build typechecks**

Run: `cd web && npx tsc --noEmit`
Expected: no errors (the generated module resolves; `AntigravityIcon` compiles).

- [ ] **Step 4: Commit**

```bash
git add web/src/components/icons/antigravityIconData.ts web/src/components/icons/AntigravityIcon.tsx
git commit -m "feat(web): add Antigravity brand icon

Faithful PNG embedded as a base64 data-URI, downscaled to 64px.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 6: Wire the icon into `DriverIcon`

**Files:**
- Modify: `web/src/components/mission/AgentPill.tsx:4-6` (imports), `web/src/components/mission/AgentPill.tsx:27-33` (switch)

- [ ] **Step 1: Add the import**

In `web/src/components/mission/AgentPill.tsx`, add the import alongside the other icon imports (after line 6):

```tsx
import { AntigravityIcon } from "../icons/AntigravityIcon";
```

- [ ] **Step 2: Add the switch case**

In the `DriverIcon` `switch (driver)`, add a case after the `aider` case:

```tsx
    case "antigravity":
      return <span style={style}><AntigravityIcon size={size} /></span>;
```

- [ ] **Step 3: Verify typecheck and existing tests pass**

Run: `cd web && npx tsc --noEmit && npm run test -- --run AgentPill`
Expected: typecheck clean; any existing AgentPill tests still pass.

- [ ] **Step 4: Commit**

```bash
git add web/src/components/mission/AgentPill.tsx
git commit -m "feat(web): render Antigravity icon in DriverIcon

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 7: Driver colors + cost-unsupported constant

**Files:**
- Modify: `web/src/components/wizard/review-constants.ts:34-48` (color maps), add `costUnsupportedDrivers`

- [ ] **Step 1: Add antigravity to the color maps and add the cost-unsupported set**

In `web/src/components/wizard/review-constants.ts`, add antigravity to both maps and append the new constant after `driverTextColors`:

```ts
export const driverColors: Record<string, string> = {
  "claude-code": "rgba(31,111,235,0.2)",
  codex: "rgba(46,160,67,0.2)",
  aider: "rgba(240,136,62,0.2)",
  "kimi-code": "rgba(167,139,250,0.2)",
  antigravity: "rgba(66,133,244,0.2)",
  generic: "rgba(139,148,158,0.2)",
};

export const driverTextColors: Record<string, string> = {
  "claude-code": "var(--blue)",
  codex: "var(--green)",
  aider: "var(--orange)",
  "kimi-code": "#a78bfa",
  antigravity: "#4285f4",
  generic: "var(--text-secondary)",
};

// Drivers ccusage cannot report on — their cost column renders blank.
// Mirror of the Go source of truth in internal/cost/cost.go (driverSource);
// keep these two in sync by hand.
export const costUnsupportedDrivers = ["aider", "generic", "antigravity"];
```

- [ ] **Step 2: Verify typecheck**

Run: `cd web && npx tsc --noEmit`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add web/src/components/wizard/review-constants.ts
git commit -m "feat(web): add antigravity driver colors and cost-unsupported set

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 8: Antigravity option in AI Generate

**Files:**
- Modify: `web/src/components/wizard/AIGeneratePanel.tsx`
- Test: `web/src/components/wizard/AIGeneratePanel.test.tsx`

- [ ] **Step 1: Write the failing tests**

In `web/src/components/wizard/AIGeneratePanel.test.tsx`, update the default `beforeEach` mock to include antigravity and add two tests. Change the `beforeEach` block (lines 22-28) to:

```tsx
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetAvailableDrivers.mockResolvedValue([
      { name: "claude-code", available: true },
      { name: "codex", available: false },
      { name: "antigravity", available: false },
    ]);
  });
```

Add these tests inside the `describe("AIGeneratePanel", ...)` block:

```tsx
  it("disables the Antigravity option when agy is not installed", async () => {
    render(<AIGeneratePanel {...defaultProps} />);
    expect(
      await screen.findByRole("option", { name: /antigravity not installed/i })
    ).toBeDisabled();
  });

  it("sends antigravity when available and selected", async () => {
    const user = userEvent.setup();
    mockGetAvailableDrivers.mockResolvedValue([
      { name: "claude-code", available: true },
      { name: "codex", available: false },
      { name: "antigravity", available: true },
    ]);
    mockGenerateAgents.mockResolvedValue({
      agents: [
        { name: "agy-agent", branch: "", prompt: "Build with agy", driver: "antigravity", persona: "" },
      ],
    });

    render(<AIGeneratePanel {...defaultProps} />);
    await vi.waitFor(() => {
      expect(screen.getByRole("option", { name: "Antigravity" })).not.toBeDisabled();
    });
    await user.selectOptions(screen.getByLabelText("Driver"), "antigravity");
    await user.type(screen.getByLabelText("Task description for AI generation"), "Build agy things");
    await user.click(screen.getByRole("button", { name: /generate agent breakdown/i }));

    expect(mockGenerateAgents).toHaveBeenCalledWith("Build agy things", "antigravity");
  });
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd web && npm run test -- --run AIGeneratePanel`
Expected: FAIL — no Antigravity option exists yet.

- [ ] **Step 3: Add antigravity availability state and the option**

In `web/src/components/wizard/AIGeneratePanel.tsx`:

(a) Add an availability state next to `codexAvailable` (after line 97):

```tsx
  const [antigravityAvailable, setAntigravityAvailable] = useState(false);
```

(b) In the `useEffect` that loads drivers, set both flags. Replace the `.then((drivers) => { ... })` body (lines 104-114) with:

```tsx
      .then((drivers) => {
        if (cancelled) return;
        setCodexAvailable(
          drivers.some((driver) => driver.name === "codex" && driver.available)
        );
        setAntigravityAvailable(
          drivers.some((driver) => driver.name === "antigravity" && driver.available)
        );
      })
      .catch(() => {
        if (!cancelled) {
          setCodexAvailable(false);
          setAntigravityAvailable(false);
        }
      });
```

(c) Add the Antigravity `<option>` to the `<select>`, after the codex option (after line 179):

```tsx
          <option value="antigravity" disabled={!antigravityAvailable} title={!antigravityAvailable ? "antigravity not installed" : undefined}>
            Antigravity{antigravityAvailable ? "" : " (antigravity not installed)"}
          </option>
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd web && npm run test -- --run AIGeneratePanel`
Expected: PASS (new tests + all existing ones).

- [ ] **Step 5: Commit**

```bash
git add web/src/components/wizard/AIGeneratePanel.tsx web/src/components/wizard/AIGeneratePanel.test.tsx
git commit -m "feat(web): add Antigravity option to AI Generate panel

Gated on agy availability, mirroring the existing codex gating.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 9: Pre-launch babysitting warning

**Files:**
- Modify: `web/src/components/wizard/ReviewStep.tsx`
- Test: `web/src/components/wizard/ReviewStep.test.tsx`

- [ ] **Step 1: Write the failing tests**

Append to `web/src/components/wizard/ReviewStep.test.tsx`, inside the `describe("ReviewStep", ...)` block. The shared `agents` fixture (claude-code + codex) has no antigravity agent, so the warning must be absent by default:

```tsx
  it("does not show the antigravity babysitting warning without an antigravity agent", () => {
    render(<ReviewStep {...defaultProps} />);
    expect(screen.queryByText(/babysit/i)).not.toBeInTheDocument();
  });

  it("shows the antigravity babysitting warning when an antigravity agent is present", () => {
    const withAgy: SquadronAgent[] = [
      ...agents,
      { name: "agy-agent", branch: "feat/agy", prompt: "do agy", driver: "antigravity", persona: "" },
    ];
    render(<ReviewStep {...defaultProps} agents={withAgy} />);
    expect(screen.getByText(/babysit/i)).toBeInTheDocument();
    // Advisory only — launch stays enabled.
    expect(screen.getByRole("button", { name: /launch squadron/i })).not.toBeDisabled();
  });
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd web && npm run test -- --run ReviewStep`
Expected: FAIL — "shows the antigravity babysitting warning" can't find the text.

- [ ] **Step 3: Render the warning**

In `web/src/components/wizard/ReviewStep.tsx`, compute the flag near the top of the component body (after `const [state, dispatch] = useReducer(...)`, line 108):

```tsx
  const hasAntigravity = agents.some((a) => a.driver === "antigravity");
```

Then render the banner just above the action buttons — immediately before the `<div style={{ display: "flex", gap: "1rem" }}>` that holds the Launch button (line 235):

```tsx
      {hasAntigravity && (
        <div
          role="status"
          style={{
            display: "flex",
            gap: "0.5rem",
            alignItems: "flex-start",
            border: "1px solid var(--orange)",
            borderRadius: 8,
            padding: "0.75rem 1rem",
            marginBottom: "1rem",
            color: "var(--orange)",
            fontSize: "0.85rem",
          }}
        >
          <span aria-hidden="true">⚠️</span>
          <span>
            Antigravity agents don't have a true bypass-permissions flag. You may
            need to babysit this agent and respond to its approval prompts instead
            of leaving it unattended.
          </span>
        </div>
      )}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd web && npm run test -- --run ReviewStep`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/wizard/ReviewStep.tsx web/src/components/wizard/ReviewStep.test.tsx
git commit -m "feat(web): warn before launching antigravity agents

agy has no bypass-permissions flag, so squadron 'yolo' is best-effort —
surface that with an advisory pre-launch banner.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Task 10: Pre-launch cost-analysis info panel

**Files:**
- Modify: `web/src/components/wizard/ReviewStep.tsx`
- Test: `web/src/components/wizard/ReviewStep.test.tsx`

- [ ] **Step 1: Write the failing tests**

Append to `web/src/components/wizard/ReviewStep.test.tsx`, inside the `describe("ReviewStep", ...)` block. The default fixture is all cost-supported (claude-code + codex), so the panel must be absent there:

```tsx
  it("does not show the cost-analysis info panel for cost-supported drivers", () => {
    render(<ReviewStep {...defaultProps} />);
    expect(screen.queryByText(/does not have cost analysis enabled/i)).not.toBeInTheDocument();
  });

  it("shows the cost-analysis info panel naming a single unsupported harness", () => {
    const withAgy: SquadronAgent[] = [
      agents[0], // claude-code (supported)
      { name: "agy-agent", branch: "feat/agy", prompt: "do agy", driver: "antigravity", persona: "" },
    ];
    render(<ReviewStep {...defaultProps} agents={withAgy} />);
    const panel = screen.getByText(/does not have cost analysis enabled/i);
    expect(panel).toBeInTheDocument();
    expect(panel).toHaveTextContent("(antigravity)");
    expect(panel).toHaveTextContent(/harness \(/i); // singular
    expect(screen.getByRole("button", { name: /launch squadron/i })).not.toBeDisabled();
  });

  it("names multiple distinct unsupported harnesses in the cost panel", () => {
    const mixed: SquadronAgent[] = [
      { name: "a", branch: "b1", prompt: "p", driver: "aider", persona: "" },
      { name: "b", branch: "b2", prompt: "p", driver: "antigravity", persona: "" },
    ];
    render(<ReviewStep {...defaultProps} agents={mixed} />);
    const panel = screen.getByText(/do not have cost analysis enabled/i); // plural
    expect(panel).toHaveTextContent("aider");
    expect(panel).toHaveTextContent("antigravity");
  });
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd web && npm run test -- --run ReviewStep`
Expected: FAIL — no cost-analysis panel exists yet.

- [ ] **Step 3: Render the info panel**

In `web/src/components/wizard/ReviewStep.tsx`, add the import for the cost-unsupported set to the existing `review-constants` import (line 3 currently imports `ConsensusType`):

```tsx
import type { ConsensusType } from "./review-constants";
import { costUnsupportedDrivers } from "./review-constants";
```

Compute the distinct unsupported harnesses next to `hasAntigravity` (from Task 9):

```tsx
  const unsupportedCostDrivers = [...new Set(
    agents.map((a) => a.driver).filter((d) => costUnsupportedDrivers.includes(d))
  )];
```

Render the info panel directly **below** the babysitting warning (still above the action-buttons `<div>`):

```tsx
      {unsupportedCostDrivers.length > 0 && (
        <div
          role="status"
          style={{
            display: "flex",
            gap: "0.5rem",
            alignItems: "flex-start",
            borderLeft: "3px solid var(--border)",
            background: "var(--bg-secondary)",
            borderRadius: 6,
            padding: "0.75rem 1rem",
            marginBottom: "1rem",
            color: "var(--text-secondary)",
            fontSize: "0.85rem",
          }}
        >
          <span aria-hidden="true">ℹ️</span>
          <span>
            The selected {unsupportedCostDrivers.length === 1 ? "harness" : "harnesses"}{" "}
            ({unsupportedCostDrivers.join(", ")}){" "}
            {unsupportedCostDrivers.length === 1 ? "does" : "do"} not have cost
            analysis enabled due to tooling limitations and will display blank.
          </span>
        </div>
      )}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd web && npm run test -- --run ReviewStep`
Expected: PASS (single + plural + absent cases, non-blocking).

- [ ] **Step 5: Commit**

```bash
git add web/src/components/wizard/ReviewStep.tsx web/src/components/wizard/ReviewStep.test.tsx
git commit -m "feat(web): inform when a harness has no cost analysis

aider/generic/antigravity have no ccusage source; surface the blank cost
column as expected via an advisory info panel.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

- [ ] **Step 6: Run the full web suite + lint**

Run: `cd web && npm run test -- --run && npx tsc --noEmit`
Expected: all tests pass, typecheck clean.

---

## Task 11: Documentation

**Files:**
- Modify: `CLAUDE.md:45`, `CLAUDE.md:67`
- Modify: `AGENTS.md:52`
- Modify: `README.md:120`, `README.md:316-322` (driver table)

- [ ] **Step 1: Update CLAUDE.md driver enumerations**

In `CLAUDE.md`, line 45, change the `Driver` list:

```
- `Agent` — `Name` (unique, used as tmux session: `fleet-<Name>`), `Branch`, `WorktreePath`, `Driver` (one of `claude-code`, `codex`, `aider`, `kimi-code`, `antigravity`, `generic`), `StateFile`, optional `Persona`, `FightMode` bool.
```

And line 67:

```
- `internal/driver/` — Driver interface and implementations (`claude-code`, `codex`, `aider`, `kimi-code`, `antigravity`, `generic`). Each driver implements state detection, hook injection (where supported), and command building.
```

- [ ] **Step 2: Update AGENTS.md**

In `AGENTS.md`, line 52:

```
- `Driver` — which agent CLI drives this session (`claude-code`, `codex`, `aider`, `kimi-code`, `antigravity`, `generic`)
```

- [ ] **Step 3: Update README.md driver references**

In `README.md`, line 120:

```
| `fleet add <name> <branch> --driver <name>` | Add an agent backed by a specific driver (`claude-code`, `codex`, `aider`, `kimi-code`, `antigravity`, `generic`) |
```

And add a row to the driver table (after the `kimi-code` row, line 321):

```
| `antigravity` | [Google Antigravity](https://antigravity.google) | Pane-scrape state detection; headless planning via `agy -p`. No bypass flag — squadron yolo is best-effort. No `ccusage` source, so cost shows blank. |
```

- [ ] **Step 4: Verify the doc edits**

Run: `grep -n "antigravity" CLAUDE.md AGENTS.md README.md`
Expected: matches on all the edited lines.

- [ ] **Step 5: Commit**

```bash
git add CLAUDE.md AGENTS.md README.md
git commit -m "docs: document the antigravity driver

No test added — documentation-only change.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>"
```

---

## Final verification

- [ ] **Step 1: Full backend suite + vet**

Run: `go test ./... && go vet ./...`
Expected: PASS, no warnings.

- [ ] **Step 2: Full web suite + typecheck**

Run: `cd web && npm run test -- --run && npx tsc --noEmit`
Expected: PASS, typecheck clean.

- [ ] **Step 3: End-to-end build (embedded SPA)**

Run: `make build-all`
Expected: builds the web UI, embeds it, installs the `fleet` binary with no errors. (Confirms the generated icon module bundles cleanly into the embedded SPA.)

- [ ] **Step 4: Manual smoke (optional, if `agy` is installed)**

Run: `agy --help` to confirm `-p` exists and no bypass flag is documented; if installed, capture a real pane via `tmux capture-pane` to validate/tune `DetectState` patterns in `internal/driver/antigravity.go`.

---

## Self-review notes

- **Spec coverage:** driver (Tasks 1-2), availability API (Task 3), AI-Generate (Task 4), icon (Tasks 5-6), driver colors (Task 7), AI-Generate option (Task 8), babysitting warning (Task 9), cost info panel (Task 10), docs (Task 11). Cost reporting is intentionally *not* wired (spec "Cost reporting: intentionally unsupported") — Task 10 surfaces the blank column instead; no `cost.driverSource` change.
- **Type consistency:** `costUnsupportedDrivers` (Task 7) is consumed in Task 10; `ANTIGRAVITY_ICON_DATA_URI` (Task 5) is consumed in Task 5's component; `antigravityAvailable` (Task 8) is self-contained. `driverBinaries["antigravity"] = "agy"` (Task 3) is what makes the Task 4 availability check resolve.
- **Ordering:** Task 3 (driverBinaries) precedes Task 4 (which relies on it). Task 7 (constant) precedes Task 10 (consumer). Task 5 (icon) precedes Task 6 (icon wiring).
