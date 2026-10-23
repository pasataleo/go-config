.PHONY: all test build lint lint_install tidy generate fmt

GOLANGCI_LINT_VERSION := v2.15.0

all: tidy generate fmt lint test build

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
