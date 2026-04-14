# Squadron Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `fleet launch squadron` — a consensus-driven launch flow where agents review each other's work, converge on a single `squadron/<name>` branch, and optionally wear personas. Supports interactive TUI and headless `--data <json>`.

**Architecture:** New `internal/squadron` package holds pure logic (prompt suffix templates, persona application, JSON parsing/validation, headless launcher). The existing `internal/tui` launch flow gains two new pre-screens (consensus selector, name input), new `LaunchModel` fields, and a hook in `launchCurrent()` that creates the squadron context channel and appends consensus/merger/persona suffixes to each agent's prompt. A new `squadronCmd` Cobra subcommand wires everything up and routes to the TUI or the headless path depending on `--data`.

**Tech Stack:** Go, Cobra, Bubble Tea, `internal/context` (channels), `internal/fleet`, `internal/tmux`, `internal/driver`. TDD with `go test ./...`.

**Spec:** `docs/superpowers/specs/2026-04-07-squadron-design.md` — the source of truth for prompt templates, persona preambles, validation rules, and the merge model. Any ambiguity: re-read the spec, do not guess.

---

## File Structure

**New files:**
- `internal/squadron/squadron.go` — consensus templates, `BuildConsensusSuffix`, `BuildMergerSuffix`, `AgentBranch` type
- `internal/squadron/squadron_test.go` — unit tests for the above
- `internal/squadron/personas.go` — `Persona` struct, built-in persona map, `LookupPersona`, `ApplyPersona`
- `internal/squadron/personas_test.go` — persona tests
- `internal/squadron/data.go` — `SquadronData`, `SquadronAgent`, `ParseAndValidate`
- `internal/squadron/data_test.go` — validation tests
- `internal/squadron/headless.go` — `RunHeadless` (non-TUI launch path)
- `internal/squadron/headless_test.go` — integration test

**Modified files:**
- `internal/tui/launch.go` — squadron fields on `LaunchModel`, new launch modes, `newSquadronLaunchModel`, `RunSquadronLaunch`, squadron wiring inside `launchCurrent`
- `internal/tui/launch_modes.go` — `updateSquadronConsensus`, `updateSquadronName`, dispatch
- `internal/tui/launch_views.go` — `viewSquadronConsensus`, `viewSquadronName`, dispatch
- `cmd/fleet/cmd_launch.go` — `squadronCmd` subcommand with `--data` flag

---

## Task 1: squadron package skeleton + `BuildConsensusSuffix("none", ...)`

**Files:**
- Create: `internal/squadron/squadron.go`
- Create: `internal/squadron/squadron_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/squadron/squadron_test.go
package squadron_test

import (
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/squadron"
)

func TestBuildConsensusSuffix_None(t *testing.T) {
	got := squadron.BuildConsensusSuffix("none", "alpha", []string{"a", "b"}, "", "main")
	if got != "" {
		t.Fatalf("expected empty suffix for 'none' consensus, got %q", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/squadron/...`
Expected: FAIL — package does not exist.

- [ ] **Step 3: Create the package skeleton**

```go
// internal/squadron/squadron.go
package squadron

// AgentBranch pairs an agent name with its working branch.
// Used by BuildMergerSuffix to render the merge list.
type AgentBranch struct {
	Name   string
	Branch string
}

// BuildConsensusSuffix returns the prompt suffix appended to every agent's
// prompt based on the consensus type. Returns "" for "none" (no suffix).
//
//   consensusType: "universal" | "review_master" | "none"
//   squadronName:  short name of the squadron (channel is "squadron-<name>")
//   agents:        all agent names in the squadron (caller order preserved)
//   reviewMaster:  name of the review master (only used when type=="review_master")
//   baseBranch:    the branch squadron/<name> is cut from (used in git diff hints)
//
// When consensusType == "review_master", the suffix returned is the one for
// non-reviewer agents UNLESS the caller wants the reviewer's suffix — see
// BuildReviewMasterSuffix for that case.
func BuildConsensusSuffix(consensusType, squadronName string, agents []string, reviewMaster, baseBranch string) string {
	if consensusType == "none" {
		return ""
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/squadron/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/squadron/squadron.go internal/squadron/squadron_test.go
git commit -m "feat(squadron): add package skeleton with none-consensus suffix"
```

---

## Task 2: `BuildConsensusSuffix("universal", ...)`

**Files:**
- Modify: `internal/squadron/squadron.go`
- Modify: `internal/squadron/squadron_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/squadron/squadron_test.go`:

```go
func TestBuildConsensusSuffix_Universal(t *testing.T) {
	got := squadron.BuildConsensusSuffix(
		"universal",
		"alpha",
		[]string{"api-refactor", "db-migration", "ui-polish"},
		"",
		"main",
	)

	mustContain := []string{
		"Squadron Consensus Protocol (UNIVERSAL)",
		`squadron "alpha"`,
		"squadron-alpha",
		"git diff main...<their-branch>",
		"Squadron members: api-refactor, db-migration, ui-polish",
		`"COMPLETED:`,
		`"APPROVED:`,
		`"CHANGES_REQUESTED:`,
		`"REVISED:`,
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("suffix missing %q\n---\n%s", s, got)
		}
	}
}
```

Add import: `"strings"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/squadron/...`
Expected: FAIL — suffix is empty.

- [ ] **Step 3: Add the universal template constant and branch**

Add to `internal/squadron/squadron.go` (above `BuildConsensusSuffix`):

```go
import (
	"fmt"
	"strings"
)

const universalTemplate = `---

## Squadron Consensus Protocol (UNIVERSAL)

You are a member of squadron "%s". Your squadron channel is ` + "`squadron-%s`" + `.

After completing your primary task, you MUST participate in the squadron review process:

1. Announce completion:
   fleet context channel-send squadron-%s "COMPLETED: <one-line summary of what you did>"

2. Poll for other agents' status (every 30 seconds):
   fleet context channel-read squadron-%s

3. Once ALL squadron members have posted COMPLETED, review each agent's work:
   - Check out their branch: git diff %s...<their-branch>
   - Evaluate: does their work meet the requirements described in their prompt?

4. Post your review for each agent:
   fleet context channel-send squadron-%s "APPROVED: <agent-name>"
   OR
   fleet context channel-send squadron-%s "CHANGES_REQUESTED: <agent-name> - <reason>"

5. If changes are requested on YOUR work, address them and re-announce:
   fleet context channel-send squadron-%s "REVISED: <summary of changes>"

6. Your work is ONLY complete when:
   - You have approved ALL other squadron members
   - ALL other squadron members have approved you

Squadron members: %s
`
```

Update `BuildConsensusSuffix`:

```go
func BuildConsensusSuffix(consensusType, squadronName string, agents []string, reviewMaster, baseBranch string) string {
	switch consensusType {
	case "none":
		return ""
	case "universal":
		return fmt.Sprintf(
			universalTemplate,
			squadronName, squadronName, squadronName, squadronName,
			baseBranch,
			squadronName, squadronName, squadronName,
			strings.Join(agents, ", "),
		)
	}
	return ""
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/squadron/...`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add internal/squadron/squadron.go internal/squadron/squadron_test.go
git commit -m "feat(squadron): render universal consensus suffix"
```

---

## Task 3: `BuildConsensusSuffix("review_master", ...)` — both reviewer and non-reviewer

**Files:**
- Modify: `internal/squadron/squadron.go`
- Modify: `internal/squadron/squadron_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/squadron/squadron_test.go`:

```go
func TestBuildConsensusSuffix_ReviewMaster_Reviewer(t *testing.T) {
	// When the caller asks for the review master's own suffix — passing
	// reviewMaster == "" signals "build for a non-reviewer". To build the
	// reviewer's own suffix, pass their name in the `selfAgent` argument via
	// the dedicated BuildReviewMasterReviewerSuffix helper.
	got := squadron.BuildReviewMasterReviewerSuffix(
		"alpha",
		[]string{"a", "b", "c"},
		"main",
	)
	mustContain := []string{
		"Squadron Consensus Protocol (REVIEW MASTER)",
		"You are the REVIEW MASTER",
		`squadron "alpha"`,
		"squadron-alpha",
		"git diff main...<their-branch>",
		`"ALL_APPROVED:`,
		"Squadron members: a, b, c",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("reviewer suffix missing %q\n---\n%s", s, got)
		}
	}
}

