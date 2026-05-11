.PHONY: all test build lint lint_install tidy generate fmt

all: tidy generate fmt lint test build

test:
	go test ./...

build:
	go build -o bin/go-template main.go

tidy:
	go mod tidy

generate:
	go generate ./...

fmt:
	go fmt ./...

lint_install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

lint: lint_install
	golangci-lint run