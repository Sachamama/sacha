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

### `sacha-quick.tape` — Quick Showcase (6-10s)

Fast tour through all 4 services. Best for social media and README.

```bash
cd demos && vhs sacha-quick.tape
```

### `sacha-full.tape` — Full Demo (20-25s)

Deeper feature exploration: log tailing, S3 browsing, DynamoDB scanning, Lambda filtering.

Requires live log events for the tailing section:

```bash
# Terminal 1: start live events
make local-seed-live

# Terminal 2: record
cd demos && vhs sacha-full.tape
```

## Output

Recordings are saved as `.mp4` files in this directory (gitignored).

## Customization

Edit the `.tape` files to adjust:
- `Set FontSize` — Text size (default: 16)
- `Set Theme` — Color scheme (default: Catppuccin Mocha)
- `Sleep` durations — Pacing between actions
- `TypingSpeed` — How fast commands are typed

See [VHS documentation](https://github.com/charmbracelet/vhs) for all options.