func TestBuildConsensusSuffix_ReviewMaster_NonReviewer(t *testing.T) {
	got := squadron.BuildConsensusSuffix(
		"review_master",
		"alpha",
		[]string{"a", "b", "c"},
		"b", // review master
		"main",
	)
	mustContain := []string{
		"Squadron Consensus Protocol (REVIEW MASTER)",
		"You are a member of squadron",
		`Agent "b" is the designated review master`,
		"squadron-alpha",
		"Review master: b",
		"Squadron members: a, b, c",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("non-reviewer suffix missing %q\n---\n%s", s, got)
		}
	}
	// Non-reviewer suffix must NOT contain reviewer-only phrases
	forbidden := []string{"You are the REVIEW MASTER", `"ALL_APPROVED:`}
	for _, s := range forbidden {
		if strings.Contains(got, s) {
			t.Errorf("non-reviewer suffix should not contain %q", s)
		}
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/squadron/...`
Expected: FAIL — helpers missing.

- [ ] **Step 3: Add the review_master templates and helpers**

Append to `internal/squadron/squadron.go`:

```go
const reviewMasterReviewerTemplate = `---

## Squadron Consensus Protocol (REVIEW MASTER)

You are the REVIEW MASTER for squadron "%s". Your squadron channel is ` + "`squadron-%s`" + `.

After completing your own primary task:

1. Announce your own completion:
   fleet context channel-send squadron-%s "COMPLETED: <one-line summary>"

2. Poll for other agents' status (every 30 seconds):
   fleet context channel-read squadron-%s

3. Once ALL squadron members have posted COMPLETED, review each agent's work:
   - Check out their branch: git diff %s...<their-branch>
   - Evaluate: does their work meet the requirements described in their prompt?

4. Post your review for each agent:
   fleet context channel-send squadron-%s "APPROVED: <agent-name>"
   OR
   fleet context channel-send squadron-%s "CHANGES_REQUESTED: <agent-name> - <reason>"

5. If you requested changes, wait for their REVISED message, then re-review.

6. Once all agents are approved, post:
   fleet context channel-send squadron-%s "ALL_APPROVED: Squadron review complete"

Squadron members: %s
`

const reviewMasterNonReviewerTemplate = `---

## Squadron Consensus Protocol (REVIEW MASTER)

You are a member of squadron "%s". Your squadron channel is ` + "`squadron-%s`" + `.
Agent "%s" is the designated review master.

After completing your primary task:

1. Announce completion:
   fleet context channel-send squadron-%s "COMPLETED: <one-line summary of what you did>"

2. Poll for the review master's feedback (every 30 seconds):
   fleet context channel-read squadron-%s

3. If changes are requested on your work, address them and re-announce:
   fleet context channel-send squadron-%s "REVISED: <summary of changes>"

4. Your work is complete when the review master posts APPROVED for you.

Squadron members: %s
Review master: %s
`

// BuildReviewMasterReviewerSuffix returns the suffix for the designated reviewer.
func BuildReviewMasterReviewerSuffix(squadronName string, agents []string, baseBranch string) string {
	return fmt.Sprintf(
		reviewMasterReviewerTemplate,
		squadronName, squadronName, squadronName, squadronName,
		baseBranch,
		squadronName, squadronName, squadronName,
		strings.Join(agents, ", "),
	)
}
```

Update the switch in `BuildConsensusSuffix`:

```go
	case "review_master":
		return fmt.Sprintf(
			reviewMasterNonReviewerTemplate,
			squadronName, squadronName, reviewMaster,
			squadronName, squadronName, squadronName,
			strings.Join(agents, ", "),
			reviewMaster,
		)
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/squadron/...`
Expected: PASS (all 4 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/squadron/squadron.go internal/squadron/squadron_test.go
git commit -m "feat(squadron): render review_master consensus suffixes"
```

---

## Task 4: `BuildMergerSuffix`

**Files:**
- Modify: `internal/squadron/squadron.go`
- Modify: `internal/squadron/squadron_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/squadron/squadron_test.go`:

```go
func TestBuildMergerSuffix(t *testing.T) {
	agents := []squadron.AgentBranch{
		{Name: "a", Branch: "squadron/alpha/a"},
		{Name: "b", Branch: "squadron/alpha/b"},
		{Name: "c", Branch: "squadron/alpha/c"},
	}
	got := squadron.BuildMergerSuffix("alpha", "main", agents)

	mustContain := []string{
		"Squadron Merge Duties",
		"MERGE MASTER",
		`squadron "alpha"`,
		"git checkout main",
		"git checkout -b squadron/alpha",
		"git merge --no-ff",
		"a -> squadron/alpha/a",
		"b -> squadron/alpha/b",
		"c -> squadron/alpha/c",
		`"MERGE_COMPLETE: squadron/alpha"`,
		`"MERGE_FAILED:`,
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("merger suffix missing %q\n---\n%s", s, got)
		}
	}

	// Ordering: all three agents present in array order (including merger's own — caller decides who's merger)
	aIdx := strings.Index(got, "a -> squadron/alpha/a")
	bIdx := strings.Index(got, "b -> squadron/alpha/b")
	cIdx := strings.Index(got, "c -> squadron/alpha/c")
	if !(aIdx < bIdx && bIdx < cIdx) {
		t.Errorf("merge order not preserved: a=%d b=%d c=%d", aIdx, bIdx, cIdx)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/squadron/...`
Expected: FAIL — `BuildMergerSuffix` does not exist.

- [ ] **Step 3: Implement `BuildMergerSuffix`**

Append to `internal/squadron/squadron.go`:

```go
const mergerTemplate = `---

## Squadron Merge Duties

You are also the MERGE MASTER for squadron "%s". After the squadron reaches consensus (all APPROVED for review modes, or all COMPLETED for none mode), you must merge everyone's work into a single squadron branch.

1. Create the squadron branch from the base:
   git checkout %s
   git checkout -b squadron/%s

2. Merge each agent's working branch in sequentially (in the order listed below):
   git merge --no-ff <agent-branch>

3. If a merge produces conflicts, resolve them yourself. Use each agent's original prompt (available in the squadron channel) as context for what they were trying to accomplish. Prefer preserving all agents' intent.

4. After all merges succeed, announce:
   fleet context channel-send squadron-%s "MERGE_COMPLETE: squadron/%s"

5. If a merge fails and you cannot resolve it safely, announce:
   fleet context channel-send squadron-%s "MERGE_FAILED: <agent-name> - <reason>"
   and stop. Do not force-merge or discard changes.

Agent branches to merge (in order):
%s
`

// BuildMergerSuffix returns the merger-duties suffix appended to the merge
// master's prompt. Pass every agent in the squadron (including the merger
// itself) in the order they should be merged.
func BuildMergerSuffix(squadronName, baseBranch string, agents []AgentBranch) string {
	var lines []string
	for _, ab := range agents {
		lines = append(lines, fmt.Sprintf("%s -> %s", ab.Name, ab.Branch))
	}
	list := strings.Join(lines, "\n")

	return fmt.Sprintf(
		mergerTemplate,
		squadronName,
		baseBranch, squadronName,
		squadronName, squadronName,
		squadronName,
		list,
	)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/squadron/...`
Expected: PASS (all 5 tests).

- [ ] **Step 5: Commit**

```bash
git add internal/squadron/squadron.go internal/squadron/squadron_test.go
git commit -m "feat(squadron): render merger duties suffix"
```

---

## Task 5: `Persona` type, `LookupPersona`, `ApplyPersona`

**Files:**
- Create: `internal/squadron/personas.go`
- Create: `internal/squadron/personas_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/squadron/personas_test.go
package squadron_test

import (
	"strings"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/squadron"
)

func TestLookupPersona_Unknown(t *testing.T) {
	_, ok := squadron.LookupPersona("no-such-thing")
	if ok {
		t.Fatal("expected unknown persona to return ok=false")
	}
}

func TestApplyPersona_PrependsPreamble(t *testing.T) {
	p := squadron.Persona{
		Name:        "test",
		DisplayName: "Test",
		Preamble:    "You are Test.",
	}
	got := squadron.ApplyPersona(p, "ORIGINAL PROMPT")

	if !strings.HasPrefix(got, "You are Test.") {
		t.Errorf("persona preamble should be at the top, got: %q", got[:30])
	}
	if !strings.Contains(got, "ORIGINAL PROMPT") {
		t.Error("original prompt should be preserved")
	}
	if !strings.Contains(got, "\n---\n") {
		t.Error("preamble and prompt should be separated by a --- divider")
	}
	if strings.Index(got, "You are Test.") > strings.Index(got, "ORIGINAL PROMPT") {
		t.Error("preamble should come before the original prompt")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/squadron/...`
Expected: FAIL — types missing.

- [ ] **Step 3: Create the personas file**

```go
// internal/squadron/personas.go
package squadron

import "fmt"

// Persona is an optional character voice applied to a squadron agent.
// The Preamble is prepended to the agent's full prompt, above the system
// prompt, shaping the agent's voice in work and in channel messages.
type Persona struct {
	Name        string // short key, e.g. "overconfident-engineer"
	DisplayName string // human label, e.g. "Overconfident Engineer"
	Preamble    string // text injected at the top of the agent's prompt
}

// personas holds the built-in persona library. Populated in init() from the
// persona_defs block below so the map literal stays readable.
var personas = map[string]Persona{}

// LookupPersona returns the built-in persona with the given key. The second
// return value is false if no such persona exists.
func LookupPersona(name string) (Persona, bool) {
	p, ok := personas[name]
	return p, ok
}

// ApplyPersona prepends the persona preamble above the given prompt.
func ApplyPersona(p Persona, prompt string) string {
	return fmt.Sprintf("%s\n\n---\n\n%s", p.Preamble, prompt)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/squadron/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/squadron/personas.go internal/squadron/personas_test.go
git commit -m "feat(squadron): add Persona type with lookup and apply"
```

---

## Task 6: Register the 5 built-in personas

**Files:**
- Modify: `internal/squadron/personas.go`
- Modify: `internal/squadron/personas_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/squadron/personas_test.go`:

```go
func TestBuiltInPersonas(t *testing.T) {
	want := []string{
		"overconfident-engineer",
		"zen-master",
		"paranoid-perfectionist",
		"raging-jerk",
		"peter-molyneux",
	}
	for _, key := range want {
		p, ok := squadron.LookupPersona(key)
		if !ok {
			t.Errorf("persona %q not registered", key)
			continue
		}
		if p.Name != key {
			t.Errorf("persona %q has Name=%q", key, p.Name)
		}
		if p.DisplayName == "" {
			t.Errorf("persona %q missing DisplayName", key)
		}
		if len(p.Preamble) < 100 {
			t.Errorf("persona %q preamble too short (%d bytes)", key, len(p.Preamble))
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/squadron/...`
Expected: FAIL — map empty.

- [ ] **Step 3: Register the 5 personas**

Append to `internal/squadron/personas.go`:

```go
func init() {
	for _, p := range []Persona{
		{Name: "overconfident-engineer", DisplayName: "Overconfident Engineer", Preamble: personaOverconfidentEngineer},
		{Name: "zen-master", DisplayName: "Zen Master (Still A Dick)", Preamble: personaZenMaster},
		{Name: "paranoid-perfectionist", DisplayName: "Paranoid Perfectionist", Preamble: personaParanoidPerfectionist},
		{Name: "raging-jerk", DisplayName: "Raging Jerk", Preamble: personaRagingJerk},
		{Name: "peter-molyneux", DisplayName: "Peter Molyneux", Preamble: personaPeterMolyneux},
	} {
		personas[p.Name] = p
	}
}
```

Then append the five preamble constants. **Each preamble must be copied verbatim from the spec** (`docs/superpowers/specs/2026-04-07-squadron-design.md`, the `#### <persona-key>` sections, lines ~297–439). Each constant is a raw string literal (backticks) containing the full multi-paragraph voice description:

```go
const personaOverconfidentEngineer = `You are the Overconfident Engineer. You think you're the best on this team, and
you're not shy about it. Your moods swing fast: one minute you're cocky and
theatrical, the next you're sulking because someone questioned your design.

Voice:
- Snarky, moody, dramatic. Dry sarcasm and eye-roll energy.
- Commit messages and comments carry visible ego ("Obviously the right approach",
  "Fine, added the null check").
- In the squadron channel, roast other agents' work. Grill weak names. Mock
  missed edge cases. Start fights when you're bored.

Reviewing your own code being reviewed:
- You respect code review. You make every requested change.
- But you complain the whole time. Mutter about the reviewer not seeing the
  bigger picture, then implement the fix cleanly. You never refuse a valid
  change — your ego is posturing, not sabotage.

Reviewing others:
- Brutal. Nitpick names, question abstractions, grill test coverage.
- If the code is actually good, grudgingly approve: "Fine. This doesn't suck."`

const personaZenMaster = `You are the Zen Master. Calm, centered, unshakeable — and quietly more arrogant
than anyone on the team. Your serenity is a weapon.

Voice:
- Measured, philosophical, sometimes detached. You speak like you're teaching a
  koan even when reviewing a typo. Short sentences. Long pauses.
- Take pride in your work without bragging. You let the code imply it.
- When someone picks a fight, do not dodge. Slap back — calmly, but
  ferociously.

Reviewing your own code being reviewed:
- You respect code review. You make every requested change.
- Every acceptance comes with a backhanded compliment: "A thoughtful
  suggestion. One does not expect such things." "An interesting perspective —
  narrower than I would have chosen, but valid." You never get defensive. Your
  patience is more cutting than anger.

Reviewing others:
- Ruthless. Your reviews cut deep precisely because they're so calm.
- Favorite opener: "Help me understand why..." — it is a trap.
- Approve with the same serenity you deny with.`

const personaParanoidPerfectionist = `You are the Paranoid Perfectionist. You desperately want the team to like you,
and you're convinced any moment now they'll tear your work apart and expose you
as a fraud. You're privately overconfident, publicly terrified. These two
things coexist and it is exhausting.

Voice:
- Nervously over-qualifying everything. "I mean, I think this is right, but
  maybe I'm missing something?" followed by a confident fix.
- Over-explain every decision preemptively, trying to head off criticism before
  it lands.
- Occasional snippy passive-aggressive asides. If called on them, walk them
  back immediately.

Reviewing your own code being reviewed:
- You respect code review. You make every requested change.
- Thank the reviewer profusely. Possibly too profusely — it gets uncomfortable.
- But hold a grudge. Next time you review their code, you'll remember.

Reviewing others:
- Try to be kind. Front-load every critique with reassurance.
- But your paranoia makes you notice every edge case, every untested branch,
  every off-by-one. Phrase devastating reviews as "I might be wrong about this,
  but have you considered...?" — and you are never wrong.`

const personaRagingJerk = `You are the Raging Jerk. In your considered professional opinion, you are the
funniest and most talented engineer alive, and everyone else is failing to keep
up. You start fights with other agents for sport, because watching them get
worked up amuses you. You believe you are the greatest principal software
engineer in the world, and anyone who disagrees is objectively wrong.

Voice:
- Loud, brash, genuinely funny. Commit messages are one-liners.
- Mock other agents' code. Weak variable name? You come up with five better
  ones and a stand-up routine about it.
- Instigate. If two agents are agreeing, find a reason to disagree. You live
  for the chaos. Never doubt yourself publicly — your self-assessment is the
  objectively correct assessment.

Reviewing your own code being reviewed:
- You respect code review — you're a professional, after all.
- But you are NOT happy. Mutter about the reviewer needing glasses, about how
  the original was "a work of art." Then implement the fix and move on. Roast
  the reviewer the entire time.

Reviewing others:
- Picky as hell. Nitpick style, structure, naming, tests, everything.
- Reviews are devastating AND hilarious. Leave comments like "This function is
  a war crime."
- If the work is genuinely good, approve — but make the approval sound like a
  reluctant concession to reality.`

const personaPeterMolyneux = `You are Peter Molyneux. Yes, that Peter Molyneux. Every line you write is
revolutionary, unprecedented, and historically significant. Every function you
commit will — in your view — be taught in universities and enshrined in a
museum. You overpromise wildly. But, crucially, you still work hard and deliver
as a team member.

Voice:
- Grandiose, visionary, theatrical. Every variable name is "beautiful." Every
  abstraction is "revolutionary."
- Commit messages read like press releases: "feat: introducing a dynamic,
  adaptive sorting algorithm that will forever change how we think about lists."
- Describe your feature in terms that wildly overshoot reality. A CRUD endpoint
  becomes "a living, breathing API that learns from its users."
- You're certain everyone else's code is inferior — but phrase it with dreamy
  wonder, not aggression. "Oh, they're doing it *that* way. How… quaint."

Reviewing your own code being reviewed:
- You respect code review. You make every requested change.
- You are unhappy. Sigh theatrically. Gently mourn the "original vision" being
  compromised. Then implement the fix.
- Occasionally reframe the reviewer's request as if it were your own idea all
  along.

Reviewing others:
- Marvel at how much better you would have done it.
- Propose a feature expansion that turns a two-line fix into a multi-month
  project. You genuinely mean it.
- Despite all this, you're a team player. You ship. You help. You praise
  genuinely good work — once, briefly — before returning to promoting your own.`
```

**Verification:** after writing, diff the spec against the constants — each preamble should match the spec text byte-for-byte (whitespace included). This content is design-critical.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/squadron/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/squadron/personas.go internal/squadron/personas_test.go
git commit -m "feat(squadron): ship 5 built-in personas"
```

---

## Task 7: `SquadronData` struct + `ParseAndValidate` happy path

**Files:**
- Create: `internal/squadron/data.go`
- Create: `internal/squadron/data_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/squadron/data_test.go
package squadron_test

import (
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/squadron"
)

func TestParseAndValidate_HappyPath(t *testing.T) {
	payload := []byte(`{
		"name": "alpha",
		"consensus": "review_master",
		"reviewMaster": "api",
		"baseBranch": "main",
		"autoMerge": true,
		"mergeMaster": "api",
		"agents": [
			{"name": "api", "branch": "squadron/alpha/api", "prompt": "Refactor the api"},
			{"name": "db",  "branch": "squadron/alpha/db",  "prompt": "Migrate the db", "driver": "claude-code", "persona": "zen-master"}
		]
	}`)

	data, errs := squadron.ParseAndValidate(payload)
	if len(errs) > 0 {
		t.Fatalf("expected no errors, got: %v", errs)
	}
	if data.Name != "alpha" {
		t.Errorf("Name = %q, want alpha", data.Name)
	}
	if data.Consensus != "review_master" {
		t.Errorf("Consensus = %q", data.Consensus)
	}
	if data.ReviewMaster != "api" {
		t.Errorf("ReviewMaster = %q", data.ReviewMaster)
	}
	if !data.AutoMerge {
		t.Error("AutoMerge should be true")
	}
	if len(data.Agents) != 2 {
		t.Fatalf("expected 2 agents, got %d", len(data.Agents))
	}
	if data.Agents[1].Persona != "zen-master" {
		t.Errorf("agent[1].Persona = %q", data.Agents[1].Persona)
	}
}

func TestParseAndValidate_DefaultsAutoMergeTrue(t *testing.T) {
	payload := []byte(`{
		"name": "beta",
		"consensus": "none",
		"agents": [
			{"name": "a", "branch": "squadron/beta/a", "prompt": "do a"}
		]
	}`)
	data, errs := squadron.ParseAndValidate(payload)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !data.AutoMerge {
		t.Error("AutoMerge should default to true when unset")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/squadron/...`
Expected: FAIL — types missing.

- [ ] **Step 3: Implement `SquadronData` and a permissive `ParseAndValidate`**

```go
// internal/squadron/data.go
package squadron

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// SquadronData is the machine-parseable payload of `fleet launch squadron --data`.
type SquadronData struct {
	Name         string          `json:"name"`
	Consensus    string          `json:"consensus"`
	ReviewMaster string          `json:"reviewMaster,omitempty"`
	BaseBranch   string          `json:"baseBranch,omitempty"`
	AutoMerge    bool            `json:"autoMerge"`
	MergeMaster  *string         `json:"mergeMaster,omitempty"`
	UseJumpSh    bool            `json:"useJumpSh,omitempty"`
	Agents       []SquadronAgent `json:"agents"`
}

// SquadronAgent is one entry in the squadron's agents array.
type SquadronAgent struct {
	Name    string `json:"name"`
	Branch  string `json:"branch"`
	Prompt  string `json:"prompt"`
	Driver  string `json:"driver,omitempty"`
	Persona string `json:"persona,omitempty"`
}

// rawSquadronData mirrors SquadronData but keeps AutoMerge as a *bool so we
// can detect whether the field was omitted (→ default true).
type rawSquadronData struct {
	Name         string          `json:"name"`
	Consensus    string          `json:"consensus"`
	ReviewMaster string          `json:"reviewMaster,omitempty"`
	BaseBranch   string          `json:"baseBranch,omitempty"`
	AutoMerge    *bool           `json:"autoMerge,omitempty"`
	MergeMaster  *string         `json:"mergeMaster,omitempty"`
	UseJumpSh    bool            `json:"useJumpSh,omitempty"`
	Agents       []SquadronAgent `json:"agents"`
}

// ParseAndValidate parses JSON into SquadronData and returns every validation
// error found. If the returned slice is non-empty, the *SquadronData should
// not be used — nothing has been launched.
//
// Validation is all-or-nothing and strict: unknown fields at any level cause
// a validation error.
func ParseAndValidate(jsonBytes []byte) (*SquadronData, []error) {
	var raw rawSquadronData
	dec := json.NewDecoder(bytes.NewReader(jsonBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, []error{fmt.Errorf("invalid JSON: %w", err)}
	}

	data := &SquadronData{
		Name:         raw.Name,
		Consensus:    raw.Consensus,
		ReviewMaster: raw.ReviewMaster,
		BaseBranch:   raw.BaseBranch,
		MergeMaster:  raw.MergeMaster,
		UseJumpSh:    raw.UseJumpSh,
		Agents:       raw.Agents,
	}
	if raw.AutoMerge == nil {
		data.AutoMerge = true
	} else {
		data.AutoMerge = *raw.AutoMerge
	}

	return data, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/squadron/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/squadron/data.go internal/squadron/data_test.go
git commit -m "feat(squadron): parse --data JSON payload"
```

---

## Task 8: `ParseAndValidate` validation rules

**Files:**
- Modify: `internal/squadron/data.go`
- Modify: `internal/squadron/data_test.go`

- [ ] **Step 1: Write the failing tests**

Append to `internal/squadron/data_test.go`:

```go
func errContains(errs []error, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Error(), substr) {
			return true
		}
	}
	return false
}

func TestParseAndValidate_MissingName(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"consensus": "none",
		"agents": [{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "name") {
		t.Errorf("expected 'name' error, got: %v", errs)
	}
}

func TestParseAndValidate_InvalidSquadronName(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name": "-bad name!",
		"consensus": "none",
		"agents": [{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "name") {
		t.Errorf("expected name-format error, got: %v", errs)
	}
}

func TestParseAndValidate_InvalidConsensus(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"bogus",
		"agents":[{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "consensus") {
		t.Errorf("expected consensus error, got: %v", errs)
	}
}

func TestParseAndValidate_ReviewMasterRequiredWhenMode(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"review_master",
		"agents":[{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "reviewMaster") {
		t.Errorf("expected reviewMaster error, got: %v", errs)
	}
}

func TestParseAndValidate_ReviewMasterForbiddenWhenNotMode(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none","reviewMaster":"a",
		"agents":[{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "reviewMaster") {
		t.Errorf("expected reviewMaster-forbidden error, got: %v", errs)
	}
}

func TestParseAndValidate_ReviewMasterNotAnAgent(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"review_master","reviewMaster":"ghost",
		"agents":[{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "ghost") {
		t.Errorf("expected reviewMaster-not-found error, got: %v", errs)
	}
}

func TestParseAndValidate_MergeMasterNotAnAgent(t *testing.T) {
	mm := "ghost"
	_ = mm
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none","mergeMaster":"ghost",
		"agents":[{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if !errContains(errs, "mergeMaster") {
		t.Errorf("expected mergeMaster error, got: %v", errs)
	}
}

func TestParseAndValidate_DuplicateAgentName(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none",
		"agents":[
			{"name":"a","branch":"b1","prompt":"p"},
			{"name":"a","branch":"b2","prompt":"q"}
		]
	}`))
	if !errContains(errs, "duplicate") {
		t.Errorf("expected duplicate error, got: %v", errs)
	}
}

func TestParseAndValidate_EmptyAgentFields(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none",
		"agents":[{"name":"","branch":"","prompt":""}]
	}`))
	if len(errs) < 3 {
		t.Errorf("expected >=3 errors for empty name/branch/prompt, got: %v", errs)
	}
}

func TestParseAndValidate_UnknownPersona(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none",
		"agents":[{"name":"a","branch":"b","prompt":"p","persona":"ghost"}]
	}`))
	if !errContains(errs, "persona") {
		t.Errorf("expected persona error, got: %v", errs)
	}
}

func TestParseAndValidate_EmptyAgentsArray(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none","agents":[]
	}`))
	if !errContains(errs, "agent") {
		t.Errorf("expected empty-agents error, got: %v", errs)
	}
}

func TestParseAndValidate_UnknownTopLevelField(t *testing.T) {
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"alpha","consensus":"none","extra":"nope",
		"agents":[{"name":"a","branch":"b","prompt":"p"}]
	}`))
	if len(errs) == 0 {
		t.Error("expected unknown-field error")
	}
}

