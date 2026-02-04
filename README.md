# sacha

<a href="https://buymeacoffee.com/sachamama"><img src="https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png" alt="Buy Me A Coffee" height="36"></a>

sacha is a keyboard-first AWS TUI inspired by classic two-pane file managers. It keeps you in the terminal while you browse, search, and manage AWS resources without bouncing between consoles. Currently supports CloudWatch Logs and S3, with an extensible architecture for more AWS services.

## Highlights
- Two-pane TUI for fast AWS resource exploration.
- **CloudWatch Logs**: Search, multi-select, and tail multiple log groups. JSON logs auto-format as tables.
- **S3**: Browse buckets, download/delete files, preview text content.
- Remembers your last region/service and plays nicely with AWS profiles.
- Minimal dependencies; install and run with a single command.

## What’s in a name?
- `sachamama` comes from Quechua and means “mother of the forest,” which is also the username of the author.
- `sacha` shortens the idea to “forest,” reflecting how the tool helps you see the bigger AWS landscape without getting lost in individual trees.

## Install

Prerequisites: Go 1.22+ and AWS credentials that can read CloudWatch Logs.

### Pre-built Binaries

Download the latest release for your platform:

```bash
# macOS (Apple Silicon)
curl -Lo sacha.tar.gz https://github.com/Sachamama/sacha/releases/latest/download/sacha_VERSION_darwin_arm64.tar.gz
tar xzf sacha.tar.gz && sudo mv sacha /usr/local/bin/
rm sacha.tar.gz

# macOS (Intel)
curl -Lo sacha.tar.gz https://github.com/Sachamama/sacha/releases/latest/download/sacha_VERSION_darwin_amd64.tar.gz
tar xzf sacha.tar.gz && sudo mv sacha /usr/local/bin/
rm sacha.tar.gz

# Linux (x86_64)
curl -Lo sacha.tar.gz https://github.com/Sachamama/sacha/releases/latest/download/sacha_VERSION_linux_amd64.tar.gz
tar xzf sacha.tar.gz && sudo mv sacha /usr/local/bin/
rm sacha.tar.gz

# Linux (ARM64)
curl -Lo sacha.tar.gz https://github.com/Sachamama/sacha/releases/latest/download/sacha_VERSION_linux_arm64.tar.gz
tar xzf sacha.tar.gz && sudo mv sacha /usr/local/bin/
rm sacha.tar.gz

# Windows (PowerShell)
curl -Lo sacha.zip https://github.com/Sachamama/sacha/releases/latest/download/sacha_VERSION_windows_amd64.zip
Expand-Archive sacha.zip -DestinationPath .
# Add sacha.exe to your PATH
```

Replace `VERSION` with the actual version number (e.g., `1.0.0`), or browse [releases](https://github.com/Sachamama/sacha/releases) to download directly.

### Other Install Methods

- With Go: `go install github.com/sachamama/sacha/cmd/sacha@latest`
- From source: `make build` (binary at `bin/sacha`)

## Update

Check your current version with `sacha --version`.

To update sacha to the latest version:
- **Pre-built binary**: Re-run the install command from the [Install](#install) section with the new version.
- **Go install**: `go install github.com/sachamama/sacha/cmd/sacha@latest`
- **From source**: `git pull && make build`

## Versioning

Sacha follows [semantic versioning](https://semver.org/). Releases are tagged as `vMAJOR.MINOR.PATCH` (e.g., `v1.2.3`).

New releases are automatically built and published to [GitHub Releases](https://github.com/Sachamama/sacha/releases) when version tags are pushed. Each release includes:
- Pre-built binaries for Linux, macOS, and Windows (amd64 and arm64)
- Checksums for verification
- Automated changelog with features, bug fixes, and performance improvements

## Quickstart

```
make run
# or directly after install
sacha --profile my-aws-profile --region us-east-1
```

Global flags:
- `--profile` – AWS profile to use
- `--region` – AWS region
- `--service` – AWS service (`cloudwatch-logs`, `s3`)
- `--verbose` – enable debug logging
- `--version` – show version information

Configuration lives under the OS config directory (e.g. `~/.config/sacha/config.json`) and stores defaults plus your last used region/service. Precedence: CLI flags > env (`AWS_PROFILE`, `AWS_REGION`, `AWS_DEFAULT_REGION`) > config file > AWS SDK defaults.

## Features

### CloudWatch Logs
- Split-pane TUI: left pane lists log groups; right pane tails logs.
- Log group list with search (`/`), cursor navigation, multi-select.
- Create new log groups with `c`.
- Tail multiple log groups simultaneously with `t`.
- Switch between left (groups) and right (tail) panels with `tab`, `left/h`, or `right/l`.
- Focused panel highlighted with colored border; up/down navigation works within focused panel.
- Dynamic log refresh: logs automatically refresh when log group selection changes while tailing (press `space` or `a` on groups panel).
- JSON log detection with automatic table view displaying TIME, GROUP, and JSON fields.
- Table uses full available width with proper padding; last column expands to fill space.
- Toggle between table and plain view with `v`.
- Navigate log events with arrows and expand to see full message (pretty-printed JSON) with scrollable view.
- Stop tailing with `x` to reset the log panel and return to group selection.
- Fullscreen tail mode with `f`; use left/right arrows or `h/l` to scroll horizontally for wide log lines.

### S3
- Browse buckets and objects in a two-pane interface.
- Navigate into folders, go back with `esc`/`backspace`.
- Multi-select files for batch operations.
- Download files with `d`, delete with `D`.
- Preview text files with `p`.
- Copy S3 URI to clipboard with `y`.

## Keybindings

### Global
| Key | Action |
|-----|--------|
| `r` | Change region |
| `s` | Change service |
| `Ctrl+C` | Quit |

### CloudWatch Logs
| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate |
| `/` | Search/filter |
| `space` | Toggle selection |
| `a` | Select all |
| `c` | Create log group |
| `t` | Start tailing |
| `tab`, `left/h`, `right/l` | Switch panel focus (while tailing) |
| `enter/space` | Expand log event (while tailing) |
| `↑/↓` or `j/k` | Scroll in expanded view |
| `pgup/pgdn` | Page scroll in expanded view |
| `esc` | Close expanded view |
| `v` | Toggle table/plain view (while tailing) |
| `x` | Stop tailing |
| `f` | Toggle fullscreen (while tailing) |
| `←/→` or `h/l` | Scroll horizontally (fullscreen only) |
| `q/esc` | Stop tailing |

### S3
| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate |
| `/` | Search/filter |
| `enter` | Open bucket/folder |
| `space` | Toggle selection |
| `a` | Select all |
| `d` | Download |
| `D` | Delete |
| `p` | Preview text file |
| `y` | Copy S3 URI |
| `esc/backspace` | Go back |

## Development

```
make test
make run
```

### Adding services

Implement the `awsx.Service` interface, register the service in `cmd/sacha/main.go`, and provide a TUI model under `internal/ui/<service>`. Services receive AWS config scoped to the active region/profile.
