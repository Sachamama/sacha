APP := sacha
MODULE := github.com/sachamama/sacha

# Version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Build flags
LDFLAGS := -s -w \
	-X $(MODULE)/internal/version.Version=$(VERSION) \
	-X $(MODULE)/internal/version.Commit=$(COMMIT) \
	-X $(MODULE)/internal/version.Date=$(DATE)

.PHONY: build test run lint clean install snapshot release-dry-run setup fmt

build:
	@echo "Building $(APP) $(VERSION)..."
	@go build -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/$(APP)

install:
	@echo "Installing $(APP) $(VERSION)..."
	@go install -ldflags "$(LDFLAGS)" ./cmd/$(APP)

test:
	@go test -race ./...

lint:
	@command -v golangci-lint >/dev/null 2>&1 && golangci-lint run ./... || echo "golangci-lint not installed; skipping lint"

clean:
	@rm -rf bin/ dist/

run:
	@go run -ldflags "$(LDFLAGS)" ./cmd/$(APP)

# GoReleaser targets (requires goreleaser installed)
snapshot:
	@goreleaser release --snapshot --clean

release-dry-run:
	@goreleaser release --skip=publish --clean

# Development setup
setup:
	@echo "Installing git hooks..."
	@git config core.hooksPath .githooks
	@echo "Installing development tools..."
	@go install mvdan.cc/gofumpt@latest
	@echo "Setup complete! Git hooks are now active."

fmt:
	@echo "Formatting code..."
	@$(shell go env GOPATH)/bin/gofumpt -w .