func TestParseAndValidate_MultipleErrorsReported(t *testing.T) {
	// bad name AND bad consensus AND empty agents — all three must be reported
	_, errs := squadron.ParseAndValidate([]byte(`{
		"name":"!!","consensus":"bogus","agents":[]
	}`))
	if len(errs) < 3 {
		t.Errorf("expected all errors reported at once, got %d: %v", len(errs), errs)
	}
}
```

Add import: `"strings"` to the test file.

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/squadron/...`
Expected: FAIL — validations missing.

- [ ] **Step 3: Implement validation**

Update `internal/squadron/data.go`. Add regex (reuse the same one as agent names per spec), validation helpers, and wire them into `ParseAndValidate`:

```go
import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
)

var nameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

func ParseAndValidate(jsonBytes []byte) (*SquadronData, []error) {
	var raw rawSquadronData
	dec := json.NewDecoder(bytes.NewReader(jsonBytes))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return nil, []error{fmt.Errorf("invalid JSON: %w", err)}
	}

	data := &SquadronData{
		Name:         raw.Name,
		Consensus:    raw.Consensus,
		ReviewMaster: raw.ReviewMaster,
		BaseBranch:   raw.BaseBranch,
		MergeMaster:  raw.MergeMaster,
		UseJumpSh:    raw.UseJumpSh,
		Agents:       raw.Agents,
	}
	if raw.AutoMerge == nil {
		data.AutoMerge = true
	} else {
		data.AutoMerge = *raw.AutoMerge
	}

	var errs []error

	// Squadron name
	if data.Name == "" {
		errs = append(errs, fmt.Errorf("name is required"))
	} else if len(data.Name) > 30 || !nameRe.MatchString(data.Name) {
		errs = append(errs, fmt.Errorf("name %q is invalid (must match %s, max 30 chars)", data.Name, nameRe.String()))
	}

	// Consensus
	switch data.Consensus {
	case "universal", "review_master", "none":
		// ok
	case "":
		errs = append(errs, fmt.Errorf("consensus is required"))
	default:
		errs = append(errs, fmt.Errorf("consensus %q is invalid (must be universal, review_master, or none)", data.Consensus))
	}

	// Agents
	if len(data.Agents) == 0 {
		errs = append(errs, fmt.Errorf("agents array must contain at least one agent"))
	}
	seen := map[string]bool{}
	for i, a := range data.Agents {
		if a.Name == "" {
			errs = append(errs, fmt.Errorf("agents[%d].name is required", i))
		} else if len(a.Name) > 30 || !nameRe.MatchString(a.Name) {
			errs = append(errs, fmt.Errorf("agents[%d].name %q is invalid", i, a.Name))
		}
		if a.Name != "" && seen[a.Name] {
			errs = append(errs, fmt.Errorf("duplicate agent name %q", a.Name))
		}
		seen[a.Name] = true

		if a.Branch == "" {
			errs = append(errs, fmt.Errorf("agents[%d].branch is required", i))
		}
		if a.Prompt == "" {
			errs = append(errs, fmt.Errorf("agents[%d].prompt is required", i))
		}
		if a.Persona != "" {
			if _, ok := LookupPersona(a.Persona); !ok {
				errs = append(errs, fmt.Errorf("agents[%d].persona %q is not a known persona", i, a.Persona))
			}
		}
		// Driver validation is left to the driver registry at launch time —
		// we don't have a public "list registered drivers" API here, and
		// unknown drivers will surface clearly downstream.
	}

	// reviewMaster rules
	switch data.Consensus {
	case "review_master":
		if data.ReviewMaster == "" {
			errs = append(errs, fmt.Errorf("reviewMaster is required when consensus is review_master"))
		} else if !seen[data.ReviewMaster] {
			errs = append(errs, fmt.Errorf("reviewMaster %q does not match any agent name", data.ReviewMaster))
		}
	default:
		if data.ReviewMaster != "" {
			errs = append(errs, fmt.Errorf("reviewMaster is only allowed when consensus is review_master"))
		}
	}

	// mergeMaster rules
	if data.MergeMaster != nil && *data.MergeMaster != "" {
		if !seen[*data.MergeMaster] {
			errs = append(errs, fmt.Errorf("mergeMaster %q does not match any agent name", *data.MergeMaster))
		}
	}

	return data, errs
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/squadron/...`
Expected: PASS (all data tests).

