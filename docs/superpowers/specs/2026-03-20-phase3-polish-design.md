# Fleet Commander — Phase 3 Polish Design

**Date:** 2026-03-20
**Status:** Approved
**Scope:** Phase 3 (Polish for public release)
**Depends on:** Phase 1 & 2 hardening (spec: `2026-03-19-phase1-2-hardening-design.md`)

---

## Background

Phases 1 and 2 harden Fleet Commander's internals: test coverage, error handling, config locking, dead code removal. Phase 3 is the public-facing layer that makes the project shippable to people who didn't build it. It covers four items:

1. CI/CD pipeline with full release automation
2. `fleet doctor` diagnostic command
3. User-facing troubleshooting documentation
4. Godoc on all exported symbols with example functions

**Implementation order:** CI first (so every subsequent change is validated), then `fleet doctor`, troubleshooting docs, and godoc last.

---

## P3-1: CI/CD Pipeline

**Problem:** No automated validation exists. Tests, linting, and builds run only when a developer remembers to. There is no release process — users must clone the repo and `go build` themselves.

**Design:**

### GitHub Actions Workflows

Three workflow files in `.github/workflows/`:

**`ci.yml`** — triggers on push and pull_request to `main`.

Jobs:
- **lint:** runs `golangci-lint` against the codebase
- **test:** runs `go test -race -coverprofile=coverage.out ./...`, uploads coverage artifact
- **vet:** runs `go vet ./...`

Go version: `1.25.x` (match `go.mod`). Runs on `ubuntu-latest`.

**`build.yml`** — triggers on push to `main` (after CI passes). Runs `goreleaser build --snapshot --clean` to build binaries for all four platform/arch combos (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64) without publishing. This confirms cross-compilation works on every push to main. Binaries are not released — that's the release workflow's job.

**`release.yml`** — triggers on tags matching `v*` (e.g., `v0.1.0`). Runs goreleaser to:
- Build binaries for all four platform/arch combos
- Create a GitHub Release with auto-generated changelog
- Attach compressed archives + checksums to the release

### goreleaser Configuration

`.goreleaser.yml` at repo root:

```yaml
version: 2
project_name: fleet-commander
builds:
  - env:
      - CGO_ENABLED=0
    main: ./cmd/fleet/
    binary: fleet
    goos: [darwin, linux]
    goarch: [amd64, arm64]
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.ShortCommit}}
      - -X main.date={{.Date}}
archives:
  - format: tar.gz
    name_template: "fleet-commander_{{ .Os }}_{{ .Arch }}"
    files:
      - fleet.tmux.conf
checksum:
  name_template: checksums.txt
changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"
```

### Linter Configuration

`.golangci.yml` at repo root:

```yaml
linters:
  disable-all: true
  enable:
    - gofmt
    - govet
    - gosimple
    - staticcheck
    - errcheck
    - unused
    - ineffassign
```

No style-only linters (e.g., `goimports`, `lll`) that would fire on existing code without value.

### Version Embedding

Add variables to `cmd/fleet/main.go` (populated by goreleaser `ldflags`):

```go
var (
    version = "dev"
    commit  = "unknown"
    date    = "unknown"
)
```

Set `rootCmd.Version` to format these values — Cobra provides both a `--version` flag and a `version` subcommand automatically. When built without ldflags (e.g., `go build`), the values default to the strings above. The `doctor` package accesses version info via a `Version` parameter on `RunAll` (or by reading `debug.ReadBuildInfo()`).

### Coverage Reporting

Coverage is optional for Phase 3 — the CI workflow generates `coverage.out` as an artifact. Integration with codecov.io or similar can be added later without changing the workflow structure.

**Files created:**
- `.github/workflows/ci.yml`
- `.github/workflows/build.yml`
- `.github/workflows/release.yml`
- `.goreleaser.yml`
- `.golangci.yml`

**Files modified:**
- `cmd/fleet/main.go` (version variables + `--version` flag)

---

## P3-2: `fleet doctor` Command

**Problem:** When something is misconfigured (tmux missing, git repo has no commits, worktree directory deleted outside of fleet), users get cryptic errors from the command that happens to hit the problem first. There is no single command that checks the full environment health.

**Design:**

### Command Interface

```
fleet doctor          # advisory: prints all checks, always exits 0
fleet doctor --strict # exits 1 if any critical check fails
```

### Check Tiers

