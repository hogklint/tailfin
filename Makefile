SHELL:=/usr/bin/env bash

.PHONY: build
build:
	go build -o dist/tailfin ./cmd/tailfin

TOOLS_BIN_DIR := $(CURDIR)/hack/tools/bin
GORELEASER_VERSION ?= v2.3.2
GORELEASER := $(TOOLS_BIN_DIR)/goreleaser
GOLANGCI_LINT_VERSION ?= v1.63.1
GOLANGCI_LINT := $(TOOLS_BIN_DIR)/golangci-lint
GORELEASER_FILTER_VERSION ?= v0.3.0
GORELEASER_FILTER := $(TOOLS_BIN_DIR)/goreleaser-filter

$(GORELEASER):
	GOBIN=$(TOOLS_BIN_DIR) go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

$(GOLANGCI_LINT):
	GOBIN=$(TOOLS_BIN_DIR) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

$(GORELEASER_FILTER):
	GOBIN=$(TOOLS_BIN_DIR) go install github.com/t0yv0/goreleaser-filter@$(GORELEASER_FILTER_VERSION)

.PHONY: build-cross
build-cross: $(GORELEASER)
	$(GORELEASER) build --snapshot --clean

.PHONY: test
test: lint
	go test -v ./...

.PHONY: lint
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run --fix

README_FILE ?= ./README.md

.PHONY: update-readme
update-readme:
	go run hack/update-readme/update-readme.go $(README_FILE)

.PHONY: verify-readme
verify-readme:
	./hack/verify-readme.sh

.PHONY: dist
dist: $(GORELEASER) $(GORELEASER_FILTER)
	cat .goreleaser.yaml | $(GORELEASER_FILTER) -goos $(shell go env GOOS) -goarch $(shell go env GOARCH) | $(GORELEASER) release -f- --clean --skip=publish --snapshot

.PHONY: dist-all
dist-all: $(GORELEASER)
	$(GORELEASER) release --clean --skip=publish --snapshot

.PHONY: release
release: $(GORELEASER)
	$(GORELEASER) release --clean --skip=validate

.PHONY: clean
clean: clean-tools clean-dist

.PHONY: clean-tools
clean-tools:
	rm -rf $(TOOLS_BIN_DIR)

.PHONY: clean-dist
clean-dist:
	rm -rf ./dist
