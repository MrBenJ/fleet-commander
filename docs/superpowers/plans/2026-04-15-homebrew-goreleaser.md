# Homebrew Tap + GoReleaser Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish fleet-commander to Homebrew via a custom tap (`homebrew-fleet-commander`), automated with GoReleaser and a GitHub Actions release workflow.

**Architecture:** GoReleaser handles cross-compilation (macOS arm64/amd64), tarball packaging, GitHub Release creation, and Homebrew formula generation. A new GitHub Actions workflow triggers on version tags (`v*`), builds the web frontend first, then runs GoReleaser. The Homebrew tap repo (`MrBenJ/homebrew-fleet-commander`) hosts the auto-updated formula.

**Tech Stack:** GoReleaser v2, GitHub Actions, Homebrew, Go embed

---

## File Structure

| Action | File | Purpose |
|--------|------|---------|
| Create | `.goreleaser.yml` | GoReleaser config: builds, archives, homebrew tap |
| Create | `.github/workflows/release.yml` | GitHub Actions workflow triggered by version tags |
| Modify | `Makefile` | Add `release-tag` convenience target |
| Create | *(separate repo)* `MrBenJ/homebrew-fleet-commander/Formula/fleet.rb` | Homebrew formula (auto-managed by GoReleaser) |

---

### Task 1: Create the Homebrew Tap Repository

This task is done on GitHub, not in the fleet-commander codebase.

**Files:**
- Create: `MrBenJ/homebrew-fleet-commander` (new GitHub repo)
- Create: `MrBenJ/homebrew-fleet-commander/README.md`

- [ ] **Step 1: Create the tap repo on GitHub**

```bash
gh repo create MrBenJ/homebrew-fleet-commander --public --description "Homebrew tap for Fleet Commander" --clone=false
```

- [ ] **Step 2: Initialize the repo with a README**

Clone and add a minimal README:

```bash
cd /tmp
git clone git@github.com:MrBenJ/homebrew-fleet-commander.git
cd homebrew-fleet-commander
mkdir -p Formula
```

Create `README.md`:

```markdown
# homebrew-fleet-commander

Homebrew tap for [Fleet Commander](https://github.com/MrBenJ/fleet-commander).

## Install

```bash
brew tap MrBenJ/fleet-commander
brew install fleet
```

## Update

```bash
brew upgrade fleet
```
```

- [ ] **Step 3: Push the initial commit**

```bash
git add .
git commit -m "Initial tap repo setup"
git push origin main
```

- [ ] **Step 4: Create a GitHub Personal Access Token**

GoReleaser needs a PAT with `repo` scope to push formula updates to the tap repo. 

1. Go to GitHub Settings > Developer settings > Personal access tokens > Tokens (classic)
2. Create a token with `repo` scope
3. Add it as a repository secret named `HOMEBREW_TAP_TOKEN` in `MrBenJ/fleet-commander`:

```bash
gh secret set HOMEBREW_TAP_TOKEN --repo MrBenJ/fleet-commander
```

(Paste the token when prompted.)

---

### Task 2: Create the GoReleaser Configuration

**Files:**
- Create: `.goreleaser.yml`

- [ ] **Step 1: Create `.goreleaser.yml`**

```yaml
# yaml-language-server: $schema=https://goreleaser.com/static/schema.json
version: 2

before:
  hooks:
    - go mod tidy

builds:
  - id: fleet
    main: ./cmd/fleet/
    binary: fleet
    ldflags:
      - -s -w
      - -X main.version={{.Version}}
      - -X main.commit={{.ShortCommit}}
    goos:
      - darwin
      - linux
    goarch:
      - amd64
      - arm64
    env:
      - CGO_ENABLED=0

archives:
  - id: fleet-archive
    builds:
      - fleet
    format: tar.gz
    name_template: "fleet_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
    files:
      - fleet.tmux.conf

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"

brews:
  - name: fleet
    ids:
      - fleet-archive
    repository:
      owner: MrBenJ
      name: homebrew-fleet-commander
      token: "{{ .Env.HOMEBREW_TAP_TOKEN }}"
    commit_author:
      name: goreleaserbot
      email: bot@goreleaser.com
    directory: Formula
    homepage: "https://github.com/MrBenJ/fleet-commander"
    description: "CLI for managing parallel Claude Code sessions in git worktrees"
    license: "MIT"
    dependencies:
      - name: tmux
    install: |
      bin.install "fleet"
      # Install tmux config alongside the binary for auto-sourcing
      (share/"fleet").install "fleet.tmux.conf"
    test: |
      assert_match version.to_s, shell_output("#{bin}/fleet --version")
```

