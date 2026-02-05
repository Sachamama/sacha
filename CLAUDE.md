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

The project includes pre-commit hooks that run automatically before each commit:
1. **Format check** - Verifies code is formatted with gofumpt
2. **Lint** - Runs golangci-lint on changed files
3. **Tests** - Runs the full test suite with race detection

To enable hooks after cloning:
```bash
make setup
```

Or manually:
```bash
git config core.hooksPath .githooks
```

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
4. **Domain** (`internal/logs/`) - CloudWatch Logs client and business logic
5. **UI** (`internal/ui/`) - Bubble Tea TUI framework
   - `app/` - Main application shell (region/service switching)
   - `logs/` - CloudWatch Logs specific UI

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
- See `internal/dynamodb/client_test.go` and `internal/s3/client_test.go` for examples.

## UI Views & Keyboard Shortcuts

### Global Keys
- `r` - Change region
- `s` - Change service
- `ctrl+c` - Quit

### CloudWatch Logs (`internal/ui/logs/`)

**Log Groups View (default)**
- `↑/↓` or `j/k` - Navigate
- `/` - Search/filter
- `space` - Toggle selection
- `a` - Select all
- `c` - Create log group
- `t` - Start tailing selected groups

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
