SHELL := /bin/bash
GO ?= go
PKG := ./...
BINARY := cdvd
CMD_DIR := ./cmd/cdvd
COVER_PROFILE := coverage.out

.PHONY: help
help:
	@echo "Common targets:"
	@echo "  make fmt           - format code (gofmt/gofumpt via golangci-lint fix)"
	@echo "  make fmt-check     - check formatting only"
	@echo "  make lint          - run static analysis"
	@echo "  make test          - run unit tests with race and coverage"
	@echo "  make cover         - open coverage report"
	@echo "  make build         - build $(BINARY) from $(CMD_DIR)"
	@echo "  make api-validate  - optional API naming check against upstream"

.PHONY: fmt
fmt:
	@$(GO) fmt $(PKG)
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run --fix || true

.PHONY: fmt-check
fmt-check:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run || { echo "golangci-lint not installed"; exit 1; }

.PHONY: lint
lint:
	@golangci-lint run

.PHONY: test
test:
	@$(GO) test -race -covermode=atomic -coverprofile=$(COVER_PROFILE) $(PKG)

.PHONY: cover
cover: test
	@$(GO) tool cover -func=$(COVER_PROFILE) | tail -n 1
	@$(GO) tool cover -html=$(COVER_PROFILE) -o coverage.html
	@echo "Coverage report: coverage.html"

.PHONY: build
build:
	@$(GO) build -o bin/$(BINARY) $(CMD_DIR)
	@echo "Built bin/$(BINARY)"

.PHONY: api-validate
api-validate:
	@echo "API validation stub: ensure naming aligns with registryaccord-specs schemas/INDEX.md"
	@echo "PASS (stub)"
