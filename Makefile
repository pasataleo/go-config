.PHONY: all test build lint lint_install tidy generate fmt install-hooks

GOLANGCI_LINT_VERSION := v$(shell jq -r '.["golangci-lint"].version' .tool-versions.json)
GIT_HOOKS_DIR := $(shell git rev-parse --git-path hooks)

all: install-hooks tidy generate fmt lint test build

install-hooks:
	@mkdir -p $(GIT_HOOKS_DIR)
	@cp .githooks/pre-commit $(GIT_HOOKS_DIR)/pre-commit && chmod +x $(GIT_HOOKS_DIR)/pre-commit

test:
	go test ./...

build:
	go build ./...

tidy:
	go mod tidy

generate:
	go generate ./...

fmt:
	go fmt ./...

lint_install:
	@golangci-lint version 2>/dev/null | grep -Eq 'version v?$(patsubst v%,%,$(GOLANGCI_LINT_VERSION)) ' || \
		go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)

lint: lint_install
	golangci-lint run
