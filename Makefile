VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build:
	go install -ldflags "$(LDFLAGS)" ./cmd/fleet/

build-web:
	cd web && npm run build

# Copy dist into cmd/fleet/ for go:embed, then install to PATH
build-all: build-web
	rm -rf cmd/fleet/webdist
	cp -r web/dist cmd/fleet/webdist
	go install -ldflags "$(LDFLAGS)" ./cmd/fleet/
	rm -rf cmd/fleet/webdist
	mkdir -p cmd/fleet/webdist && touch cmd/fleet/webdist/.gitkeep

test:
	go test ./...

vet:
	go vet ./...
