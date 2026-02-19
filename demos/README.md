# Demo Videos

VHS tape files for recording Sacha demo videos.

## Prerequisites

1. **VHS** - Terminal recorder: `brew install vhs`
2. **LocalStack** - Local AWS: `docker compose up -d`
3. **Seed data**: `make local-seed`
4. **Build**: `make build`

## Quick Setup

```bash
# From project root
make local-up && make local-seed && make build
```

## Available Demos

### `demo.tape` — Full Demo

Tours through all services: log tailing, S3 browsing, DynamoDB scanning, and more.

Requires live log events for the tailing section:

```bash
# Terminal 1: start live events
make local-seed-live

# Terminal 2: record
cd demos && vhs demo.tape
```

### Starter Template

Minimal tape with shell, output, dimensions, and LocalStack environment pre-configured. Use as a starting point for new recordings:

```tape
Set Shell zsh
Output demo.gif
Set Width 1900
Set Height 800
Env AWS_ENDPOINT_URL "http://localhost:4566"
Env AWS_ACCESS_KEY_ID "test"
Env AWS_SECRET_ACCESS_KEY "test"
Env AWS_REGION "us-east-1"
```

Copy and extend with your own VHS commands.

## Environment variables

```bash
export AWS_ENDPOINT_URL="http://localhost:4566"
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_REGION=us-east-1
```

## Output

Recordings are saved as `.gif` files in this directory (gitignored).

## Customization

Edit the `.tape` files to adjust:
- `Set FontSize` — Text size (default: 16)
- `Set Theme` — Color scheme (default: Catppuccin Mocha)
- `Sleep` durations — Pacing between actions
- `TypingSpeed` — How fast commands are typed

See [VHS documentation](https://github.com/charmbracelet/vhs) for all options.