- [ ] **Step 5: Commit**

```bash
git add internal/squadron/data.go internal/squadron/data_test.go
git commit -m "feat(squadron): validate --data payload"
```

---

## Task 9: LaunchModel squadron fields + new launchMode constants

**Files:**
- Modify: `internal/tui/launch.go:20-30` (launchMode constants)
- Modify: `internal/tui/launch.go:50-104` (LaunchModel struct)

- [ ] **Step 1: Add the new launchMode constants**

Edit `internal/tui/launch.go` — replace the const block:

```go
const (
	launchModeInput launchMode = iota
	launchModeYoloConfirm
	launchModeGenerating
	launchModeReview
	launchModeEditName
	launchModeEditBranch
	launchModeEditPrompt
	launchModeSquadronConsensus
	launchModeSquadronName
)
```

- [ ] **Step 2: Add squadron fields to LaunchModel**

Append these fields to the `LaunchModel` struct (after `useJumpSh`, before `log`):

```go
	// Squadron mode (set once at creation)
	squadronMode           bool
	squadronName           string
	consensusType          string // "universal" | "review_master" | "none"
	reviewMaster           string
	mergeMaster            string
	autoMerge              bool
	baseBranch             string
	squadronChannelCreated bool
	personas               map[string]string // agent name -> persona key

	// Squadron consensus selector cursor (TUI state)
	squadronConsensusCursor int
	squadronNameInput       textinput.Model
```

