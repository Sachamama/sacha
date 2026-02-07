# Contributing to Sacha

Thanks for your interest in contributing to sacha! This guide covers everything you need to get started.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Code Style](#code-style)
- [Commit Conventions](#commit-conventions)
- [Pull Request Process](#pull-request-process)
- [Adding a New AWS Service](#adding-a-new-aws-service)
- [Project Architecture](#project-architecture)
- [Testing](#testing)
- [Reporting Issues](#reporting-issues)

---

## Getting Started

### Prerequisites

- **Go 1.22+** &mdash; [go.dev/dl](https://go.dev/dl/)
- **gofumpt** &mdash; `go install mvdan.cc/gofumpt@latest`
- **golangci-lint** &mdash; [golangci-lint.run/usage/install](https://golangci-lint.run/usage/install/)
- **AWS credentials** &mdash; for manual testing

### Setup

```bash
git clone https://github.com/Sachamama/sacha.git
cd sacha
make setup   # installs git hooks and dev tools
make build   # compile to bin/sacha
make test    # run tests with race detection
```

`make setup` configures git to use the project's hooks in `.githooks/`.

---

## Development Workflow

### Branch Strategy

Use feature branches for all changes. Never commit directly to `main`.

```bash
# Create a feature branch
git checkout -b feature/my-feature

# Or for bug fixes
git checkout -b fix/my-fix
```

### Build & Test Commands

| Command | What it does |
|---------|--------------|
| `make build` | Compile binary to `bin/sacha` with version info |
| `make test` | Run all tests with race detection (`go test -race ./...`) |
| `make lint` | Run golangci-lint |
| `make fmt` | Format code with gofumpt |
| `make run` | Build and run the application |
| `make clean` | Remove `bin/` and `dist/` |

Run a single test:

```bash
go test -run TestName ./internal/config/
```

### Git Hooks

The project includes pre-commit and commit-msg hooks that run automatically after `make setup`:

**Pre-commit** checks:
1. Code formatting (gofumpt)
2. Lint (golangci-lint on changed files)
3. Tests (full suite with race detection)

**Commit message lint:**
- Enforces [Conventional Commits](https://www.conventionalcommits.org/) format (see below)

---

## Code Style

- Format with **gofumpt** (stricter than `gofmt`)
- Follow [Effective Go](https://go.dev/doc/effective_go) conventions
- Lint with **golangci-lint** (config in `.golangci.yml`)
- Keep functions focused and short
- Use meaningful variable names; avoid single-letter names outside of loops
- Handle errors explicitly; don't ignore them

---

## Commit Conventions

All commits must follow [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>[optional scope]: <description>
```

### Allowed Types

| Type | Use for |
|------|---------|
| `feat` | New feature |
| `fix` | Bug fix |
| `docs` | Documentation only |
| `chore` | Maintenance, dependencies |
| `refactor` | Code restructuring (no behavior change) |
| `test` | Adding or updating tests |
| `perf` | Performance improvement |
| `ci` | CI/CD changes |
| `build` | Build system changes |
| `style` | Code style (formatting, no logic change) |
| `revert` | Reverting a previous commit |

### Rules

- Description must start with a lowercase letter
- First line must be 72 characters or fewer
- Breaking changes use `!` suffix: `feat!: redesign config format`

### Examples

```
feat: add SQS queue browser with message peeking
fix: prevent duplicate pagination requests in S3 view
docs: add SSM Parameter Store to keybindings table
refactor: extract shared popup logic into helper
test: add pagination tests for Lambda client
```

---

## Pull Request Process

1. **Create a branch** from `main` with a descriptive name
2. **Make your changes** with clear, atomic commits
3. **Run checks locally:**
   ```bash
   make fmt
   make lint
   make test
   ```
4. **Push and open a PR** against `main`
5. **Fill out the PR template** with a summary, changes, and test plan
6. **Required checks** must pass: `test`, `lint`, `security`
7. **One approving review** is required before merging
8. PRs are **squash-merged** with automatic branch cleanup

### PR Title Guidelines

- Keep under 70 characters
- Use the same conventions as commit messages: `feat: add EC2 instance browser`
- Use the description for details, not the title

---

## Adding a New AWS Service

Sacha uses a plugin architecture. Each service implements the `awsx.Service` interface and registers itself in `main.go`. Here's the full process:

### 1. Create the domain client

Create `internal/<service>/client.go`:

```go
package myservice

import (
    "context"
    "github.com/aws/aws-sdk-go-v2/aws"
    // import the relevant AWS SDK service package
)

type Client struct {
    api *awsservice.Client
}

func NewClient(cfg aws.Config) *Client {
    return &Client{api: awsservice.NewFromConfig(cfg)}
}

// Add methods for listing, getting details, etc.
// Always accept pagination tokens and return next tokens.
```

### 2. Create domain types

Create `internal/<service>/types.go` with your domain models.

### 3. Write tests

Create `internal/<service>/client_test.go` with tests covering:
- First page load (token nil, returns a token)
- Continuation (passing a token returns next page)
- Last page (returned token is nil)
- Error handling

### 4. Build the UI model

Create the UI package `internal/ui/<service>/`:

- `service.go` &mdash; implements `awsx.Service` interface
- `model.go` &mdash; Bubble Tea model with state, keybindings, pagination
- `views.go` &mdash; rendering logic for two-pane layout
- `messages.go` &mdash; message types for Bubble Tea commands

Follow existing services for patterns (lazy loading threshold of 5 items, `ensureCursorVisible()`, scroll memory, etc.).

### 5. Register the service

In `cmd/sacha/main.go`, add your service to the `services` map:

```go
services["myservice"] = &myservice.MyServiceService{}
```

### 6. Create a label

Add an `area: <service>` label in the GitHub repository settings.

### Reference implementations

- **Simple service:** `internal/ui/lambda/` (list + expand popup)
- **Hierarchical navigation:** `internal/ui/ssm/` (path-based folder navigation)
- **Complex service:** `internal/ui/logs/` (multi-pane with tailing, multiple modes)

---

## Project Architecture

```
cmd/sacha/main.go            CLI entry point (Cobra)
internal/
  config/                    Configuration (CLI > env > file > defaults)
  aws/                       AWS SDK v2 abstraction + Service interface
  version/                   Build-time version info
  <service>/                 Domain client + types per service
  ui/
    app/                     Main app shell (region/service switching)
    <service>/               Bubble Tea model + views per service
```

### Key interfaces

```go
// internal/aws/service.go
type Service interface {
    Name() string
    Title() string
    Init(ctx context.Context, cfg aws.Config, opts ServiceOptions) (tea.Model, error)
}
```

### Key dependencies

| Package | Purpose |
|---------|---------|
| `github.com/charmbracelet/bubbletea` | TUI framework (Elm architecture) |
| `github.com/charmbracelet/bubbles` | TUI components (viewport, textinput) |
| `github.com/charmbracelet/lipgloss` | Terminal styling |
| `github.com/aws/aws-sdk-go-v2` | AWS SDK v2 |
| `github.com/spf13/cobra` | CLI framework |
| `github.com/rs/zerolog` | Structured logging |

---

## Testing

### Running Tests

```bash
# Full suite with race detection
make test

# Single test
go test -run TestListQueues ./internal/sqs/

# Verbose output
go test -v -run TestListQueues ./internal/sqs/

# With coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Writing Tests

- Use table-driven tests where appropriate
- Mock AWS API calls using interface-based clients
- Test pagination (first page, continuation, last page)
- Test error handling
- See `internal/dynamodb/client_test.go` and `internal/s3/client_test.go` for examples

---

## Reporting Issues

### Bug Reports

Use the [bug report template](https://github.com/Sachamama/sacha/issues/new?template=bug_report.yml) and include:
- Sacha version (`sacha --version`)
- Operating system and terminal
- Steps to reproduce
- Expected vs. actual behavior

### Feature Requests

Use the [feature request template](https://github.com/Sachamama/sacha/issues/new?template=feature_request.yml) and describe:
- The problem you're trying to solve
- Your proposed solution
- Which AWS service area it relates to

---

## License

By contributing to sacha, you agree that your contributions will be licensed under the [MIT License](LICENSE).
