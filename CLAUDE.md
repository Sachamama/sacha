# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Sacha is a keyboard-first AWS TUI (Terminal User Interface) inspired by classic two-pane file managers. It provides a split-pane interface for browsing, searching, and tailing CloudWatch Logs. Built with an extensible architecture to support additional AWS services.

## Build & Development Commands

```bash
make build      # Compiles binary to bin/sacha
make test       # Runs all tests (go test ./...)
make run        # Runs the application directly
make lint       # Runs golangci-lint (if installed)
```

Run a single test:
```bash
go test -run TestName ./internal/config/
```

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
- `↑/↓` or `j/k` - Navigate
- `/` - Search/filter
- `enter` - Open folder
- `space` - Toggle selection
- `a` - Select all
- `d` - Download selected
- `D` - Delete selected (with confirmation)
- `p` - Preview text file
- `y` - Copy S3 URI
- `esc/backspace/h` - Go back

**Preview Mode**
- `↑/↓` - Scroll
- `p/esc` - Close preview

**Delete Confirmation**
- `y` - Confirm
- `n/esc` - Cancel
