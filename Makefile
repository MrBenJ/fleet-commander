VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
LDFLAGS := -X main.version=$(VERSION) -X main.commit=$(COMMIT)

build:
	go install -ldflags "$(LDFLAGS)" ./cmd/fleet/

test:
	go test ./...

vet:
	go vet ./...
