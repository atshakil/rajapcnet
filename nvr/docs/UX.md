# UX — Web Interface

The NVR web UI is a single-page application embedded in [internal/api/web/index.html](../internal/api/web/index.html). No frameworks, no build step — vanilla HTML/CSS/JS served directly by the Go binary.

## Screens

### Login

Centered card (320px) on a dark background. Username + password fields, submit button. Error messages appear in red below the form.

- `POST /api/login` → returns JWT + user object
- Token stored in `localStorage` as `nvr_token`
- On page load, if a token exists the UI silently validates it (GET `/api/cameras`). Valid → straight to app. Invalid → login screen. No flash.

### App Shell

Full-viewport dark theme (`#0f0f0f` background). Three regions:

```
┌──────────────────────────────────────────────┐
│ Topbar: "NVR"   [Grid] [Full]   user (role) │
├──────────────────────────────────────────────┤
│                                              │
│              Video Grid                      │
│                                              │
│   ┌──────────┐  ┌──────────┐                │
│   │  Cam 14   │  │  Cam 15   │                │
│   │           │  │           │                │
│   └──────────┘  └──────────┘                │
│   ┌──────────┐  ┌──────────┐                │
│   │  Cam 16   │  │  Cam 17   │                │
│   │           │  │           │                │
│   └──────────┘  └──────────┘                │
└──────────────────────────────────────────────┘
```

## Grid Layouts

Two modes toggled via topbar buttons:

| Mode | CSS Grid | Use Case |
|------|----------|----------|
| **2×2** (default) | `grid-template-columns: 1fr 1fr` | Overview of all cameras |
| **1×1** | `grid-template-columns: 1fr` | Fullscreen single camera |

- Clicking a camera cell in 2×2 switches to 1×1 showing only that camera.
- Clicking the cell in 1×1 returns to 2×2.
- Grid/Full buttons in the topbar also toggle layout.
- Gap between cells: 4px.

## Camera Cell

Each cell is a self-contained component with layered elements:

```
┌─────────────────────────────────────┐
│ [Rooftop Access]          ● online  │  ← label + status dot
│                                     │
│         (video / snapshot)          │  ← media area
│                                     │
│  ONVIF  PTZ  Motion  IR            │  ← capability badges
│              [Snap] [Primary] [Sub] │  ← mode buttons
└─────────────────────────────────────┘
```

### Elements

| Element | Position | Description |
|---------|----------|-------------|
| **Label** | Top-left | Camera name on translucent black pill |
| **Status dot** | Top-right | 8px circle — green (`#4caf50`) = online, red = offline |
| **Placeholder** | Center | Manufacturer/model or IP, shown when no media active |
| **Media** | Full cell | `<img>` for snapshots, `<video>` for WebRTC. `object-fit: cover` |
| **Bottom bar** | Bottom | Gradient overlay with capability badges and mode buttons |
| **Capability badges** | Bottom-left | Small blue pills: ONVIF, PTZ, Motion, IR, Light |
| **Mode buttons** | Bottom-right | Snap / Primary / Sub — active mode highlighted in blue |

### Media Modes

Each camera has three possible modes. The active mode is persisted per-user per-camera (`PUT /api/cameras/{id}/pref`).

| Mode | Source | Transport | Latency |
|------|--------|-----------|---------|
| **Snapshot** | `GET /api/cameras/{id}/snapshot` | HTTP polling (2s interval) | ~2s |
| **Primary** | `POST /api/cameras/{id}/webrtc?stream=primary` | WebRTC (go2rtc relay) | <1s |
| **Sub** | `POST /api/cameras/{id}/webrtc?stream=sub` | WebRTC (go2rtc relay) | <1s |

Mode selection is remembered across sessions via the preferences API.

## WebRTC Flow

When a user selects Primary or Sub mode:

```
Browser                       NVR (:8080)                go2rtc (:1984)           Camera
   │                              │                           │                      │
   │  1. createOffer (SDP)        │                           │                      │
   │  2. gather ICE candidates    │                           │                      │
   │  3. POST /api/.../webrtc ──►│                           │                      │
   │     (SDP offer)              │  4. POST /api/webrtc ───►│                      │
   │                              │     (register stream)     │  5. RTSP SETUP ────►│
   │                              │                           │  6. RTP stream ◄────│
   │  7. SDP answer ◄────────────│◄── SDP answer ───────────│                      │
   │                              │                           │                      │
   │  8. WebRTC media ◄──────────────────────────────────────│                      │
   │     (SRTP over UDP)          │                           │                      │
```