- [ ] **Step 2: Verify the config is valid (dry run)**

Install GoReleaser if not present, then validate:

```bash
go install github.com/goreleaser/goreleaser/v2@latest
goreleaser check
```

Expected: `config is valid`

- [ ] **Step 3: Commit**

```bash
git add .goreleaser.yml
git commit -m "feat: add GoReleaser config for Homebrew distribution"
```

---

### Task 3: Create the GitHub Actions Release Workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1: Create `.github/workflows/release.yml`**

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

permissions:
  contents: write

jobs:
  release:
    name: Release
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
          cache: true

      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: npm
          cache-dependency-path: web/package-lock.json

      - name: Build frontend
        run: |
          cd web && npm ci && npm run build
          cd ..
          rm -rf cmd/fleet/webdist
          cp -r web/dist cmd/fleet/webdist

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_TOKEN: ${{ secrets.HOMEBREW_TAP_TOKEN }}

      - name: Clean up webdist
        if: always()
        run: |
          rm -rf cmd/fleet/webdist
          mkdir -p cmd/fleet/webdist && touch cmd/fleet/webdist/.gitkeep
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat: add GitHub Actions release workflow for GoReleaser"
```

---

### Task 4: Add Makefile Convenience Target

**Files:**
- Modify: `Makefile`

- [ ] **Step 1: Add `release-tag` target to Makefile**

Append after the existing `release` target:

```makefile
# Push a release tag to trigger GoReleaser via GitHub Actions.
# Usage:
#   make release-tag            # patch bump, push tag
#   make release-tag BUMP=minor # minor bump, push tag
release-tag: release
	@TAG=$$(git describe --tags --abbrev=0); \
	echo "Pushing $$TAG to trigger release workflow..."; \
	git push origin "$$TAG"
```

- [ ] **Step 2: Commit**

```bash
git add Makefile
git commit -m "feat: add release-tag Makefile target to push tags for GoReleaser"
```

---

### Task 5: Test the Full Pipeline (Dry Run)

- [ ] **Step 1: Build the web frontend**

```bash
cd web && npm install && npm run build && cd ..
rm -rf cmd/fleet/webdist
cp -r web/dist cmd/fleet/webdist
```

- [ ] **Step 2: Run GoReleaser in snapshot mode**

This does a full build without publishing anything:

```bash
goreleaser release --snapshot --clean
```

Expected: builds complete for `darwin_amd64`, `darwin_arm64`, `linux_amd64`, `linux_arm64`. Archives and checksums appear in `dist/`.

- [ ] **Step 3: Verify the archives contain expected files**

```bash
tar -tzf dist/fleet_*_darwin_arm64.tar.gz
```

Expected output should include:
```
fleet
fleet.tmux.conf
```

- [ ] **Step 4: Clean up**

```bash
rm -rf dist/
rm -rf cmd/fleet/webdist
mkdir -p cmd/fleet/webdist && touch cmd/fleet/webdist/.gitkeep
```

- [ ] **Step 5: Commit all changes (if any fixups were needed)**

```bash
git add -A
git commit -m "fix: goreleaser config adjustments from dry run"
```

(Skip if no changes were needed.)

---

### Task 6: Cut the First Release

- [ ] **Step 1: Tag and push**

```bash
make release-tag BUMP=patch
```

This bumps the version, creates the tag, and pushes it to trigger the release workflow.

- [ ] **Step 2: Monitor the release workflow**

```bash
gh run watch --repo MrBenJ/fleet-commander
```

Expected: workflow completes successfully, GitHub Release is created with binaries, and the formula is pushed to `MrBenJ/homebrew-fleet-commander`.

- [ ] **Step 3: Verify the Homebrew formula was created**

```bash
gh api repos/MrBenJ/homebrew-fleet-commander/contents/Formula/fleet.rb --jq '.name'
```

Expected: `fleet.rb`

- [ ] **Step 4: Test the install**

```bash
brew tap MrBenJ/fleet-commander
brew install fleet
fleet --version
```

Expected: prints the version matching the tag.

---

## Release Cheat Sheet

After this is all set up, the release workflow is:

```bash
# 1. Make sure main is clean and tested
# 2. Tag and push
make release-tag BUMP=patch   # or minor / major

# That's it. GitHub Actions handles:
#   - Building the web frontend
#   - Cross-compiling Go binaries
#   - Creating the GitHub Release with tarballs
#   - Updating the Homebrew formula in the tap repo
```

Users install/upgrade with:

```bash
brew tap MrBenJ/fleet-commander
brew install fleet
brew upgrade fleet
```
