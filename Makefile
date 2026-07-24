SHELL := bash
.SHELLFLAGS := -eu -o pipefail -c
.DEFAULT_GOAL := help

BIN := $(CURDIR)/bin

.PHONY: help
help: ## List the available targets.
	@grep -hE '^[A-Za-z0-9_./-]+:[^=]*?## ' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":[^=]*?## "} {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Compile every package.
	go build ./...

.PHONY: test
test: ## Run the test suites.
	go test -race ./...

.PHONY: tidy
tidy: ## Sync go.mod and go.sum.
	go mod tidy

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter.
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes.
	$(GOLANGCI_LINT) run --fix

.PHONY: fmt
fmt: golangci-lint ## Apply the configured formatters.
	$(GOLANGCI_LINT) fmt

.PHONY: verify
verify: build test lint ## Run everything CI runs.
	$(GOLANGCI_LINT) run

.PHONY: clean
clean: ## Remove built binaries.
	rm -rf $(BIN)

##@ Tools

GOLANGCI_LINT = $(BIN)/custom-gcl
GOLANGCI_LINT_VERSION ?= $(shell awk -F"[ \"']+" '/^version:/ { print $$2; exit }' .custom-gcl.yml)

PLUGIN_SOURCES := $(shell find golangci -name '*.go' \
	-not -name '*_test.go' -not -path '*/testdata/*' 2>/dev/null)

.PHONY: golangci-lint
golangci-lint: $(GOLANGCI_LINT) ## Build golangci-lint with the code-hygiene linters.

$(GOLANGCI_LINT): .custom-gcl.yml go.mod go.sum $(PLUGIN_SOURCES)
	@echo "Building golangci-lint with the code-hygiene linters..."
	@mkdir -p $(BIN)
	GOBIN=$(BIN) go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	$(BIN)/golangci-lint custom
	$@ cache clean
