# sacha

sacha is a keyboard-first AWS TUI inspired by classic two-pane file managers. The first release focuses on CloudWatch Logs with an extensible architecture for more AWS services.

## Install

- With Go: `go install github.com/sachamama/sacha/cmd/sacha@latest`
- From source: `make build` (binary at `bin/sacha`)

## Run

```
make run
# or directly
sacha --profile my-aws-profile --region us-east-1
```

Global flags:
- `--profile` – AWS profile to use
- `--region` – AWS region
- `--service` – AWS service (currently only `cloudwatch-logs`)
- `--verbose` – enable debug logging

Configuration is stored at the OS config directory (e.g. `~/.config/sacha/config.json`) and records defaults plus last used region/service. Precedence: CLI flags > env (`AWS_PROFILE`, `AWS_REGION`, `AWS_DEFAULT_REGION`) > config file > AWS SDK defaults.

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
