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
| DELETE | `/api/cameras/{id}` | Delete camera (cascades motion data) |
| GET | `/api/cameras/{id}/snapshot` | Fetch JPEG snapshot |
| POST | `/api/cameras/{id}/webrtc` | WebRTC SDP exchange |
| POST | `/api/cameras/{id}/set-h264` | Switch camera encoder to H.264 |
| PUT | `/api/cameras/{id}/pref` | Set stream mode preference |
| GET | `/api/cameras/{id}/motion-log` | Get motion settings for camera |
| PUT | `/api/cameras/{id}/motion-log` | Enable/disable motion logging (admin) |
| GET | `/api/cameras/{id}/motion-log/events` | List motion episodes |
| GET | `/api/cameras/{id}/motion-log/stream` | SSE stream of motion events |
| GET | `/api/motion-log/events` | List all motion episodes |
| GET | `/api/motion-log/stream` | SSE stream of all motion events |
| GET | `/api/motion-log/status` | Motion worker runtime status |

## Architecture

### Overview

Single Go binary (`nvr`) with embedded SQLite, cross-compiled for `linux/arm64`. Cameras are discovered via ONVIF and streamed via go2rtc (WebRTC/RTSP relay). No transcoding — streams are passed through as-is.

See [docs/](docs/) for feature-specific documentation.

### Hardware

| Component | Detail |
|-----------|--------|
| **Host** | Raspberry Pi 3 Model B+ (BCM2837B0, 4× Cortex-A53 @ 1.4 GHz, 1 GB RAM) |
| **System disk** | 512 GB KIOXIA EXCERIA SATA SSD (USB, mounted at `/`) |
| **Recording storage** | 2 TB Seagate ST2000VX017 SkyHawk HDD (USB, mounted at `/mnt/nvr`) |
| **Database** | SQLite (`/opt/nvr/nvr.db`, ~28 KB) — camera config, users, metadata |
| **Swap** | 955 MB zram (compressed RAM) |

### Software Stack

| Component | Version | Role |
|-----------|---------|------|
| **nvr** | Go 1.25, `linux/arm64` | API server, camera management, ONVIF probe, recording orchestration |
| **go2rtc** | v1.9.14 | RTSP→WebRTC relay, on-demand stream proxy |
| **ffmpeg** | 7.1.3 | Recording segmentation (RTSP→MP4) |
| **sqlite3** | 3.46.1 | CLI for database inspection (`apt install sqlite3`) |
| **OS** | Debian 13 (trixie), kernel 6.12.62+rpt-rpi-v8 | Raspberry Pi OS headless |
| **Tailscale** | — | Remote access mesh VPN |

### Network

```
Cameras (VLAN 20, 192.168.20.0/24)          Pi (VLAN 10, 192.168.10.250)
  ┌─ 192.168.20.134 Dahua   ─┐               ┌──────────────────────┐
  ├─ 192.168.20.162 Hikvision─┤  inter-VLAN   │ nvr        :8080     │  Tailscale
  ├─ 192.168.20.179 Hikvision─├──(RouterF2)──►│ go2rtc API :1984     │◄──────────► Clients
  └─ 192.168.20.198 Reolink  ─┘    routing    │ go2rtc RTSP:8554     │
                                               │ go2rtc RTC :8555     │
                                               └──────────────────────┘
```

- Cameras on **VLAN 20** (IoT/cameras) with static DHCP leases on RouterF2
- Pi on **VLAN 10** (IoT) — reaches cameras via inter-VLAN routing on RouterF2
- Remote access via **Tailscale** (`cyg.finch-algol.ts.net`)
- Pi has dual NICs: `wlan0` (192.168.10.250, metric 600) and `eth0` (192.168.10.251, metric 100)

### Service Architecture

```
Browser ──WebRTC──► go2rtc ◄──RTSP──► Camera
                      ▲
                      │ HTTP API (localhost:1984)
                      │
  Browser ──REST──► nvr (localhost:8080)
  Browser ◄──SSE───┘
                      │
                      ├── SQLite (camera DB, users, motion settings+episodes)
                      ├── ONVIF SOAP (probe, codec switching)
                      ├── ONVIF PullPoint (motion event subscriptions)
                      ├── Motion Manager (per-camera workers, episode state machine)
                      └── ffmpeg (recording segments → /mnt/nvr)
```

