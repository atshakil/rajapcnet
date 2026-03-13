# NVR

Lightweight camera streaming and recording manager. Built in Go with SQLite, designed for ONVIF-capable IP cameras.

## Requirements

- Go 1.24+
- ffmpeg (on target host)
- fswatch (on dev machine, `brew install fswatch`)

## Setup

1. Copy `.env.client.example` to `.env.client` and configure your target host and paths.
2. Copy `.env.host.example` to `.env.host` for the target's runtime config.
3. Run `make setup-pi` once to prepare the target.

## Development

```bash
# Single command: build, deploy, enable, watch for changes
make dev

# Or individual steps:
make build          # cross-compile daemon + CLI for target, build CLI for local
make deploy         # build + scp to target
make restart        # restart service on target
make stop           # stop service on target
make enable         # start + enable boot persistence
make disable        # stop + disable boot persistence
make run            # run locally

# One-time target setup
make setup-pi
```

## CLI

```bash
# On Mac (uses .env.client → remote daemon)
bin/nvrctl health
bin/nvrctl cameras ls

# On Pi (uses .env.host → localhost daemon)
/opt/nvr/nvrctl cameras status
```

Configuration is via environment variables. See `.env.client.example` and `.env.host.example`.

## API

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/api/health` | Health check |
| GET | `/api/cameras` | List cameras |
| POST | `/api/cameras` | Add camera |
| GET | `/api/cameras/{id}` | Get camera |
| PUT | `/api/cameras/{id}` | Update camera |
| DELETE | `/api/cameras/{id}` | Delete camera |

## Architecture

Single Go binary with embedded SQLite. Cameras are managed dynamically via the API.

See [docs/](docs/) for feature-specific documentation.
