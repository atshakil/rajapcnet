# NVR

Lightweight camera streaming and recording manager. Built in Go with SQLite, designed for ONVIF-capable IP cameras.

## Requirements

- Go 1.24+
- ffmpeg (on target host)
- fswatch (on dev machine, `brew install fswatch`)

## Setup

1. Copy `.env.example` to `.env` and configure your target host and paths.
2. Run `make setup-pi` once to prepare the target.

## Development

```bash
# Single command: build, deploy, restart, watch for changes
make dev

# Or individual steps:
make build          # cross-compile for target
make deploy         # build + scp to target
make restart        # restart service on target
make run            # run locally

# One-time target setup
make setup-pi
```

Configuration is via environment variables. See `.env.example` for available options.

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