- ICE gathering: up to 3s timeout, then sends whatever candidates are available.
- STUN server: `stun:stun.l.google.com:19302`
- On-demand: go2rtc only pulls from the camera when a viewer connects.

## H.265 Codec Handling

If go2rtc returns a codec mismatch (camera streams H.265, browsers need H.264), the UI shows an overlay instead of silently failing:

```
┌─────────────────────────────────────┐
│                                     │
│       ⚠ Camera uses H265           │  ← orange title
│                                     │
│  WebRTC requires H.264. Use the    │
│  button below to switch the        │
│  camera's encoder via ONVIF.       │
│                                     │
│      [ Fix: Switch to H.264 ]      │  ← blue button
│      [ Keep Snapshot ]              │  ← muted fallback
│                                     │
└─────────────────────────────────────┘
```

- **Fix button**: `POST /api/cameras/{id}/set-h264` → calls ONVIF `SetVideoEncoderConfiguration` (Media 1.x and 2.0). On success, retries WebRTC after 2s.
- **Keep Snapshot**: Dismisses overlay and falls back to snapshot mode.
- Button shows "Switching…" while in flight and is disabled to prevent double-click.

## Authentication & Authorization

| Concept | Detail |
|---------|--------|
| **Token** | JWT (HS256), 24h expiry |
| **Storage** | `localStorage.nvr_token` |
| **Header** | `Authorization: Bearer <token>` on all API calls |
| **Roles** | `admin` (full access), `viewer` (cameras + prefs only) |
| **Session restore** | On load, validates stored token silently. No login flash. |
| **Logout** | Clears token, hides app, shows login screen |
| **Bootstrap** | `POST /api/bootstrap` creates the first admin user (one-time, disabled after) |

## API Surface (UI-relevant)

| Method | Endpoint | Purpose |
|--------|----------|---------|
| POST | `/api/login` | Authenticate → `{token, user}` |
| POST | `/api/bootstrap` | Create first admin (one-time) |
| GET | `/api/cameras` | List all cameras |
| POST | `/api/cameras` | Add camera (nvrctl/API only) |
| GET | `/api/cameras/{id}` | Get single camera |
| PUT | `/api/cameras/{id}` | Update camera |
| DELETE | `/api/cameras/{id}` | Delete camera |
| GET | `/api/cameras/{id}/snapshot` | Fetch JPEG snapshot |
| POST | `/api/cameras/{id}/webrtc` | WebRTC SDP exchange |
| POST | `/api/cameras/{id}/set-h264` | Switch encoder to H.264 |
| GET | `/api/prefs` | Get user's stream mode prefs |
| PUT | `/api/cameras/{id}/pref` | Set stream mode pref |
| GET | `/api/users` | List users (admin) |
| POST | `/api/users` | Create user (admin) |
| GET | `/api/users/{id}` | Get user (admin) |
| PUT | `/api/users/{id}` | Update user (admin) |
| DELETE | `/api/users/{id}` | Delete user (admin) |

## Color Scheme

Dark theme with CSS custom properties:

| Variable | Value | Use |
|----------|-------|-----|
| `--bg` | `#0f0f0f` | Page background |
| `--surface` | `#1a1a1a` | Cards, cells, login box |
| `--border` | `#2a2a2a` | Dividers, cell borders |
| `--text` | `#e0e0e0` | Primary text |
| `--muted` | `#888` | Secondary text, placeholders |
| `--accent` | `#4a9eff` | Buttons, active states, badges |
| `--danger` | `#ff4a4a` | Logout, errors, offline indicator |

## State Management

All state is in-memory JS variables:

| Variable | Type | Description |
|----------|------|-------------|
| `token` | string | JWT from localStorage |
| `user` | object | `{username, role}` from login response |
| `cameras` | array | Camera objects from API |
| `camPrefs` | object | `{cameraId: "snapshot"|"primary"|"sub"}` |
| `gridMode` | string | `"2x2"` or `"1x1"` |
| `selectedCam` | number | Camera ID for 1×1 view, or null |

## Cleanup

Each camera cell stores a `_cleanup` function that is called when re-rendering the grid. It:

1. Clears the snapshot polling interval
2. Revokes any blob URLs (prevents memory leaks)
3. Closes the RTCPeerConnection
4. Removes any codec overlay from the DOM

## Limitations

- **No camera management in UI** — cameras are added/removed via `nvrctl` CLI or raw API calls
- **No recording controls** — recording is managed server-side
- **No PTZ controls** — PTZ-capable cameras are detected but no UI for control
- **No multi-page** — single HTML file, no routing
- **No mobile optimization** — responsive grid but no touch gestures or mobile-specific layout
- **2×2 grid fixed** — no 3×3 or custom layouts
