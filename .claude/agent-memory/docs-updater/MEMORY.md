# Documentation Update Patterns for Sacha

## Project Overview
- Go-based AWS TUI (Terminal User Interface) using Bubble Tea framework
- Two documentation files: README.md (user-facing) and CLAUDE.md (AI/developer-facing)
- Active development with frequent UI/UX improvements

## Documentation Structure

### README.md
- Highlights section: high-level feature overview
- Features section: detailed feature descriptions per service
- Keybindings section: comprehensive keyboard shortcuts organized by service
- Focus: end-user perspective, what the tool does

### CLAUDE.md
- Architecture section: technical implementation details
- UI Views & Keyboard Shortcuts section: implementation-aware documentation
- Focus: developer/AI perspective, how it works
- More technical details about internal behavior (e.g., time windows, panel focus states)

## Key Patterns
- When documenting table/list views, specify column names and layout behavior
- UI changes often affect both README and CLAUDE simultaneously
- Keep feature descriptions concise but specific (e.g., "TIME, GROUP, JSON fields" not "multiple columns")
- Width/layout details matter for TUI applications - mention when components expand to fill space
- Keybindings table in README: simple format, action only
- CLAUDE.md keyboard section: can include technical context and implementation notes

## New Feature Documentation Pattern (observed 2026-02-04)
When documenting interactive features:
1. In README Features section: describe what happens (user perspective)
   - Example: "Dynamic log refresh: logs automatically refresh when log group selection changes while tailing"
2. In README Keybindings: add key with simple action name
   - Example: `x` | Clear logs
3. In CLAUDE.md: add technical context (what gets reset, timing details)
   - Example: "Clear/reset logs pane (clears all events and resets to 15-minute window from current time, continues tailing)"

## Implementation Details to Watch
- CloudWatch Logs tail view uses dynamic table rendering (`internal/ui/logs/views.go`)
- Table layout: fixed columns (TIME, GROUP) + dynamic JSON fields + last column expansion
- JSON detection drives view mode (table vs plain)
- Selection changes while tailing trigger log refresh (lines 252-259, 269-276 in model.go)
- Clear operation (key 'x') resets events slice and tailStart timestamp
