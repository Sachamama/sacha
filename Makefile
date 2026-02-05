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

.PHONY: build test run lint clean install snapshot release-dry-run setup fmt \
	local-up local-down local-seed local-run local-reset

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

# LocalStack targets
LOCAL_ENDPOINT := http://localhost:4566
LOCAL_ENV := AWS_ENDPOINT_URL=$(LOCAL_ENDPOINT) AWS_ACCESS_KEY_ID=test AWS_SECRET_ACCESS_KEY=test AWS_REGION=us-east-1

local-up:
	@docker compose up -d
	@echo "Waiting for LocalStack..."
	@until curl -s $(LOCAL_ENDPOINT)/_localstack/health | grep -q '"s3": "available"'; do sleep 1; done
	@echo "LocalStack is ready"

local-down:
	@docker compose down

local-seed:
	@$(LOCAL_ENV) bash scripts/seed-localstack.sh

local-run: build
	@$(LOCAL_ENV) ./bin/$(APP)

local-reset:
	@docker compose down -v
	@$(MAKE) local-up
	@$(MAKE) local-seed
