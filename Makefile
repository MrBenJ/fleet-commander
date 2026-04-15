VERSION := $(shell cat VERSION 2>/dev/null || git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

# Version bump type: major, minor, or patch (default: patch)
BUMP := patch

build:
	go install -ldflags "$(LDFLAGS)" ./cmd/fleet/

build-web:
	cd web && npm install && npm run build

# Copy dist into cmd/fleet/ for go:embed, then install to PATH
build-all: build-web
	rm -rf cmd/fleet/webdist
	cp -r web/dist cmd/fleet/webdist
	go clean -cache
	go install -ldflags "$(LDFLAGS)" ./cmd/fleet/
	rm -rf cmd/fleet/webdist
	mkdir -p cmd/fleet/webdist && touch cmd/fleet/webdist/.gitkeep

test:
	go test ./...

vet:
	go vet ./...

# Create a release: bump version, commit, and tag.
# Usage:
#   make release            # patch bump (default): v0.1.0 -> v0.1.1
#   make release BUMP=minor # minor bump: v0.1.0 -> v0.2.0
#   make release BUMP=major # major bump: v0.1.0 -> v1.0.0
release:
	@CURRENT=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0"); \
	MAJOR=$$(echo $$CURRENT | sed 's/^v//' | cut -d. -f1); \
	MINOR=$$(echo $$CURRENT | sed 's/^v//' | cut -d. -f2); \
	PATCH=$$(echo $$CURRENT | sed 's/^v//' | cut -d. -f3); \
	case "$(BUMP)" in \
		major) MAJOR=$$((MAJOR + 1)); MINOR=0; PATCH=0 ;; \
		minor) MINOR=$$((MINOR + 1)); PATCH=0 ;; \
		patch) PATCH=$$((PATCH + 1)) ;; \
		*) echo "Error: BUMP must be major, minor, or patch (got '$(BUMP)')"; exit 1 ;; \
	esac; \
	NEW="v$$MAJOR.$$MINOR.$$PATCH"; \
	git tag "$$NEW"; \
	echo "Released $$NEW (was $$CURRENT)"
