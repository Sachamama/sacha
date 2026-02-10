# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sacha is a keyboard-first AWS TUI (Terminal User Interface) inspired by classic two-pane file managers. It provides a split-pane interface for browsing, searching, and tailing CloudWatch Logs. Built with an extensible architecture to support additional AWS services.

## Build & Development Commands

```bash
make setup             # Install git hooks and dev tools (run once after clone)
make build             # Compiles binary to bin/sacha with version info
make install           # Installs binary to $GOPATH/bin with version info
make test              # Runs all tests (go test -race ./...)
make run               # Runs the application directly
make lint              # Runs golangci-lint (if installed)
make fmt               # Format code with gofumpt
make clean             # Removes bin/ and dist/ directories
make snapshot          # Creates a local snapshot release (requires goreleaser)
make release-dry-run   # Tests release build without publishing (requires goreleaser)
```

Run a single test:
```bash
go test -run TestName ./internal/config/
```

### Git Hooks

The project includes hooks that run automatically:

**Pre-commit** (`.githooks/pre-commit`):
1. **Format check** - Verifies code is formatted with gofumpt
2. **Lint** - Runs golangci-lint on changed files
3. **Tests** - Runs the full test suite with race detection

**Commit message lint** (`.githooks/commit-msg`):
- Enforces [Conventional Commits](https://www.conventionalcommits.org/) format: `<type>[scope]: <description>`
- Allowed types: `feat`, `fix`, `docs`, `chore`, `refactor`, `test`, `perf`, `ci`, `build`, `style`, `revert`
- Description must start lowercase, first line max 72 characters
- Breaking changes use `!` suffix: `feat!: redesign config format`
- Merge commits are allowed through automatically

To enable hooks after cloning:
```bash
make setup
```

Or manually:
```bash
git config core.hooksPath .githooks
```

### Git Worktree Workflow

Always use `git worktree` to create feature or fix branches. This avoids conflicts with uncommitted work on the main branch and keeps the primary working tree clean.

```bash
# Create a worktree with a new feature branch
git worktree add ../sacha-<short-name> -b feature/<branch-name>

# Work, commit, and push from the worktree directory
cd ../sacha-<short-name>
# ... make changes ...
git add <files> && git commit -m "feat: description"
git push -u origin feature/<branch-name>

# Clean up after PR is merged
git worktree remove ../sacha-<short-name>
```

Key rules:
- Never commit directly to `main` — always use a feature branch via worktree
- Name worktree directories `../sacha-<short-name>` to keep them adjacent to the main repo
- Name branches `feature/<descriptive-name>` or `fix/<descriptive-name>` for AI-authored changes
- Remove worktrees after the PR is merged to avoid stale checkouts

### Version Information

Version information is injected at build time via ldflags into `internal/version/version.go`. The Makefile automatically sets:
- `Version` - Git tag or "dev" if no tag
- `Commit` - Short git commit SHA
- `Date` - Build timestamp

Access version info via CLI: `sacha --version`

## Architecture

### Layers
1. **CLI** (`cmd/sacha/main.go`) - Cobra-based CLI entry point with flags for profile, region, service, verbose
2. **Config** (`internal/config/`) - Configuration with precedence: CLI > Env > File > Defaults
3. **AWS** (`internal/aws/`) - AWS SDK v2 abstraction with pluggable Service interface
4. **Domain** - Service-specific clients and business logic
   - `internal/logs/` - CloudWatch Logs
   - `internal/s3/` - S3
   - `internal/dynamodb/` - DynamoDB
   - `internal/lambda/` - Lambda
   - `internal/ssm/` - SSM Parameter Store
   - `internal/sqs/` - SQS
   - `internal/ec2/` - EC2
5. **UI** (`internal/ui/`) - Bubble Tea TUI framework
   - `app/` - Main application shell (region/service switching)
   - `logs/` - CloudWatch Logs specific UI
   - `s3/` - S3 browser UI
   - `dynamodb/` - DynamoDB browser UI
   - `lambda/` - Lambda browser UI
   - `ssm/` - SSM Parameter Store browser UI
   - `sqs/` - SQS Queue browser UI
   - `ec2/` - EC2 Instance browser UI

### Plugin Architecture

Services implement the `awsx.Service` interface (`internal/aws/service.go`):
```go
type Service interface {
    Name() string
    Title() string
    Init(ctx context.Context, cfg aws.Config, opts ServiceOptions) (tea.Model, error)
}
```

Services are registered in `cmd/sacha/main.go` and provide a TUI model under `internal/ui/<service>`.

### Key Dependencies
- `github.com/aws/aws-sdk-go-v2` - AWS SDK v2
- `github.com/charmbracelet/bubbletea` - TUI framework
- `github.com/charmbracelet/bubbles` - TUI components
- `github.com/charmbracelet/lipgloss` - Terminal styling
- `github.com/spf13/cobra` - CLI framework
- `github.com/rs/zerolog` - Structured logging

## Configuration

Config file: `~/.config/sacha/config.json`

Resolution precedence:
1. CLI flags (`--profile`, `--region`, `--service`)
2. Environment variables (`AWS_PROFILE`, `AWS_REGION`, `AWS_DEFAULT_REGION`)
3. Config file
4. AWS SDK defaults

## CI/CD & Release Process

### GitHub Actions Workflows

**CI Pipeline** (`.github/workflows/ci.yml`)
- Triggers on: PRs and pushes to main branch
- Jobs:
  - **Test**: Runs tests with race detection and coverage
  - **Lint**: Runs golangci-lint for code quality
  - **Security**: Runs govulncheck to scan for known vulnerabilities

**Release Pipeline** (`.github/workflows/release.yml`)
- Triggers on: Version tags pushed (e.g., `v1.2.3`)
- Runs tests, then uses GoReleaser to build and publish release

### Creating a Release

1. Tag the commit with a semantic version: `git tag v1.2.3`
2. Push the tag: `git push origin v1.2.3`
3. GitHub Actions automatically:
   - Runs tests
   - Builds binaries for all platforms (Linux, macOS, Windows; amd64 and arm64)
   - Creates GitHub Release with changelog and download links
   - Publishes binaries and checksums

### GoReleaser Configuration

`.goreleaser.yml` defines:
- Cross-platform build targets (darwin, linux, windows; amd64, arm64)
- Version injection via ldflags into `internal/version` package
- Archive formats (tar.gz for Unix, zip for Windows)
- Changelog generation (groups commits by type: features, bug fixes, performance)
- Release template with install instructions
- Homebrew tap integration (optional, requires token)

## GitHub Repository Best Practices

The repository is configured with the following GitHub best practices. Maintain these when making changes.

### Branch Protection (`main`)

- **1 approving review** required on all PRs
- **Stale reviews dismissed** automatically when new commits are pushed
- **Required status checks** (`test`, `lint`, `security`) must pass and be up-to-date with base
- **Linear history** enforced (no merge commits)
- **Force pushes** and **branch deletions** blocked
- **Conversation resolution** required before merging

### Merge Strategy

- **Squash merge only** — merge commits and rebase merges are disabled
- **Delete branch on merge** enabled — head branches are cleaned up automatically
- **Auto-merge** enabled — PRs can be set to merge automatically once checks pass

### Dependabot (`.github/dependabot.yml`)

- **Go modules**: Weekly updates on Mondays, grouped by `aws-sdk` and `charmbracelet`
- **GitHub Actions**: Weekly updates on Mondays
- Dependency PRs are auto-labeled (`type: dependencies` or `type: ci`)

### Security

- **Dependabot vulnerability alerts** enabled
- **Automated security fixes** enabled
- **Secret scanning** + **push protection** enabled
- **SECURITY.md** defines responsible disclosure policy

### Issue & PR Templates

- **Bug report** (`.github/ISSUE_TEMPLATE/bug_report.yml`) — structured form with version, OS, repro steps
- **Feature request** (`.github/ISSUE_TEMPLATE/feature_request.yml`) — problem, solution, area dropdown
- **PR template** (`.github/pull_request_template.md`) — summary, changes, test plan, related issues

### Labels

Beyond GitHub defaults, the repo uses structured labels:

| Category | Labels |
|----------|--------|
| Priority | `priority: critical`, `priority: high`, `priority: medium`, `priority: low` |
| Area | `area: cloudwatch`, `area: s3`, `area: dynamodb`, `area: lambda`, `area: ui`, `area: config` |
| Type | `type: breaking-change`, `type: refactor`, `type: performance`, `type: security`, `type: dependencies`, `type: ci` |

When adding a new AWS service, create a corresponding `area: <service>` label.

## Pagination & Scroll Patterns

All list/table views that display AWS resources must handle pagination and scroll correctly. Follow these patterns when adding or modifying views.

### AWS API Pagination

Each AWS service uses token-based pagination. The client methods accept a pagination token and return the next token:

| Service | Endpoint | Token Field | Page Size | Location |
|---------|----------|-------------|-----------|----------|
| DynamoDB | ListTables | `ExclusiveStartTableName` / `LastEvaluatedTableName` | 100 | `internal/dynamodb/client.go` |
| DynamoDB | Scan | `ExclusiveStartKey` / `LastEvaluatedKey` | 25 | `internal/dynamodb/client.go` |
| S3 | ListObjectsV2 | `ContinuationToken` / `NextContinuationToken` | 1000 | `internal/s3/client.go` |
| CloudWatch | DescribeLogGroups | `NextToken` | 50 | `internal/logs/client.go` |
| Lambda | ListFunctions | `Marker` / `NextMarker` | 50 | `internal/lambda/client.go` |
| SSM | GetParametersByPath | `NextToken` | 50 | `internal/ssm/client.go` |
| SSM | DescribeParameters | `NextToken` | 50 | `internal/ssm/client.go` |
| SQS | ListQueues | `NextToken` | 50 | `internal/sqs/client.go` |
| EC2 | DescribeInstances | `NextToken` | 50 | `internal/ec2/client.go` |

- Client methods must accept an optional pagination token parameter and return the next token alongside results.
- Never fetch all pages eagerly on init unless the dataset is known to be small. Load one page, then lazy-load more.

### Lazy Loading (Infinite Scroll)

Views that paginate must trigger loading the next page **before** the user hits the end of the list. The standard threshold is **5 items from the bottom**:

```go
if m.cursor >= len(m.filteredItems())-5 && m.nextToken != nil && !m.loadingMore {
    m.loadingMore = true
    return m, tea.Batch(cmd, m.loadMoreCmd())
}
```

Key rules:
- Check the lazy-load condition inside the cursor-down key handler (`j`, `↓`, or equivalent).
- Guard with `!m.loadingMore` to prevent duplicate requests.
- Set `m.loadingMore = true` before dispatching the command; reset it when the result message is handled.
- Append new items to the existing slice — never replace the full list on a lazy-load.

### "Load All" Pattern

Some views (e.g., S3 objects with `A` key) allow fetching every remaining page at once. Implement this as a loop inside a `tea.Cmd` that iterates until the token is nil, then returns a single message with all accumulated results. Show a status line (e.g., "Loading all items...") while the operation runs.

### Scroll Offset & Cursor Visibility

Every list view tracks a `listOffset int` that determines which items are visible on screen. After any cursor movement or list mutation, call `ensureCursorVisible()` to clamp the offset:

```go
func (m *Model) ensureCursorVisible() {
    visibleHeight := m.height - headerLines - footerLines
    if m.cursor < m.listOffset {
        m.listOffset = m.cursor
    }
    if m.cursor >= m.listOffset+visibleHeight {
        m.listOffset = m.cursor - visibleHeight + 1
    }
}
```

- Recalculate visible height on terminal resize (`tea.WindowSizeMsg`).
- When filtering changes the list length, clamp the cursor to `len(items)-1` and re-run `ensureCursorVisible()`.

### Horizontal Scrolling

The CloudWatch Logs tail view supports horizontal scrolling in fullscreen mode via `scrollX int`. Scroll increments by 10 characters per key press (`←`/`→` or `h`/`l`). Clamp `scrollX` to `>= 0`.

### Viewport-Based Scrolling

Expanded popups and preview modes use `viewport.Model` from the Bubbles library. When setting viewport dimensions, account for borders and padding:

```go
m.viewport.Width = m.width - borderHorizontal
m.viewport.Height = m.height - borderVertical - headerHeight
```

Viewport handles its own scroll state (`↑/↓`, `pgup/pgdn`). Update `viewport.SetContent()` when the underlying data changes, and call `viewport.GotoTop()` when switching to a new item.

### Pagination State in Models

Each UI model must store:
- **Pagination token** (e.g., `nextToken *string`, `lastEvaluatedKey map[string]interface{}`) — nil means no more pages.
- **`loadingMore bool`** — prevents concurrent pagination requests.
- **Items slice** — append-only during lazy loads; replace on full refresh or navigation change.

### Message Types

Define distinct message types for initial loads vs. lazy-load continuations:
- `itemsLoadedMsg` — replaces the full list (initial load or navigation).
- `moreItemsLoadedMsg` — appends to the existing list (lazy-load page).
- `allItemsLoadedMsg` — replaces remaining items after a "load all" operation.

Each message should carry both the items and the next pagination token.

### Testing Pagination

Write tests for:
- First page load (token is nil initially, returns a token).
- Continuation (passing a token returns next page and possibly another token).
- Last page (returned token is nil, signaling no more data).
- See `internal/dynamodb/client_test.go`, `internal/s3/client_test.go`, and `internal/lambda/client_test.go` for examples.

### Scroll Memory

When users navigate into a sub-view (e.g., opening a DynamoDB table, entering an S3 bucket/folder) and then go back, the cursor position and scroll offset are restored to where they were before. This is implemented via:
- **DynamoDB**: `savedCursor` / `savedListOffset` fields saved on enter, restored on back.
- **S3**: A `scrollStack []scrollPosition` that pushes on enter (bucket or folder) and pops on back, supporting arbitrary folder depth.

## UI Views & Keyboard Shortcuts

### Global Keys
- `r` - Change region
- `s` - Change service
- `ctrl+c` - Quit

### CloudWatch Logs (`internal/ui/logs/`)

**Log Groups View (default)**
- Split-pane mode: left panel shows log groups list, right panel shows selected group details
- Right panel displays: log group name, retention policy, stored bytes, and creation date (when not tailing)
- `↑/↓` or `j/k` - Navigate
- `/` - Search/filter
- `space` - Toggle selection
- `a` - Select all
- `c` - Create log group
- `d` - Delete selected log groups (shows confirmation prompt)
- `R` - Set retention policy on selected groups (opens retention picker: 1d, 3d, 5d, 7d, 14d, 30d, 60d, 90d, 1y, never)
- `t` - Start tailing selected groups
- StatusHelp text (non-tailing): `↑↓ move, / search, space select, a all, c create, d delete, R retention, t tail`

**Tailing View**
- Split-pane mode: left panel shows groups list, right panel shows tail output
- Panel switching: `tab`, `left/h`, `right/l` to switch focus between panels
- Focused panel has colored border (pink/212); up/down navigation works within focused panel
- Left panel (groups): search, select, toggle all functionality remains active
  - **Dynamic refresh**: Changing log group selection (via `space` or `a`) while tailing automatically clears events and restarts tailing with new selection
- Right panel (tail): event navigation, expand, toggle view modes
- Table columns: TIME (HH:MM:SS), GROUP (log group name), and JSON fields (when detected)
- Table uses full available width with proportional column sizing
- Last column expands to fill remaining space
- `↑/↓` or `j/k` - Navigate within focused panel
- `enter` or `space` - Expand selected event (popup with formatted JSON) when tail panel focused
- `v` - Toggle view mode (table/plain)
- `x` - Stop tailing (stops watching logs and resets the log panel, same behavior as `q`)
- `f` - Toggle fullscreen mode (focuses tail panel, hides groups panel)
- `←/→` or `h/l` - Scroll horizontally (fullscreen mode only, for viewing wide log lines)
- `q/esc` - Stop tailing

**Expanded Event Popup**
- Accessed by pressing `enter` or `space` on a selected log event while tailing
- `↑/↓` or `j/k` - Scroll up/down
- `pgup/pgdn` - Page up/down
- `esc` - Close popup

### S3 (`internal/ui/s3/`)

**Buckets View**
- `↑/↓` or `j/k` - Navigate
- `/` - Search/filter
- `enter` - Open bucket
- `y` - Copy S3 URI

**Objects View (inside bucket)**
- `↑/↓` or `j/k` - Navigate (lazy loads more items near end)
- `/` - Search/filter
- `enter` - Open folder
- `space` - Toggle selection (files and folders)
- `a` - Toggle all (current page)
- `A` - Load all pages and select all
- `d` - Download selected (folders downloaded recursively to `./sacha-downloads/`)
- `p` - Preview text file
- `y` - Copy S3 URI
- `esc/backspace/h` - Go back

**Preview Mode**
- `↑/↓` - Scroll
- `p/esc` - Close preview

### DynamoDB (`internal/ui/dynamodb/`)

**Tables View (default)**
- `↑/↓` or `j/k` - Navigate (lazy loads more tables near end)
- `/` - Search/filter
- `enter` - Open table (scan items)
- `y` - Copy table ARN

**Items View (inside table)**
- `↑/↓` or `j/k` - Navigate (lazy loads more items near end)
- `/` - Search/filter items by value
- `enter` or `space` - Expand selected item (popup with full attribute details)
- `y` - Copy table ARN
- `esc/backspace/h` - Go back to tables list

**Expanded Item Popup**
- Accessed by pressing `enter` or `space` on a selected item
- `↑/↓` or `j/k` - Scroll up/down
- `pgup/pgdn` - Page up/down
- `esc` - Close popup

### Lambda (`internal/ui/lambda/`)

**Functions View (default)**
- `↑/↓` or `j/k` - Navigate (lazy loads more functions near end)
- `/` - Search/filter by name or runtime
- `enter` or `space` - Expand selected function (popup with full details, env vars, layers)
- `y` - Copy function ARN

**Expanded Function Popup**
- Accessed by pressing `enter` or `space` on a selected function
- `↑/↓` or `j/k` - Scroll up/down
- `pgup/pgdn` - Page up/down
- `esc` - Close popup

### SSM Parameter Store (`internal/ui/ssm/`)

**Parameters View (default)**
- `↑/↓` or `j/k` - Navigate (lazy loads more parameters near end)
- `/` - Search/filter by name
- `enter` or `space` - Navigate into path prefix (folder) or expand parameter details
- `y` - Copy parameter value or path
- `esc/backspace/h` - Go back up one level

**Expanded Parameter Popup**
- Accessed by pressing `enter` or `space` on a leaf parameter
- `↑/↓` or `j/k` - Scroll up/down
- `pgup/pgdn` - Page up/down
- `esc` - Close popup

### SQS (`internal/ui/sqs/`)

**Queues View (default)**
- `↑/↓` or `j/k` - Navigate (lazy loads more queues near end)
- `/` - Search/filter by name
- `enter` - Peek messages (receives with visibility timeout 0)
- `space` - Expand queue details in popup
- `y` - Copy queue URL

**Messages View (after peeking)**
- `↑/↓` or `j/k` - Navigate messages
- `enter` or `space` - Expand message in popup (pretty-printed JSON body)
- `y` - Copy message body
- `esc/backspace/h` - Go back to queues

**Expanded Popup (queue or message)**
- `↑/↓` or `j/k` - Scroll up/down
- `pgup/pgdn` - Page up/down
- `esc` - Close popup

### EC2 (`internal/ui/ec2/`)

**Instances View (default)**
- `↑/↓` or `j/k` - Navigate (lazy loads more instances near end)
- `/` - Search/filter by name, instance ID, type, state, or IP
- `enter` or `space` - Expand instance details in popup (full metadata and tags)
- `y` - Copy instance ID

**Expanded Instance Popup**
- `↑/↓` or `j/k` - Scroll up/down
- `pgup/pgdn` - Page up/down
- `esc` - Close popup
