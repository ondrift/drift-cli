# =============================================================================
# drift-cli — User-facing CLI
# =============================================================================

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# ---------------------------------------------------------------------------
# Formatting
# ---------------------------------------------------------------------------

.PHONY: fmt fmt-check

fmt:
	gofmt -s -w . && gofumpt -w .

fmt-check:
	@test -z "$$(gofmt -s -l .)"  || (echo "gofmt issues"   && exit 1)
	@test -z "$$(gofumpt -l .)"   || (echo "gofumpt issues" && exit 1)

# ---------------------------------------------------------------------------
# Modules
# ---------------------------------------------------------------------------

.PHONY: mod

mod:
	go mod tidy && go mod verify

# ---------------------------------------------------------------------------
# Quality
# ---------------------------------------------------------------------------

.PHONY: vet lint quality

vet:
	go vet ./...

lint:
	staticcheck ./...

quality: vet lint

# ---------------------------------------------------------------------------
# Testing
# ---------------------------------------------------------------------------

.PHONY: test race test-all

test:
	go test ./...

race:
	go test -race ./...

test-all: test race

# ---------------------------------------------------------------------------
# Security
# ---------------------------------------------------------------------------

.PHONY: vuln gosec scan

vuln:
	govulncheck ./...

gosec:
	gosec -severity=low -confidence=low ./...

scan: vuln gosec

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------

SDK_SRC   := ../drift-sdk
SDK_DEST  := cmd/slice/atomic/cmd/deploy/sdk

.PHONY: sync-sdk build install

sync-sdk:
	@mkdir -p $(SDK_DEST)
	@cp $(SDK_SRC)/*.go $(SDK_DEST)/
	@cp $(SDK_SRC)/go.mod $(SDK_DEST)/go.mod.txt

build: sync-sdk
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o drift .

install: build
	install -m 0755 drift /usr/local/bin/drift

# ---------------------------------------------------------------------------
# Release (requires goreleaser)
# ---------------------------------------------------------------------------

.PHONY: release snapshot

release:
	goreleaser release --clean

snapshot:
	goreleaser release --snapshot --clean

# ---------------------------------------------------------------------------
# High-level targets
# ---------------------------------------------------------------------------

.PHONY: dev ci

dev: fmt mod quality test

ci: fmt-check mod quality test-all scan build