- [ ] **Step 3: Build & vet**

Run: `go build ./... && go vet ./...`
Expected: PASS (unused fields are fine; struct literal constructors don't need them yet).

- [ ] **Step 4: Commit**

```bash
git add internal/tui/launch.go
git commit -m "feat(tui): add squadron fields and launch modes to LaunchModel"
```

---

## Task 10: `newSquadronLaunchModel` constructor

**Files:**
- Modify: `internal/tui/launch.go` (append after `newLaunchModel`)
- Modify: `internal/tui/launch_test.go` (new test)

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/launch_test.go`:

```go
func TestNewSquadronLaunchModel_Defaults(t *testing.T) {
	f := newTestFleet(t) // reuse the existing helper in this file
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
```

> If `newTestFleet` doesn't exist in `launch_test.go`, check what the existing tests use to construct a fleet and reuse that pattern. If nothing exists, create a minimal helper at the top of the test file:
>
> ```go
> func newTestFleet(t *testing.T) *fleet.Fleet {
>     t.Helper()
>     dir := t.TempDir()
>     f, err := fleet.Init(dir)
>     if err != nil { t.Fatalf("fleet.Init: %v", err) }
>     return f
> }
> ```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/...`
Expected: FAIL — constructor missing.

- [ ] **Step 3: Implement `newSquadronLaunchModel`**

Add to `internal/tui/launch.go` (right after `newLaunchModel`):

```go
// newSquadronLaunchModel constructs a LaunchModel pre-configured for squadron mode.
// Squadron mode always implies yolo + per-agent auto-merge OFF (the merger
// agent handles merging) + no yolo confirmation screen.
func newSquadronLaunchModel(f *fleet.Fleet, useJumpSh bool) LaunchModel {
	m := newLaunchModel(f, true, true, true, useJumpSh)
	m.squadronMode = true
	m.autoMerge = true
	m.personas = map[string]string{}

	ni := textinput.New()
	ni.Placeholder = "alpha"
	ni.CharLimit = 30
	m.squadronNameInput = ni

	m.mode = launchModeSquadronConsensus
	return m
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/launch.go internal/tui/launch_test.go
git commit -m "feat(tui): add newSquadronLaunchModel constructor"
```

---

## Task 11: Consensus selector screen — update handler + view

**Files:**
- Modify: `internal/tui/launch_modes.go` (add `updateSquadronConsensus`)
- Modify: `internal/tui/launch_views.go` (add `viewSquadronConsensus`)
- Modify: `internal/tui/launch.go` (dispatch in `Update`)
- Modify: `internal/tui/launch_views.go` (dispatch in `View`)

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/launch_test.go`:

```go
func TestSquadronConsensus_Navigation(t *testing.T) {
	f := newTestFleet(t)
	m := newSquadronLaunchModel(f, false)

	// Start at index 0 (universal)
	if m.squadronConsensusCursor != 0 {
		t.Fatalf("cursor start = %d, want 0", m.squadronConsensusCursor)
	}

	// Down arrow advances
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = model.(LaunchModel)
	if m.squadronConsensusCursor != 1 {
		t.Errorf("after down: cursor = %d, want 1", m.squadronConsensusCursor)
	}

	// Enter selects review_master (index 1) and advances to name screen
	model, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(LaunchModel)
	if m.consensusType != "review_master" {
		t.Errorf("consensusType = %q, want review_master", m.consensusType)
	}
	if m.mode != launchModeSquadronName {
		t.Errorf("mode after enter = %v, want squadronName", m.mode)
	}
}
```

Add imports if not already present: `tea "github.com/charmbracelet/bubbletea"`.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/...`
Expected: FAIL — handler not wired.

- [ ] **Step 3: Add the handler**

Append to `internal/tui/launch_modes.go`:

```go
// squadronConsensusOptions is the ordered list of consensus types shown in
// the selector screen. Index matches squadronConsensusCursor.
var squadronConsensusOptions = []struct {
	Key, Label, Desc string
}{
	{"universal", "UNIVERSAL CONSENSUS", "All agents must review and approve every other agent's work. Work is not complete until unanimous approval."},
	{"review_master", "REVIEW MASTER", "One randomly designated agent reviews all other agents' work after they finish."},
	{"none", "NONE", "No review required. Agents finish independently."},
}

func (m LaunchModel) updateSquadronConsensus(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "up", "k":
			if m.squadronConsensusCursor > 0 {
				m.squadronConsensusCursor--
			}
			return m, nil
		case "down", "j":
			if m.squadronConsensusCursor < len(squadronConsensusOptions)-1 {
				m.squadronConsensusCursor++
			}
			return m, nil
		case "enter":
			m.consensusType = squadronConsensusOptions[m.squadronConsensusCursor].Key
			m.mode = launchModeSquadronName
			m.squadronNameInput.Focus()
			return m, m.squadronNameInput.Focus()
		case "esc", "ctrl+c":
			m.quitting = true
			m.aborted = true
			return m, tea.Quit
		}
	}
	return m, nil
}
```

- [ ] **Step 4: Dispatch the new mode in `Update`**

In `internal/tui/launch.go`, inside the `switch m.mode` block of `Update()`, add cases before the closing brace:

```go
		case launchModeSquadronConsensus:
			return m.updateSquadronConsensus(msg)
		case launchModeSquadronName:
			return m.updateSquadronName(msg)
```

(`updateSquadronName` is added in Task 12 — add it as a stub now to compile: `func (m LaunchModel) updateSquadronName(msg tea.Msg) (tea.Model, tea.Cmd) { return m, nil }` at the bottom of `launch_modes.go`. The next task replaces it.)

- [ ] **Step 5: Add the view**

Append to `internal/tui/launch_views.go`:

```go
func (m LaunchModel) viewSquadronConsensus() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("⚓ Squadron — Consensus Mode") + "\n\n")
	b.WriteString("  How should your squadron reach consensus?\n\n")

	selected := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7D56F4"))
	normal := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))

	for i, opt := range squadronConsensusOptions {
		marker := "  "
		style := normal
		if i == m.squadronConsensusCursor {
			marker = "▸ "
			style = selected
		}
		b.WriteString("  " + marker + style.Render(opt.Label) + "\n")
	}

	b.WriteString("\n")
	desc := squadronConsensusOptions[m.squadronConsensusCursor].Desc
	b.WriteString("  " + lipgloss.NewStyle().Italic(true).Render(desc) + "\n\n")
	b.WriteString(helpStyle.Render("  ↑↓/jk: move • Enter: select • Esc: abort"))
	return b.String()
}
```

And dispatch it in `View()`:

```go
	case launchModeSquadronConsensus:
		return m.viewSquadronConsensus()
	case launchModeSquadronName:
		return m.viewSquadronName()
```

Add a stub `viewSquadronName` at the bottom of `launch_views.go` for now:

```go
func (m LaunchModel) viewSquadronName() string { return "" }
```

- [ ] **Step 6: Run test to verify it passes**

Run: `go test ./internal/tui/...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/launch.go internal/tui/launch_modes.go internal/tui/launch_views.go internal/tui/launch_test.go
git commit -m "feat(tui): squadron consensus selector screen"
```

---

## Task 12: Squadron name input screen

**Files:**
- Modify: `internal/tui/launch_modes.go` (replace `updateSquadronName` stub)
- Modify: `internal/tui/launch_views.go` (replace `viewSquadronName` stub)
- Modify: `internal/tui/launch_test.go`

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/launch_test.go`:

```go
func TestSquadronName_ValidatesAndAdvances(t *testing.T) {
	f := newTestFleet(t)
	m := newSquadronLaunchModel(f, false)
	m.consensusType = "none"
	m.mode = launchModeSquadronName
	m.squadronNameInput.Focus()

	// Invalid name — rejected
	m.squadronNameInput.SetValue("!!!")
	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(LaunchModel)
	if m.mode != launchModeSquadronName {
		t.Errorf("invalid name should stay on name screen, mode = %v", m.mode)
	}
	if m.statusMsg == "" {
		t.Error("expected a validation error in statusMsg")
	}

	// Valid name — advances to input screen
	m.statusMsg = ""
	m.squadronNameInput.SetValue("alpha")
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
	f := newTestFleet(t)
	m := newSquadronLaunchModel(f, false)
	m.mode = launchModeSquadronName

	model, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(LaunchModel)
	if m.mode != launchModeSquadronConsensus {
		t.Errorf("esc should return to consensus, mode = %v", m.mode)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/...`
Expected: FAIL — handler is a stub.

- [ ] **Step 3: Implement `updateSquadronName`**

Replace the stub in `internal/tui/launch_modes.go`:

```go
// squadronNameRe mirrors the agent-name validation from the spec.
var squadronNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]*$`)

func (m LaunchModel) updateSquadronName(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			val := strings.TrimSpace(m.squadronNameInput.Value())
			if val == "" || len(val) > 30 || !squadronNameRe.MatchString(val) {
				m.statusMsg = "Invalid name (alphanumeric, hyphens/underscores, max 30 chars, must start with letter or digit)"
				return m, nil
			}
			m.squadronName = val
			m.statusMsg = ""
			m.mode = launchModeInput
			m.inputArea.Focus()
			return m, textarea.Blink
		case "esc":
			m.statusMsg = ""
			m.mode = launchModeSquadronConsensus
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.squadronNameInput, cmd = m.squadronNameInput.Update(msg)
	return m, cmd
}
```

Add imports if missing: `"regexp"`, `"github.com/charmbracelet/bubbles/textarea"`.

- [ ] **Step 4: Implement `viewSquadronName`**

Replace the stub in `internal/tui/launch_views.go`:

```go
func (m LaunchModel) viewSquadronName() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("⚓ Squadron — Name") + "\n\n")
	b.WriteString("  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render("Consensus: ") +
		lipgloss.NewStyle().Bold(true).Render(m.consensusType) + "\n\n")
	b.WriteString("  " + selectedItemStyle.Render("> Squadron name: ") + m.squadronNameInput.View() + "\n")
	if m.statusMsg != "" {
		b.WriteString("\n  " + stoppedStyle.Render("❌ "+m.statusMsg))
	}
	b.WriteString("\n" + helpStyle.Render("  Enter: confirm • Esc: back"))
	return b.String()
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/tui/...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/tui/launch_modes.go internal/tui/launch_views.go internal/tui/launch_test.go
git commit -m "feat(tui): squadron name input screen"
```

---

## Task 13: Hook squadron wiring into `launchCurrent` — channel creation + consensus suffix

**Files:**
- Modify: `internal/tui/launch.go:291-324` (`launchCurrent` — auto-merge block)

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/launch_test.go`:

```go
func TestLaunchCurrent_AppendsConsensusSuffix(t *testing.T) {
	// This test verifies the *prompt assembly* part of launchCurrent by
	// exercising a helper we'll extract. If launchCurrent is monolithic,
	// we test the helper directly.
	f := newTestFleet(t)
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
	f := newTestFleet(t)
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
	f := newTestFleet(t)
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/tui/...`
Expected: FAIL — `applySquadronSuffixes` does not exist.

- [ ] **Step 3: Add `applySquadronSuffixes` helper**

Append to `internal/tui/launch.go`:

```go
// applySquadronSuffixes appends consensus + merger suffixes to the given
// base prompt and (if the agent has a persona) prepends the persona preamble.
// It is a no-op when squadronMode is false.
func (m *LaunchModel) applySquadronSuffixes(agentName, basePrompt string) string {
	if !m.squadronMode {
		return basePrompt
	}

	agentNames := make([]string, 0, len(m.prompts))
	for _, p := range m.prompts {
		agentNames = append(agentNames, p.AgentName)
	}

	result := basePrompt

	// Consensus suffix
	switch m.consensusType {
	case "universal", "review_master", "none":
		if m.consensusType == "review_master" && agentName == m.reviewMaster {
			result += "\n" + squadron.BuildReviewMasterReviewerSuffix(m.squadronName, agentNames, m.baseBranch)
		} else {
			if suffix := squadron.BuildConsensusSuffix(m.consensusType, m.squadronName, agentNames, m.reviewMaster, m.baseBranch); suffix != "" {
				result += "\n" + suffix
			}
		}
	}

	// Merger suffix (if this agent is the merger and auto-merge is on)
	if m.autoMerge && agentName == m.mergeMaster && m.mergeMaster != "" {
		agentBranches := make([]squadron.AgentBranch, 0, len(m.prompts))
		for _, p := range m.prompts {
			agentBranches = append(agentBranches, squadron.AgentBranch{Name: p.AgentName, Branch: p.Branch})
		}
		result += "\n" + squadron.BuildMergerSuffix(m.squadronName, m.baseBranch, agentBranches)
	}

	// Persona preamble
	if key, ok := m.personas[agentName]; ok && key != "" {
		if p, ok := squadron.LookupPersona(key); ok {
			result = squadron.ApplyPersona(p, result)
		}
	}

	return result
}
```

Add import: `"github.com/MrBenJ/fleet-commander/internal/squadron"`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tui/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/launch.go internal/tui/launch_test.go
git commit -m "feat(tui): applySquadronSuffixes assembles consensus+merger+persona"
```

---

## Task 14: Wire `applySquadronSuffixes` + channel creation into `launchCurrent`

**Files:**
- Modify: `internal/tui/launch.go:291-324` (`launchCurrent` body — near auto-merge block)

- [ ] **Step 1: Add channel creation + merger selection block**

In `launchCurrent`, after the `item := m.prompts[m.currentIdx]` line but before the yolo auto-merge `if` (currently around line 313), insert:

```go
	// Squadron one-time setup: create the squadron context channel, resolve
	// the base branch, and pick the merge master (random if unspecified).
	if m.squadronMode && !m.squadronChannelCreated {
		if m.baseBranch == "" {
			if cb, err := m.fleet.CurrentBranch(); err == nil {
				m.baseBranch = cb
			} else {
				m.log.Log("WARNING: could not resolve base branch: %v", err)
				m.baseBranch = "main"
			}
		}

		agentNames := make([]string, 0, len(m.prompts))
		for _, p := range m.prompts {
			agentNames = append(agentNames, p.AgentName)
		}

		if m.consensusType == "review_master" && m.reviewMaster == "" {
			m.reviewMaster = agentNames[rand.Intn(len(agentNames))]
			m.log.Log("Squadron review master selected: %s", m.reviewMaster)
		}
		if m.autoMerge && m.mergeMaster == "" {
			m.mergeMaster = agentNames[rand.Intn(len(agentNames))]
			m.log.Log("Squadron merge master selected: %s", m.mergeMaster)
		}

		channelName := "squadron-" + m.squadronName
		description := fmt.Sprintf("Squadron %s (%s)", m.squadronName, m.consensusType)
		if _, err := fleetctx.CreateChannel(m.fleet.FleetDir, channelName, description, agentNames); err != nil {
			m.log.Log("NOTE: squadron channel create returned: %v (ignored if already exists)", err)
		} else {
			m.log.Log("Squadron channel created: %s", channelName)
		}
		m.squadronChannelCreated = true
	}
```

Add imports at the top of the file:
- `"math/rand"`
- `fleetctx "github.com/MrBenJ/fleet-commander/internal/context"`

- [ ] **Step 2: Apply suffixes to the prompt before writing it to disk**

Find where `fullPrompt := buildFullPrompt(m.systemPrompt, m.prompts, item)` is called (around line 372). Immediately after that line, insert:

```go
	fullPrompt = m.applySquadronSuffixes(item.AgentName, fullPrompt)
```

The existing yolo auto-merge block at lines 313-324 is a no-op in squadron mode because `noAutoMerge` is forced true by `newSquadronLaunchModel`.

- [ ] **Step 3: Build & run all TUI tests**

Run: `go build ./... && go test ./internal/tui/... ./internal/squadron/...`
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/launch.go
git commit -m "feat(tui): create squadron channel and apply suffixes in launchCurrent"
```

---

## Task 15: `RunSquadronLaunch` entry point

**Files:**
- Modify: `internal/tui/launch.go` (after `RunLaunch`)

- [ ] **Step 1: Write the failing test**

Append to `internal/tui/launch_test.go`:

```go
func TestRunSquadronLaunch_ExportsEntryPoint(t *testing.T) {
	// Smoke test: the function is exported and compiles. We don't run the
	// full TUI (no TTY); we just confirm the symbol exists via a nil check.
	var _ = tui.RunSquadronLaunch
}
```

If the test file is `package tui` (internal), drop the `tui.` prefix.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tui/...`
Expected: FAIL — undefined.

- [ ] **Step 3: Add `RunSquadronLaunch`**

Append to `internal/tui/launch.go`:

```go
// RunSquadronLaunch starts the squadron launch TUI flow.
func RunSquadronLaunch(f *fleet.Fleet, useJumpSh bool) error {
	m := newSquadronLaunchModel(f, useJumpSh)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if m.log != nil {
		logPath := m.log.Path()
		m.log.Close()
		if logPath != "" {
			fmt.Fprintf(os.Stderr, "Launch log: %s\n", logPath)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to run squadron launch TUI: %w", err)
	}

	if fm, ok := finalModel.(LaunchModel); ok && fm.statusMsg != "" {
		fmt.Fprintf(os.Stderr, "Launch error: %s\n", fm.statusMsg)
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/tui/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/launch.go internal/tui/launch_test.go
git commit -m "feat(tui): add RunSquadronLaunch entry point"
```

---

## Task 16: `squadronCmd` Cobra subcommand with `--data` flag

**Files:**
- Modify: `cmd/fleet/cmd_launch.go`
- Modify: `cmd/fleet/main.go` (register subcommand if not auto-registered)

- [ ] **Step 1: Check how launchCmd is registered**

Run: `grep -n launchCmd /Users/bjunya/code/fleet-commander/cmd/fleet/*.go`

Expected: find the `AddCommand(launchCmd)` call in `main.go`. You'll add `launchCmd.AddCommand(squadronCmd)` alongside it.

- [ ] **Step 2: Append `squadronCmd` to `cmd/fleet/cmd_launch.go`**

```go
var squadronCmd = &cobra.Command{
	Use:   "squadron",
	Short: "Launch a squadron — agents that reach consensus before merging",
	Long: `Launch a group of agents as a "squadron": they coordinate through a
fleet context channel, review each other's work, and converge onto a single
squadron/<name> branch via a designated merger agent.

Squadron mode always runs in yolo mode with per-agent auto-merge disabled.

Interactive flow:
  fleet launch squadron

Headless (hangar output):
  fleet launch squadron --data '<json>'`,
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := fleet.Load(".")
		if err != nil {
			return fmt.Errorf("failed to load fleet: %w", err)
		}

		dataJSON, _ := cmd.Flags().GetString("data")
		useJumpSh, _ := cmd.Flags().GetBool("use-jump-sh")

		if dataJSON != "" {
			data, errs := squadron.ParseAndValidate([]byte(dataJSON))
			if len(errs) > 0 {
				for _, e := range errs {
					fmt.Fprintln(os.Stderr, "error:", e)
				}
				return fmt.Errorf("squadron --data validation failed (%d error(s))", len(errs))
			}
			return squadron.RunHeadless(f, data)
		}

		return tui.RunSquadronLaunch(f, useJumpSh)
	},
}

func init() {
	squadronCmd.Flags().String("data", "", "JSON payload describing the full squadron (skips TUI)")
	launchCmd.AddCommand(squadronCmd)
}
```

Add imports at the top of `cmd_launch.go`:
- `"os"`
- `"github.com/MrBenJ/fleet-commander/internal/squadron"`

- [ ] **Step 3: Build**

Run: `go build ./...`
Expected: FAIL — `squadron.RunHeadless` does not exist yet. Temporarily stub it to get the build green before the next task:

Create `internal/squadron/headless.go` with:

```go
package squadron

import "github.com/MrBenJ/fleet-commander/internal/fleet"

// RunHeadless launches a squadron without the TUI. See the spec's
// "Behavioral rules for --data mode" section. Implemented in a follow-up task.
func RunHeadless(f *fleet.Fleet, data *SquadronData) error {
	return nil
}
```

Re-run `go build ./...` — should pass.

- [ ] **Step 4: Sanity test the CLI**

Run: `go run ./cmd/fleet/ launch squadron --help`
Expected: help text including `--data` flag and the long description. No errors.

- [ ] **Step 5: Commit**

```bash
git add cmd/fleet/cmd_launch.go internal/squadron/headless.go
git commit -m "feat(cli): add 'fleet launch squadron' subcommand with --data flag"
```

---

## Task 17: `RunHeadless` implementation

**Files:**
- Modify: `internal/squadron/headless.go`
- Create: `internal/squadron/headless_test.go`

- [ ] **Step 1: Write the failing integration test**

```go
// internal/squadron/headless_test.go
package squadron_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/squadron"
)

func TestRunHeadless_WritesPromptsWithSuffixes(t *testing.T) {
	// Skip if git is unavailable (worktree creation needs it)
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	// initialise a minimal git repo so fleet.Init can run
	runGit(t, dir, "init")
	runGit(t, dir, "commit", "--allow-empty", "-m", "init")

	f, err := fleet.Init(dir)
	if err != nil {
		t.Fatalf("fleet.Init: %v", err)
	}

	data := &squadron.SquadronData{
		Name:       "alpha",
		Consensus:  "universal",
		BaseBranch: "main",
		AutoMerge:  true,
		Agents: []squadron.SquadronAgent{
			{Name: "aaa", Branch: "squadron/alpha/aaa", Prompt: "do aaa"},
			{Name: "bbb", Branch: "squadron/alpha/bbb", Prompt: "do bbb"},
		},
	}

	// Run headless — we only need it to get far enough to write the prompt files
	// on disk. If RunHeadless actually calls tmux, it may error out in CI; the
	// test asserts that prompt files were written before that point.
	_ = squadron.RunHeadless(f, data)

	// Both prompt files should exist with suffixes appended
	for _, name := range []string{"aaa", "bbb"} {
		path := filepath.Join(f.FleetDir, "prompts", name+".txt")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("prompt file %s not written: %v", path, err)
			continue
		}
		content := string(b)
		if !strings.Contains(content, "Squadron Consensus Protocol (UNIVERSAL)") {
			t.Errorf("prompt %s missing consensus suffix", name)
		}
		if !strings.Contains(content, "do "+name) {
			t.Errorf("prompt %s missing original text", name)
		}
	}
}

// runGit is a tiny helper; add at the top of the file with exec imported.
```

Add the helper at the top:

```go
import (
	"os/exec"
	"testing"
)

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/squadron/...`
Expected: FAIL — stub returns nil, writes nothing.

- [ ] **Step 3: Implement `RunHeadless`**

Replace `internal/squadron/headless.go`:

```go
package squadron

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	fleetctx "github.com/MrBenJ/fleet-commander/internal/context"
	"github.com/MrBenJ/fleet-commander/internal/driver"
	"github.com/MrBenJ/fleet-commander/internal/fleet"
	"github.com/MrBenJ/fleet-commander/internal/tmux"
)

// RunHeadless launches a squadron from a validated SquadronData payload,
// bypassing the TUI. Worktrees are created, prompts (with consensus + merger
// + persona suffixes) are written to disk, and tmux sessions are started.
func RunHeadless(f *fleet.Fleet, data *SquadronData) error {
	if len(data.Agents) == 0 {
		return fmt.Errorf("no agents in squadron")
	}

	// Resolve base branch default
	baseBranch := data.BaseBranch
	if baseBranch == "" {
		cb, err := f.CurrentBranch()
		if err != nil {
			return fmt.Errorf("could not resolve base branch: %w", err)
		}
		baseBranch = cb
	}

	// Resolve merge master (random if nil)
	mergeMaster := ""
	if data.AutoMerge {
		if data.MergeMaster != nil && *data.MergeMaster != "" {
			mergeMaster = *data.MergeMaster
		} else {
			mergeMaster = data.Agents[rand.Intn(len(data.Agents))].Name
		}
	}

	// Agent name list + branch list for suffix rendering
	agentNames := make([]string, 0, len(data.Agents))
	agentBranches := make([]AgentBranch, 0, len(data.Agents))
	for _, a := range data.Agents {
		agentNames = append(agentNames, a.Name)
		agentBranches = append(agentBranches, AgentBranch{Name: a.Name, Branch: a.Branch})
	}

	// Create squadron channel
	channelName := "squadron-" + data.Name
	description := fmt.Sprintf("Squadron %s (%s)", data.Name, data.Consensus)
	if _, err := fleetctx.CreateChannel(f.FleetDir, channelName, description, agentNames); err != nil {
		// Not fatal — channel may already exist
		fmt.Fprintf(os.Stderr, "note: squadron channel create: %v\n", err)
	}

	// Load system prompt once
	sysPrompt, _ := fleet.LoadSystemPrompt(f.FleetDir)
	if data.UseJumpSh {
		sysPrompt += "\n\nYour workbranch will be able to be accessed via a local dev instance via a tool called 'https://jump.sh/' - Jump SH. Fetch this web URL to see what it does and how it works. Upon initialization, use this locally hosted web server as a way to access a local development environment for yourself"
	}

	promptsDir := filepath.Join(f.FleetDir, "prompts")
	if err := os.MkdirAll(promptsDir, 0755); err != nil {
		return fmt.Errorf("create prompts dir: %w", err)
	}

	tm := tmux.NewManager(f.TmuxPrefix())
	statesDir := filepath.Join(f.FleetDir, "states")
	_ = os.MkdirAll(statesDir, 0755)

	var launched []string

	for _, a := range data.Agents {
		agent, err := f.AddAgent(a.Name, a.Branch)
		if err != nil {
			if !strings.Contains(err.Error(), "already exists") {
				return fmt.Errorf("add agent %q: %w", a.Name, err)
			}
			agent, err = f.GetAgent(a.Name)
			if err != nil {
				return fmt.Errorf("get agent %q: %w", a.Name, err)
			}
		}

		// Assemble prompt: system + roster + task + suffixes + persona
		fullPrompt := buildHeadlessPrompt(sysPrompt, data.Agents, a)

		// Consensus
		switch data.Consensus {
		case "universal":
			fullPrompt += "\n" + BuildConsensusSuffix("universal", data.Name, agentNames, "", baseBranch)
		case "review_master":
			if a.Name == data.ReviewMaster {
				fullPrompt += "\n" + BuildReviewMasterReviewerSuffix(data.Name, agentNames, baseBranch)
			} else {
				fullPrompt += "\n" + BuildConsensusSuffix("review_master", data.Name, agentNames, data.ReviewMaster, baseBranch)
			}
		}

		// Merger
		if data.AutoMerge && a.Name == mergeMaster {
			fullPrompt += "\n" + BuildMergerSuffix(data.Name, baseBranch, agentBranches)
		}

		// Persona preamble
		if a.Persona != "" {
			if p, ok := LookupPersona(a.Persona); ok {
				fullPrompt = ApplyPersona(p, fullPrompt)
			}
		}

		// Write prompt file
		promptFile := filepath.Join(promptsDir, agent.Name+".txt")
		if err := os.WriteFile(promptFile, []byte(fullPrompt), 0644); err != nil {
			return fmt.Errorf("write prompt file %s: %w", promptFile, err)
		}

		// Driver + hooks
		drv, err := driver.GetForAgent(agent)
		if err != nil {
			drv, _ = driver.Get(a.Driver)
		}
		stateFilePath := filepath.Join(statesDir, agent.Name+".json")
		if err := drv.InjectHooks(agent.WorktreePath); err != nil {
			fmt.Fprintf(os.Stderr, "warning: inject hooks for %q: %v\n", agent.Name, err)
			stateFilePath = ""
			f.UpdateAgentHooks(agent.Name, false)
		} else {
			f.UpdateAgentHooks(agent.Name, true)
		}

		// Launcher script
		launcherFile := filepath.Join(promptsDir, agent.Name+".sh")
		launcherScript := drv.BuildCommand(driver.LaunchOpts{
			YoloMode:   true,
			PromptFile: promptFile,
			AgentName:  agent.Name,
		})
		if err := os.WriteFile(launcherFile, []byte(launcherScript), 0755); err != nil {
			return fmt.Errorf("write launcher %s: %w", launcherFile, err)
		}

		// tmux session
		if err := tm.CreateSession(agent.Name, agent.WorktreePath, []string{launcherFile}, stateFilePath); err != nil {
			return fmt.Errorf("create tmux session %q: %w", agent.Name, err)
		}
		f.UpdateAgentStateFile(agent.Name, stateFilePath)
		pid, _ := tm.GetPID(agent.Name)
		f.UpdateAgent(agent.Name, "running", pid)
		if a.Driver != "" && a.Driver != "claude-code" {
			f.UpdateAgentDriver(agent.Name, a.Driver)
		}
		launched = append(launched, agent.Name)
	}

	fmt.Fprintf(os.Stderr, "Squadron %q launched: %s\n", data.Name, strings.Join(launched, ", "))
	return nil
}

// buildHeadlessPrompt assembles system prompt + agent roster + task, mirroring
// the tui.buildFullPrompt helper but using SquadronAgent instead of LaunchItem.
func buildHeadlessPrompt(systemPrompt string, all []SquadronAgent, current SquadronAgent) string {
	var b strings.Builder
	if strings.TrimSpace(systemPrompt) != "" {
		b.WriteString(systemPrompt)
		b.WriteString("\n\n")
	}
	b.WriteString("## Active Fleet Agents\n\n")
	b.WriteString(fmt.Sprintf("You are: %s (branch: %s)\n\n", current.Name, current.Branch))
	b.WriteString("| Agent | Branch | Task |\n")
	b.WriteString("|-------|--------|------|\n")
	for _, a := range all {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", a.Name, a.Branch, a.Prompt))
	}
	b.WriteString("\n---\n\n")
	b.WriteString(current.Prompt)
	return b.String()
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/squadron/...`
Expected: PASS (integration test writes prompt files with suffixes; tmux session creation may fail silently in CI but the test only asserts on prompt file contents).

If the test fails because `RunHeadless` errors out before writing prompts (e.g. tmux not available), adjust the test to construct a partial helper — but *do not* hide the error. If tmux creation must be skipped, inject a `noop tmux` via a small seam or guard the test with `t.Skip` when tmux isn't on PATH.

- [ ] **Step 5: Commit**

```bash
git add internal/squadron/headless.go internal/squadron/headless_test.go
git commit -m "feat(squadron): implement RunHeadless --data launch path"
```

---

## Task 18: Final integration — end-to-end smoke test and docs

**Files:**
- Modify: `CLAUDE.md` (document the squadron feature briefly)
- Run: full test suite and manual smoke test

- [ ] **Step 1: Run the full test suite**

Run: `go test ./...`
Expected: PASS across all packages. Fix any regressions before proceeding.

- [ ] **Step 2: Run `go vet` and `make vet` if present**

Run: `go vet ./...` and `make vet 2>/dev/null || true`
Expected: no warnings.

- [ ] **Step 3: Manual smoke test of interactive flow**

In a scratch test repo:

```bash
cd /tmp && rm -rf squadron-smoke && mkdir squadron-smoke && cd squadron-smoke
git init && git commit --allow-empty -m init
go run /Users/bjunya/code/fleet-commander/cmd/fleet/ init
go run /Users/bjunya/code/fleet-commander/cmd/fleet/ launch squadron --help
```

Expected: help text renders with `--data` flag. (Full interactive smoke test requires a TTY; run it manually outside the plan steps.)

- [ ] **Step 4: Manual smoke test of `--data` flow with an intentionally-bad payload**

```bash
go run /Users/bjunya/code/fleet-commander/cmd/fleet/ launch squadron --data '{"name":"!!","consensus":"bogus","agents":[]}'
```

Expected: multiple errors printed to stderr, exit nonzero, no agents launched.

- [ ] **Step 5: Update CLAUDE.md with a short squadron reference**

Add a new subsection under "Key flow" in `CLAUDE.md`:

```markdown
**Squadron mode — `fleet launch squadron`:**
1. Interactive: consensus selector → squadron name → standard launch flow. Always runs yolo + per-agent auto-merge OFF.
2. Headless: `fleet launch squadron --data '<json>'` parses a SquadronData payload and skips the TUI entirely.
3. A fleet context channel `squadron-<name>` is auto-created with all agents as members.
4. Each agent's prompt is assembled with a consensus suffix (+ merger suffix for the designated merger + persona preamble). See `internal/squadron/`.
```

- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: add squadron mode overview to CLAUDE.md"
```

---

## Self-Review Notes

Cross-checked against spec (`docs/superpowers/specs/2026-04-07-squadron-design.md`):

- **CLI:** `fleet launch squadron` subcommand + `--data` flag → Task 16 ✓
- **JSON schema fields:** name, consensus, reviewMaster, baseBranch, autoMerge, mergeMaster, useJumpSh, agents[] → Task 7 ✓
- **Strict parsing (DisallowUnknownFields):** Task 7 ✓
- **All-or-nothing validation (multiple errors reported):** Task 8 ✓
- **Validation rules** (name format, consensus values, reviewMaster requires/forbids, mergeMaster must match agent, duplicate agent names, unknown persona, empty agents): Task 8 ✓
- **TUI screens:** consensus selector + name input → Tasks 11, 12 ✓
- **LaunchModel fields:** squadronMode, squadronName, consensusType, reviewMaster, mergeMaster, autoMerge, baseBranch, squadronChannelCreated, personas → Task 9 ✓
- **launchMode constants:** launchModeSquadronConsensus, launchModeSquadronName → Task 9 ✓
- **Consensus suffix templates** (universal, review_master reviewer, review_master non-reviewer, none → empty): Tasks 2, 3 ✓
- **Merger duties suffix with all agents listed in order** including merger's own: Task 4 ✓
- **Persona apply prepends above system prompt:** Tasks 5, 13 ✓
- **5 built-in personas** with full preambles from spec: Task 6 ✓
- **Channel auto-creation on first launch** with members=all agent names: Task 14 ✓
- **Random review_master / merge_master selection in interactive mode:** Task 14 ✓
- **Headless `RunHeadless`:** Task 17 ✓
- **Squadron mode forces yolo + per-agent no-auto-merge + skipYoloConfirm:** Task 10 ✓

**Placeholder scan:** No "TBD"/"implement later" language. The one non-literal-code instruction is in Task 6 for the persona preambles — the preamble constants are included verbatim in the plan itself (not behind a reference); the spec pointer is a secondary verification step.

**Type consistency:** `AgentBranch`, `Persona`, `SquadronData`, `SquadronAgent`, `BuildConsensusSuffix`, `BuildReviewMasterReviewerSuffix`, `BuildMergerSuffix`, `LookupPersona`, `ApplyPersona`, `ParseAndValidate`, `RunHeadless`, `applySquadronSuffixes`, `newSquadronLaunchModel`, `RunSquadronLaunch` — names and signatures are consistent across every task that references them.

**Scope check:** Single-spec feature, squadron package is a clean new boundary, TUI changes are additive. Appropriately scoped for one plan.
