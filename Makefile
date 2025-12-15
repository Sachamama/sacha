APP := sacha

.PHONY: build test run lint
SHELL := /bin/sh

build:
	@echo "Building $(APP)..."
	@go build -o bin/$(APP) ./cmd/$(APP)

test:
	@go test ./...

run:
	@go run ./cmd/$(APP)

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed; skipping lint"
