# sacha

<a href="https://buymeacoffee.com/sachamama"><img src="https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png" alt="Buy Me A Coffee" height="36"></a>

sacha is a keyboard-first AWS TUI inspired by classic two-pane file managers. It keeps you in the terminal while you browse, search, and manage AWS resources without bouncing between consoles. Currently supports CloudWatch Logs, S3, DynamoDB, Lambda, SSM Parameter Store, and SQS, with an extensible architecture for more AWS services.

## Highlights
- Two-pane TUI for fast AWS resource exploration.
- **CloudWatch Logs**: Search, multi-select, and tail multiple log groups. JSON logs auto-format as tables.
- **S3**: Browse buckets, download files and folders recursively, preview text content.
- **DynamoDB**: Browse tables, scan items with lazy loading, view table details and item attributes.
- **Lambda**: Browse functions, view configuration details, environment variables, and layers.
- **SSM Parameter Store**: Browse parameters by path hierarchy, view values with decryption, navigate folders.
- **SQS**: Browse queues, view message counts and attributes, peek messages without consuming them.
- Remembers your last region/service and plays nicely with AWS profiles.
- Minimal dependencies; install and run with a single command.

## What’s in a name?
- `sachamama` comes from Quechua and means “mother of the forest,” which is also the username of the author.
- `sacha` shortens the idea to “forest,” reflecting how the tool helps you see the bigger AWS landscape without getting lost in individual trees.

## Install

Prerequisites: AWS credentials that can read CloudWatch Logs.

### Homebrew (macOS/Linux)

```bash
brew install sachamama/tap/sacha
```

### Go Install

```bash
go install github.com/sachamama/sacha/cmd/sacha@latest
```

Requires Go 1.22+.

### Pre-built Binaries