#### Motion Event Pipeline

```
Camera (ONVIF)        Motion Worker          Store/Hub           Browser
    │                      │                     │                  │
    │  PullMessages (≤10s) │                     │                  │
    │────notifications───►│                     │                  │
    │                      │── normalize topic ─►│                  │
    │                      │   parse active/     │                  │
    │                      │   inactive state     │                  │
    │                      │                     │                  │
    │                      │── open/bump/close ►│ episode (SQLite) │
    │                      │                     │── publish event ►│ SSE
    │                      │                     │                  │
    │  (idle 10s)          │── inferred_close ►│                  │
```

- One worker goroutine per enabled camera, managed by the Motion Manager
- Workers use ONVIF PullPoint subscriptions (long-polling, PT10S message timeout)
- Raw ONVIF events are normalized into episodes: first `active` opens, subsequent bumps count, `inactive` closes
- If no signal for 10s, episode auto-closes with status `inferred_closed`
- Stale episodes from prior crashes are closed on startup with status `interrupted`
- Hourly retention cleanup deletes episodes older than per-camera `retention_days` (batch 500)
- Worker backoff: 2s → 4s → 8s → ... → 60s cap on repeated PullPoint failures

- **go2rtc** streams are on-demand: no CPU/bandwidth until a viewer connects
- **nvr** registers RTSP source URLs with go2rtc at startup; go2rtc pulls from cameras only when a consumer (WebRTC/RTSP client) connects
- **ONVIF auth chain**: SOAP 1.2 plain → HTTP Digest → WS-Security SOAP 1.2 → WS-Security SOAP 1.1 (covers Hikvision, Reolink, Dahua)

### Thermal & Power

| Metric | Idle (0 viewers) | 2 streams | Limit |
|--------|-------------------|-----------|-------|
| CPU | ~0% | ~62% (go2rtc) | 100% (4 cores) |
| Temperature | ~71°C | ~79°C | 80°C (throttle) |
| Load average | ~0.4 | ~1.6 | 4.0 (4 cores) |
| Throttle flags | `0x80000` (prior under-voltage) | — | `0x80008` = active throttle |

- **PSU**: marginal — under-voltage flag has been set (`0x80000`). A 5V/3A supply would eliminate this.
- **Thermal headroom**: ~9°C at idle, ~1°C with 2 concurrent streams. No active cooling.

### Bottlenecks

| Resource | Constraint | Impact |
|----------|-----------|--------|
| **CPU** | 4× Cortex-A53 @ 1.4 GHz | WebRTC sessions cost ~15-30% CPU each (SRTP encryption). 2 concurrent primary streams approach thermal throttle. |
| **Thermal** | Passive cooling, 80°C throttle | Hard ceiling on concurrent streams. 3+ simultaneous primary viewers will throttle. |
| **USB bus** | Shared USB 2.0 for SSD + HDD | Both disks share 480 Mbps — concurrent recording + reads could bottleneck I/O. |
| **WiFi** | 2.4/5 GHz single-stream | Camera RTSP ingress competes with Tailscale egress on the same radio. Ethernet preferred. |
| **RAM** | 955 MB total | Tailscale (87 MB) + go2rtc (33 MB) + nvr (32 MB) = ~152 MB baseline. 679 MB free — not a bottleneck. |
| **Under-voltage** | PSU < 5V/3A | Can cause CPU throttling independently of temperature. |

### Scaling Notes

- **Sub-streams** (640×480) use significantly less CPU per WebRTC session — prefer for multi-viewer
- **go2rtc** does not transcode — H.265 cameras must be switched to H.264 via ONVIF for WebRTC compatibility (`POST /api/cameras/{id}/set-h264`)
- Adding more cameras has negligible idle cost (just DB rows + go2rtc stream definitions); the constraint is concurrent viewers