**Critical** (blocks `--strict`):
- `git` binary in PATH
- `tmux` binary in PATH
- Current directory is inside a git repository
- Git repo has at least one commit (required for worktree creation)
- `.fleet/config.json` exists and is valid JSON (skipped with "no fleet initialized" note if `.fleet/` doesn't exist)

**Warning** (printed but doesn't block `--strict`):
- `claude` binary not in PATH
- Orphaned state files in `.fleet/states/` with no matching agent in config (skipped if no fleet)
- Agent worktree directory missing on disk (agent exists in config but worktree path is gone) (skipped if no fleet)
- `fleet.tmux.conf` not found in expected locations (`filepath.Dir(os.Executable())`)

**Info** (always shown):
- Fleet path and number of agents
- `tmux` version (via `tmux -V`)
- `git` version (via `git --version`)
- Fleet Commander version (from embedded build info)

### Output Format

One line per check, prefixed with a status indicator:

```
✓ git found (2.44.0)
✓ tmux found (3.4)
✓ fleet initialized at /Users/you/code/myproject
✓ config.json is valid (3 agents)
✗ claude not in PATH — agent sessions won't auto-start Claude Code
⚠ agent "feature-auth" worktree missing at .fleet/worktrees/feature-auth
  fleet commander v0.1.0 (abc1234, 2026-03-20)
```

`✓` = pass, `✗` = fail (critical), `⚠` = warning.

### Package Structure

New package: `internal/doctor/`

**`internal/doctor/doctor.go`:**

```go
type Level int
const (
    LevelInfo Level = iota
    LevelWarning
    LevelCritical
)

type Result struct {
    Name    string
    Level   Level
    Passed  bool
    Message string // human-readable description
    Fix     string // suggested fix (empty if passed)
}

type Checker func() Result

// RunAll runs all registered checks and returns results in order.
func RunAll(fleetDir string) []Result
```

The `doctorCmd` in `main.go` does NOT follow the normal command pattern of calling `fleet.Load()` and failing on error. Instead, it attempts `fleet.Load(".")` and proceeds either way:

```go
f, _ := fleet.Load(".")  // error is OK — doctor works without a fleet
fleetDir := ""
if f != nil {
    fleetDir = f.FleetDir
}
results := doctor.RunAll(fleetDir)
```

When `fleetDir` is empty, `RunAll` skips all fleet-dependent checks (config parse, orphaned states, agent worktree missing) and reports them as info-level "no fleet initialized" notes. Environment checks (git, tmux, claude) still run. This ensures `fleet doctor` never fails with "no fleet found" — that's exactly the scenario it should diagnose.

Each check is a standalone `Checker` function registered in `RunAll`. This makes adding new checks trivial and each check independently testable.

**Files created:**
- `internal/doctor/doctor.go`
- `internal/doctor/doctor_test.go`

**Files modified:**
- `cmd/fleet/main.go` (register `doctorCmd`)

---

## P3-3: `TROUBLESHOOTING.md`

**Problem:** When something goes wrong, users have nowhere to look except source code. Error messages from the CLI are terse and don't suggest fixes.

**Design:**

### Location

`TROUBLESHOOTING.md` at repo root. Linked from README with:
```
Having issues? See [Troubleshooting](TROUBLESHOOTING.md).
```

### Structure

Organized by **symptom** (what the user sees), not by internal component. Each section follows: **symptom → cause → fix → prevention.**

### Sections

1. **"tmux is not installed"**
   - Cause: tmux not in PATH
   - Fix: `brew install tmux` (macOS), `apt install tmux` (Ubuntu/Debian)
   - Prevention: run `fleet doctor` before starting

2. **"could not inject hooks"**
   - Cause: `.claude/settings.json` in the worktree is malformed or the `.claude/` directory is unwritable
   - Fix: check the JSON syntax of `.claude/settings.json`, fix or delete it
   - What happens if ignored: monitoring degrades to tmux pane scraping (less reliable, no state file)

3. **"no fleet found"**
   - Cause: not inside an initialized fleet repo, or too many directories up
   - Fix: `cd` into the repo and run `fleet init .` if not initialized

4. **"agent worktree missing"**
   - Cause: worktree directory was deleted outside of fleet (e.g., `rm -rf`)
   - Fix: `fleet remove <name>` to clean the config, then `fleet add <name> <branch>` to recreate

5. **"orphaned tmux sessions"**
   - Cause: fleet process crashed before cleaning up
   - Fix: `tmux ls` to see sessions, `tmux kill-session -t fleet-<name>` to clean up
   - Prevention: always use `fleet stop` or `fleet remove` instead of killing tmux directly

6. **"claude not found"**
   - Cause: Claude Code CLI not installed or not in PATH
   - Fix: install Claude Code (link to docs)
   - What happens if ignored: `fleet start` will fail when creating the tmux session

7. **"status stuck on WORKING"**
   - Cause: hooks not injected (check `fleet list` HOOKS column) or state file is stale (>10 min TTL)
   - Fix: `fleet stop <name>` then `fleet start <name>` to reinject hooks. Run `fleet doctor` to check hooks status.

8. **"fleet queue shows wrong status"**
   - Cause: monitor polls every 2 seconds and relies on terminal pattern matching, which can miss transient states
   - Fix: press `r` in the TUI to force refresh. If state file is working (hooks injected), detection should be reliable.

### Tone

Direct and practical. No apologies, no jargon. Assume the reader is a developer who knows their terminal but doesn't know Fleet Commander internals.

**Files created:**
- `TROUBLESHOOTING.md`

**Files modified:**
- `README.md` (add link to troubleshooting)

---

## P3-4: Godoc + Examples

**Problem:** Zero exported symbols have doc comments. `go doc` output is useless. IDE hover shows nothing. Contributors have to read source to understand any function's contract.

**Design:**

### Doc Comments

Every exported type, function, method, and constant across all `internal/` packages gets a doc comment following Go conventions:
- First sentence starts with the symbol name: `"Init initializes a new fleet for the given repository."`
- Non-obvious behavior, error conditions, and edge cases in subsequent sentences where warranted
- 1-3 sentences for most symbols; more only where behavior is genuinely surprising (e.g., `withLock`'s re-read semantics, `detectState`'s priority ordering)

### Package-Level Documentation

Each package gets a `doc.go` file with a package-level comment explaining:
- What the package does
- How it fits into the Fleet Commander architecture
- Key types and entry points

Packages that get `doc.go`:
- `internal/fleet` — data model and config persistence
- `internal/tmux` — tmux session lifecycle management
- `internal/monitor` — agent state detection via pane scraping and state files
- `internal/worktree` — git worktree creation and management
- `internal/hooks` — Claude Code hook injection and removal
- `internal/state` — state file I/O for hook-based signaling
- `internal/tui` — Bubble Tea TUI for the queue interface
- `internal/doctor` — environment health checks (new in P3-2)

### Example Functions

`Example_*` test functions for key entry points. These appear in `go doc` output and serve as executable documentation.

| Package | Examples |
|---|---|
| `internal/fleet` | `ExampleInit`, `ExampleLoad`, `ExampleFleet_AddAgent` |
| `internal/hooks` | `ExampleInject` (uses temp dir) |
| `internal/state` | `ExampleWrite`, `ExampleRead` |
| `internal/doctor` | `ExampleRunAll` |

No examples for packages requiring tmux, git repos, or a terminal (`tmux`, `monitor`, `worktree`, `tui`) — these are impractical in example functions.

### What Gets Documented (Post-Phase 2)

After Phase 2 lands, `internal/queue` and `internal/agent` are deleted. The remaining packages and their exported symbols (including new ones from Phase 2: `UpdateAgentHooks`, `withLock`, etc.) are the full surface to document.

**Files created:**
- `internal/fleet/doc.go`
- `internal/tmux/doc.go`
- `internal/monitor/doc.go`
- `internal/worktree/doc.go`
- `internal/hooks/doc.go`
- `internal/state/doc.go`
- `internal/tui/doc.go`
- `internal/doctor/doc.go`
- `internal/fleet/example_test.go`
- `internal/hooks/example_test.go`
- `internal/state/example_test.go`
- `internal/doctor/example_test.go`

**Files modified:**
- Every `.go` file with exported symbols (adding doc comments)

---

## Implementation Order

1. **P3-1** (CI/CD) — foundational. Every change after this is validated automatically.
2. **P3-2** (`fleet doctor`) — new command, new package. Tests land with CI already running.
3. **P3-3** (Troubleshooting) — docs only. Can reference `fleet doctor` since it exists.
4. **P3-4** (Godoc) — touches every file. Linting from CI catches formatting issues.

Items within each step are independent unless noted.

---

## Files Changed Per Item

| Item | Files Created | Files Modified |
|---|---|---|
| P3-1 | `.github/workflows/ci.yml`, `.github/workflows/build.yml`, `.github/workflows/release.yml`, `.goreleaser.yml`, `.golangci.yml` | `cmd/fleet/main.go` |
| P3-2 | `internal/doctor/doctor.go`, `internal/doctor/doctor_test.go` | `cmd/fleet/main.go` |
| P3-3 | `TROUBLESHOOTING.md` | `README.md` |
| P3-4 | 8 `doc.go` files, 4 `example_test.go` files | All `.go` files with exported symbols |

---

## Success Criteria

- `go test ./...` passes in CI (GitHub Actions green)
- `golangci-lint run` passes with no findings
- `goreleaser check` validates the release config
- `fleet doctor` reports environment health accurately and exits 1 under `--strict` when critical checks fail
- `fleet doctor` tests cover all check tiers (critical, warning, info)
- `TROUBLESHOOTING.md` covers all 8 documented symptoms
- `README.md` links to troubleshooting
- `go doc ./internal/fleet` (and all other packages) produces meaningful output
- All `Example_*` functions pass as tests
- Tagged release produces downloadable binaries on GitHub

---

## Future User Value

These items are out of Phase 3 scope but represent high-impact improvements for future development:

- **`fleet logs <agent>`** — tail tmux pane output without attaching. Quick peek at agent activity.
- **`fleet status`** — richer than `fleet list`: live monitor state, hooks status, last activity time, branch diff summary.
- **`fleet version`** — print version, build date, commit hash. Trivial once goreleaser embeds build info (version flag is added in P3-1, full subcommand can follow).
- **Homebrew tap** — once goreleaser produces binaries, a `homebrew-fleet-commander` tap is ~20 lines of config. Enables `brew install fleet-commander`.
- **Notification hooks** — "ping me when agent X enters WAITING state." Desktop notifications or webhook integration.
- **`fleet status --json`** — machine-readable output for scripting, dashboards, and editor integrations.
- **Shell completions** — Cobra supports generating bash/zsh/fish completions. One command: `fleet completion zsh > _fleet`.