Download from [releases](https://github.com/Sachamama/sacha/releases):

```bash
# macOS (Apple Silicon)
curl -Lo sacha.tar.gz https://github.com/Sachamama/sacha/releases/latest/download/sacha_VERSION_darwin_arm64.tar.gz
tar xzf sacha.tar.gz && sudo mv sacha /usr/local/bin/

# macOS (Intel)
curl -Lo sacha.tar.gz https://github.com/Sachamama/sacha/releases/latest/download/sacha_VERSION_darwin_amd64.tar.gz
tar xzf sacha.tar.gz && sudo mv sacha /usr/local/bin/

# Linux (x86_64)
curl -Lo sacha.tar.gz https://github.com/Sachamama/sacha/releases/latest/download/sacha_VERSION_linux_amd64.tar.gz
tar xzf sacha.tar.gz && sudo mv sacha /usr/local/bin/

# Linux (ARM64)
curl -Lo sacha.tar.gz https://github.com/Sachamama/sacha/releases/latest/download/sacha_VERSION_linux_arm64.tar.gz
tar xzf sacha.tar.gz && sudo mv sacha /usr/local/bin/
```

Replace `VERSION` with the actual version (e.g., `1.0.0`).

### From Source

```bash
git clone https://github.com/Sachamama/sacha.git
cd sacha
make build  # binary at bin/sacha
```

## Update

Check your current version with `sacha --version`.

To update sacha to the latest version:
- **Homebrew**: `brew upgrade sacha`
- **Go install**: `go install github.com/sachamama/sacha/cmd/sacha@latest`
- **Pre-built binary**: Download the new version from [releases](https://github.com/Sachamama/sacha/releases)
- **From source**: `git pull && make build`

## Versioning

Sacha follows [semantic versioning](https://semver.org/). Releases are tagged as `vMAJOR.MINOR.PATCH` (e.g., `v1.2.3`).

New releases are automatically built and published when version tags are pushed:
- [GitHub Releases](https://github.com/Sachamama/sacha/releases) with pre-built binaries for Linux, macOS, and Windows (amd64 and arm64)
- [Homebrew tap](https://github.com/Sachamama/homebrew-tap) for easy installation via `brew`
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
- `--service` – AWS service (`cloudwatch-logs`, `s3`, `dynamodb`, `lambda`, `ssm`, `sqs`)
- `--verbose` – enable debug logging
- `--version` – show version information

Configuration lives under the OS config directory (e.g. `~/.config/sacha/config.json`) and stores defaults plus your last used region/service. Precedence: CLI flags > env (`AWS_PROFILE`, `AWS_REGION`, `AWS_DEFAULT_REGION`) > config file > AWS SDK defaults.

## Features

### CloudWatch Logs
- Split-pane TUI: left pane lists log groups; right pane shows group details or tails logs.
- Log group list with search (`/`), cursor navigation, multi-select.
- Right pane displays log group details: name, retention policy, stored bytes, and creation date.
- Create new log groups with `c`.
- Delete selected log groups with `d` (confirmation prompt).
- Set retention policy on selected groups with `R` (picker shows: 1d, 3d, 5d, 7d, 14d, 30d, 60d, 90d, 1y, never).
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
- Multi-select files and folders for batch operations.
- Download files and folders recursively with `d`.
- Lazy loading with pagination (loads more items as you scroll).
- Select all items including paginated results with `A`.
- Preview text files with `p`.
- Copy S3 URI to clipboard with `y`.
- Downloads saved to `./sacha-downloads/` preserving folder structure.

### DynamoDB
- Browse tables in a two-pane interface with table details in the right panel.
- View table metadata: status, item count, size, billing mode, key schema, and GSIs.
- Scan table items with paginated lazy loading.
- Search/filter tables and items with `/`.
- Expand any item to view full attribute details in a scrollable popup.
- Copy table ARN to clipboard with `y`.
- Supports all DynamoDB attribute types: strings, numbers, booleans, binary, sets, lists, and maps.

### Lambda
- Browse Lambda functions in a two-pane interface with function details in the right panel.
- View function configuration: runtime, handler, memory, timeout, code size, state, and architecture.
- Search/filter functions by name or runtime with `/`.
- Expand any function to view full details including environment variables and layers in a scrollable popup.
- Copy function ARN to clipboard with `y`.
- Lazy loading with pagination for large function lists.

### SSM Parameter Store
- Browse parameters by path hierarchy in a two-pane interface.
- Navigate into path prefixes like folders with `enter`, go back with `esc`/`backspace`.
- View parameter details: value (with decryption), type, version, last modified, ARN.
- Expand any parameter to view full details in a scrollable popup.
- Copy parameter value or path to clipboard with `y`.
- Search/filter parameters with `/`.
- Lazy loading with pagination for large parameter sets.
- Supports String, StringList, and SecureString parameter types.

### SQS
- Browse SQS queues in a two-pane interface with queue attributes in the right panel.
- View message counts (approximate visible, in-flight, delayed), queue type (FIFO/Standard), visibility timeout, redrive policy.
- Peek messages with `enter` — uses `ReceiveMessage` with visibility timeout 0 so messages stay in the queue.
- Navigate peeked messages, view message body (JSON auto-formatted), expand in scrollable popup.
- Copy queue URL with `y`; copy message body with `y` in message view.
- Search/filter queues with `/`.
- Lazy loading with pagination for large queue lists.

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
| `d` | Delete selected log groups |
| `R` | Set retention policy |
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
| `space` | Toggle selection (files and folders) |
| `a` | Toggle all (current page) |
| `A` | Load all pages and select all |
| `d` | Download (folders downloaded recursively) |
| `p` | Preview text file |
| `y` | Copy S3 URI |
| `esc/backspace` | Go back |

### DynamoDB
| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate |
| `/` | Search/filter |
| `enter` | Open table (scan items) |
| `enter/space` | Expand item (in items view) |
| `y` | Copy table ARN |
| `↑/↓` or `j/k` | Scroll in expanded view |
| `pgup/pgdn` | Page scroll in expanded view |
| `esc` | Close expanded view |
| `esc/backspace/h` | Go back to tables |

### Lambda
| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate |
| `/` | Search/filter |
| `enter/space` | Expand function details |
| `y` | Copy function ARN |
| `↑/↓` or `j/k` | Scroll in expanded view |
| `pgup/pgdn` | Page scroll in expanded view |
| `esc` | Close expanded view |

### SSM Parameter Store
| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate |
| `/` | Search/filter |
| `enter/space` | Open path prefix or expand parameter |
| `y` | Copy parameter value or path |
| `↑/↓` or `j/k` | Scroll in expanded view |
| `pgup/pgdn` | Page scroll in expanded view |
| `esc/backspace/h` | Go back up one level |
| `esc` | Close expanded view |

### SQS
| Key | Action |
|-----|--------|
| `↑/↓` or `j/k` | Navigate |
| `/` | Search/filter |
| `enter` | Peek messages |
| `space` | Expand queue details |
| `y` | Copy queue URL |
| `enter/space` | Expand message (in messages view) |
| `y` | Copy message body (in messages view) |
| `↑/↓` or `j/k` | Scroll in expanded view |
| `pgup/pgdn` | Page scroll in expanded view |
| `esc/backspace/h` | Go back to queues (from messages) |
| `esc` | Close expanded view |

## Development

```
make test
make run
```

### Adding services

Implement the `awsx.Service` interface, register the service in `cmd/sacha/main.go`, and provide a TUI model under `internal/ui/<service>`. Services receive AWS config scoped to the active region/profile.
