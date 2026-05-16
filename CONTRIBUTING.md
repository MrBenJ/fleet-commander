# Contributing to Fleet Commander

Thanks for your interest. This document covers how to set up a dev
environment, the contribution workflow, and the conventions the codebase
follows.

## Development setup

Prerequisites: Go 1.21+, Node 22+, tmux, git.

```bash
# Clone
git clone https://github.com/MrBenJ/fleet-commander.git
cd fleet-commander

# Install Go deps
go mod download

# Install frontend deps
cd web && npm ci && cd ..

# Build everything
make build-all
```

## Running tests

```bash
go test ./...                  # all Go tests
go test -race ./...            # with race detector (what CI runs)
cd web && npm test             # frontend Vitest suite
```

When you change Go code, run `go vet ./...` before pushing. When you change
frontend code, run `npm run build` to catch type errors.

## Code conventions

- **Go**: standard `gofmt` / `goimports`. Table-driven tests where possible.
  Test files live alongside the code as `*_test.go`.
- **TypeScript**: keep `tsconfig.json` strict. Components use Tailwind
  utility classes already present in `web/src/components/`.
- **Commits**: imperative mood, short subject (≤72 chars), longer body if the
  *why* is non-obvious.
- **Branches**: feature branches off `main`. Avoid long-lived forks.

## Pull requests

1. Open the PR against `main`.
2. Make sure CI is green — `go vet`, `go test -race`, and the frontend suite
   must all pass.
3. Describe the *why* of the change. The diff shows the *what*.
4. Link related issues with `Fixes #N` / `Refs #N`.
5. Small, focused PRs are preferred over large omnibus ones.

If your change touches the hangar UI, include a screenshot or short clip.

## Squadron-style development

Fleet Commander uses itself for non-trivial refactors — see the `squadron`
launch flow in `cmd/fleet/launch.go`. If you're shipping a multi-faceted
change, consider running `fleet hangar` to coordinate parallel work, then
combine into one PR.

## Reporting security issues

See [SECURITY.md](./SECURITY.md). Please do not file public issues for
security problems.
