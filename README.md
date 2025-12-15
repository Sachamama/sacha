# sacha

<a href="https://buymeacoffee.com/sachamama"><img src="https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png" alt="Buy Me A Coffee" height="36"></a>

sacha is a keyboard-first AWS TUI inspired by classic two-pane file managers. It keeps you in the terminal while you browse, search, and tail CloudWatch Logs without bouncing between consoles. The first release focuses on CloudWatch Logs with an extensible architecture for more AWS services.

## Highlights
- Two-pane TUI for fast CloudWatch Logs exploration.
- Search, multi-select, and tail multiple log groups at once.
- Remembers your last region/service and plays nicely with AWS profiles.
- Minimal dependencies; install and run with a single command.

## What’s in a name?
- `sachamama` comes from Quechua and means “mother of the forest,” which is also the username of the author.
- `sacha` shortens the idea to “forest,” reflecting how the tool helps you see the bigger AWS landscape without getting lost in individual trees.

## Install

Prerequisites: Go 1.22+ and AWS credentials that can read CloudWatch Logs.

- With Go: `go install github.com/sachamama/sacha/cmd/sacha@latest`
- From source: `make build` (binary at `bin/sacha`)

## Quickstart

```
make run
# or directly after install
sacha --profile my-aws-profile --region us-east-1
```

Global flags:
- `--profile` – AWS profile to use
- `--region` – AWS region
- `--service` – AWS service (currently only `cloudwatch-logs`)
- `--verbose` – enable debug logging

Configuration lives under the OS config directory (e.g. `~/.config/sacha/config.json`) and stores defaults plus your last used region/service. Precedence: CLI flags > env (`AWS_PROFILE`, `AWS_REGION`, `AWS_DEFAULT_REGION`) > config file > AWS SDK defaults.

## Current features (v0.1 – CloudWatch Logs)
- Split-pane TUI: left pane lists log groups; right pane tails logs.
- Log group list with search (`/`), cursor navigation (arrows or `j`/`k`), space to toggle selection, `a` to select all.
- Start tailing selected log groups with `t`; combined stream shows timestamp, group, and message.
- Region switch with `r`; service switch scaffold with `s` (CloudWatch Logs available today).
- Help overlay with `?`; quit with `q` or `Ctrl+C`.

## Keybindings
- Navigation: arrows / `j` `k`
- Search: `/`
- Select: `space` (toggle), `a` (select all)
- Tail: `t` (start), `q`/`Esc` while tailing to stop
- Region: `r`
- Service: `s`
- Help: `?`
- Quit: `Ctrl+C`

## Development

```
make test
make run
```

### Adding services

Implement the `awsx.Service` interface, register the service in `cmd/sacha/main.go`, and provide a TUI model under `internal/ui/<service>`. Services receive AWS config scoped to the active region/profile.
