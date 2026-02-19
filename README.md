<div align="center">

# sacha

**Keyboard-first AWS TUI for your terminal**

[![CI](https://github.com/Sachamama/sacha/actions/workflows/ci.yml/badge.svg)](https://github.com/Sachamama/sacha/actions/workflows/ci.yml)
[![Release](https://github.com/Sachamama/sacha/actions/workflows/release.yml/badge.svg)](https://github.com/Sachamama/sacha/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/sachamama/sacha)](https://goreportcard.com/report/github.com/sachamama/sacha)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://github.com/Sachamama/sacha/blob/main/LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/Sachamama/sacha)](https://go.dev/)
[![Latest Release](https://img.shields.io/github/v/release/Sachamama/sacha?sort=semver)](https://github.com/Sachamama/sacha/releases/latest)

Browse, search, and manage AWS resources directly from your terminal.
No more switching between browser tabs and consoles.

[Install](#install) &bull; [Quick Start](#quick-start) &bull; [Features](#features) &bull; [Keybindings](#keybindings) &bull; [Docs](https://sachamama.github.io/sacha/) &bull; [Contributing](CONTRIBUTING.md)

<a href="https://buymeacoffee.com/sachamama"><img src="https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png" alt="Buy Me A Coffee" height="36"></a>

</div>

<p align="center">
  <img src="demos/demo.gif" alt="sacha demo" width="800">
</p>

---

## Why sacha?

Sacha is a two-pane TUI inspired by classic file managers. It keeps you in the terminal while you explore AWS resources across seven services, with vim-style navigation, lazy-loaded pagination, and automatic JSON formatting.

> **sachamama** comes from Quechua and means "mother of the forest." **sacha** shortens the idea to "forest," reflecting how the tool helps you see the bigger AWS landscape without getting lost in individual trees.

### Supported Services

| Service | What you can do |
|---------|----------------|
| **CloudWatch Logs** | Search, multi-select, and tail multiple log groups. Highlight and filter JSON fields with jq syntax. Create, delete, and set retention policies. |
| **S3** | Browse buckets and objects. Download files and folders recursively. Preview text content. Copy S3 URIs. |
| **DynamoDB** | Browse tables and scan items. View table metadata, key schema, and GSIs. Inspect full item attributes. |
| **Lambda** | Browse functions. View configuration, environment variables, layers, and runtime details. |
| **SSM Parameter Store** | Navigate parameters by path hierarchy. View values with decryption. Supports String, StringList, SecureString. |
| **SQS** | Browse queues with message stats. Peek messages non-destructively. View FIFO/Standard attributes and redrive policies. |
| **EC2** | Browse instances with state, type, IPs, and tags. Expand for full metadata. Filter by name, ID, type, state, or IP. |

---

## Install

**Prerequisites:** AWS credentials configured (via `~/.aws/credentials`, environment variables, or IAM role).

### Homebrew (macOS / Linux)

```bash
brew install sachamama/tap/sacha
```

### Go Install

```bash
go install github.com/sachamama/sacha/cmd/sacha@latest
```

Requires Go 1.22+.

### Pre-built Binaries

Download from [GitHub Releases](https://github.com/Sachamama/sacha/releases):

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

### Update

```bash
# Homebrew
brew upgrade sacha

# Go install
go install github.com/sachamama/sacha/cmd/sacha@latest

# Pre-built binary — download from https://github.com/Sachamama/sacha/releases
# From source
git pull && make build
```

Check your current version: `sacha --version`

---

## Quick Start

```bash
sacha
# or with explicit options
sacha --profile production --region us-east-1 --service cloudwatch-logs
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--profile` | AWS profile to use |
| `--region` | AWS region |
| `--service` | Jump directly to a service (`cloudwatch-logs`, `s3`, `dynamodb`, `lambda`, `ssm`, `sqs`, `ec2`) |
| `--verbose` | Enable debug logging |
| `--version` | Show version information |

### Configuration

Config file: `~/.config/sacha/config.json`

Resolution precedence:
1. CLI flags (`--profile`, `--region`, `--service`)
2. Environment variables (`AWS_PROFILE`, `AWS_REGION`, `AWS_DEFAULT_REGION`)
3. Config file
4. AWS SDK defaults

Sacha remembers your last-used region and service automatically.

---

## Features

### CloudWatch Logs

- **Split-pane TUI** &mdash; left pane lists log groups; right pane shows details or live tail output.
- **Log group management** &mdash; create (`c`), delete (`d`) with confirmation, set retention policy (`R`) with standard presets (1d to 1y or never).
- **Multi-group tailing** &mdash; select multiple groups with `space`/`a` and tail them simultaneously with `t`. Dynamic refresh when selection changes.
- **Compact display** &mdash; log group names show only the last path segment (e.g., `/aws/lambda/my-func` &rarr; `my-func`). Timestamps show the base time for the first event and relative offsets (`+1.5s`, `+2m30s`) for subsequent events.
- **jq-style highlight** &mdash; press `H` while tailing to enter field paths (e.g., `.level .message .statusCode`). Matching JSON values are highlighted in the log output.
- **Filter by highlight** &mdash; press `F` to toggle filtering: only events containing the highlighted fields are shown. Press `F` again to show all events.
- **Panel focus** &mdash; `tab`/`h`/`l` to switch between groups and tail panels. Focused panel highlighted with colored border.
- **Fullscreen mode** &mdash; press `f` while tailing for a distraction-free view with horizontal scrolling.
- **Expand events** &mdash; `enter`/`space` to open a scrollable popup with pretty-printed JSON.

### S3

- **Bucket and object browser** &mdash; navigate into buckets and folders with `enter`, go back with `esc`/`backspace`.
- **Batch operations** &mdash; multi-select files and folders with `space`, select all with `a`.
- **Recursive downloads** &mdash; press `d` to download selected files and folders to `./sacha-downloads/`, preserving folder structure.
- **Text preview** &mdash; press `p` to preview file contents inline.
- **Clipboard** &mdash; `y` to copy S3 URI.
- **Lazy pagination** &mdash; loads more objects as you scroll near the bottom.

### DynamoDB

- **Two-pane table browser** &mdash; table list on the left, metadata on the right (status, item count, size, billing, key schema, GSIs).
- **Item scanning** &mdash; press `enter` to scan items with lazy-loaded pagination.
- **Search & filter** &mdash; `/` to filter tables or items by value.
- **Expand items** &mdash; `enter`/`space` to view full attribute details in a scrollable popup.
- **All attribute types** &mdash; strings, numbers, booleans, binary, sets, lists, and maps.
- **Clipboard** &mdash; `y` to copy table ARN.

### Lambda

- **Function browser** &mdash; two-pane view with runtime, handler, memory, timeout, code size, state, and architecture in the details panel.
- **Deep inspection** &mdash; `enter`/`space` to expand and view environment variables, layers, and full configuration.
- **Filter by name or runtime** &mdash; `/` to search.
- **Clipboard** &mdash; `y` to copy function ARN.
- **Lazy pagination** &mdash; loads more functions as you scroll.

### SSM Parameter Store

- **Hierarchical browsing** &mdash; navigate path prefixes like folders with `enter`, go back with `esc`/`backspace`/`h`.
- **Parameter details** &mdash; value (with automatic decryption), type, version, last modified, ARN.
- **All types supported** &mdash; String, StringList, SecureString.
- **Expand popup** &mdash; `enter`/`space` on a parameter for full details.
- **Clipboard** &mdash; `y` to copy parameter value or path.
- **Scroll memory** &mdash; cursor position restored when navigating back.

### SQS

- **Queue browser** &mdash; message counts (visible, in-flight, delayed), queue type, visibility timeout, redrive policy.
- **Non-destructive peek** &mdash; press `enter` to receive messages with visibility timeout 0 &mdash; messages stay in the queue.
- **Message inspection** &mdash; navigate messages, view auto-formatted JSON bodies, expand in a scrollable popup.
- **Queue details popup** &mdash; press `space` to view full queue attributes.
- **Clipboard** &mdash; `y` to copy queue URL or message body.
- **Search/filter** &mdash; `/` to filter queues by name.

### EC2

- **Instance browser** &mdash; two-pane view with name, instance ID, type, state, public/private IPs, and launch time.
- **Deep inspection** &mdash; `enter`/`space` to expand and view full metadata including tags, security groups, VPC, and subnet.
- **Filter** &mdash; `/` to search by name, instance ID, type, state, or IP address.
- **Clipboard** &mdash; `y` to copy instance ID.
- **Lazy pagination** &mdash; loads more instances as you scroll.

---

## Keybindings

### Global

| Key | Action |
|-----|--------|
| `r` | Change region |
| `s` | Change service |
| `Ctrl+C` | Quit |

### CloudWatch Logs

| Key | Context | Action |
|-----|---------|--------|
| `j/k` or `↑/↓` | List | Navigate log groups |
| `/` | List | Search / filter |
| `space` | List | Toggle selection |
| `a` | List | Select all |
| `c` | List | Create log group |
| `d` | List | Delete selected log groups |
| `R` | List | Set retention policy |
| `t` | List | Start tailing selected groups |
| `tab`, `h/l` | Tailing | Switch panel focus |
| `enter` / `space` | Tailing | Expand log event |
| `H` | Tailing | Set highlight fields (jq syntax: `.level .message`) |
| `F` | Tailing | Toggle filter by highlighted fields |
| `f` | Tailing | Toggle fullscreen |
| `h/l` or `←/→` | Fullscreen | Scroll horizontally |
| `x` or `q/esc` | Tailing | Stop tailing |
| `j/k` or `↑/↓` | Popup | Scroll content |
| `pgup/pgdn` | Popup | Page scroll |
| `esc` | Popup | Close |

### S3

| Key | Context | Action |
|-----|---------|--------|
| `j/k` or `↑/↓` | Browse | Navigate |
| `/` | Browse | Search / filter |
| `enter` | Browse | Open bucket / folder |
| `space` | Browse | Toggle selection |
| `a` | Browse | Toggle all (current page) |
| `d` | Browse | Download selected (recursive) |
| `p` | Browse | Preview text file |
| `y` | Browse | Copy S3 URI |
| `esc/backspace` | Browse | Go back |

### DynamoDB

| Key | Context | Action |
|-----|---------|--------|
| `j/k` or `↑/↓` | Tables | Navigate |
| `/` | Tables | Search / filter |
| `enter` | Tables | Open table (scan items) |
| `y` | Tables | Copy table ARN |
| `enter/space` | Items | Expand item details |
| `esc/backspace/h` | Items | Go back to tables |
| `j/k` or `↑/↓` | Popup | Scroll content |
| `pgup/pgdn` | Popup | Page scroll |
| `esc` | Popup | Close |

### Lambda

| Key | Context | Action |
|-----|---------|--------|
| `j/k` or `↑/↓` | List | Navigate |
| `/` | List | Search / filter by name or runtime |
| `enter/space` | List | Expand function details |
| `y` | List | Copy function ARN |
| `j/k` or `↑/↓` | Popup | Scroll content |
| `pgup/pgdn` | Popup | Page scroll |
| `esc` | Popup | Close |

### SSM Parameter Store

| Key | Context | Action |
|-----|---------|--------|
| `j/k` or `↑/↓` | Browse | Navigate |
| `/` | Browse | Search / filter |
| `enter/space` | Browse | Enter path prefix or expand parameter |
| `y` | Browse | Copy parameter value or path |
| `esc/backspace/h` | Browse | Go back one level |
| `j/k` or `↑/↓` | Popup | Scroll content |
| `pgup/pgdn` | Popup | Page scroll |
| `esc` | Popup | Close |

### SQS

| Key | Context | Action |
|-----|---------|--------|
| `j/k` or `↑/↓` | Queues | Navigate |
| `/` | Queues | Search / filter |
| `enter` | Queues | Peek messages |
| `space` | Queues | Expand queue details |
| `y` | Queues | Copy queue URL |
| `enter/space` | Messages | Expand message |
| `y` | Messages | Copy message body |
| `esc/backspace/h` | Messages | Go back to queues |
| `j/k` or `↑/↓` | Popup | Scroll content |
| `pgup/pgdn` | Popup | Page scroll |
| `esc` | Popup | Close |

### EC2

| Key | Context | Action |
|-----|---------|--------|
| `j/k` or `↑/↓` | List | Navigate |
| `/` | List | Search / filter by name, ID, type, state, or IP |
| `enter/space` | List | Expand instance details |
| `y` | List | Copy instance ID |
| `j/k` or `↑/↓` | Popup | Scroll content |
| `pgup/pgdn` | Popup | Page scroll |
| `esc` | Popup | Close |

---

## Architecture

Sacha uses a layered architecture built on [Bubble Tea](https://github.com/charmbracelet/bubbletea):

```
cmd/sacha/main.go          CLI entry point (Cobra)
internal/config/            Configuration (CLI > env > file > defaults)
internal/aws/               AWS SDK v2 abstraction + Service interface
internal/<service>/          Domain clients and types per service
internal/ui/<service>/       Bubble Tea models and views per service
internal/ui/app/             Main app shell (region/service switching)
```

Each service implements the `awsx.Service` interface and is registered in `main.go`. See the [Contributing Guide](CONTRIBUTING.md) for how to add a new service.

---

## Versioning

Sacha follows [semantic versioning](https://semver.org/). Releases are tagged `vMAJOR.MINOR.PATCH` and automatically built via [GoReleaser](https://goreleaser.com/):

- Pre-built binaries for Linux, macOS, Windows (amd64 + arm64)
- Homebrew tap updates
- SHA256 checksums
- Auto-generated changelogs

Browse all releases: [github.com/Sachamama/sacha/releases](https://github.com/Sachamama/sacha/releases)

---

## Contributing

Contributions are welcome! Please read the [Contributing Guide](CONTRIBUTING.md) for details on the development workflow, coding standards, commit conventions, and how to add new AWS services.

---

## License

[MIT](LICENSE) &copy; [Sachamama](https://github.com/Sachamama)
