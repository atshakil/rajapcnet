#!/usr/bin/env bash
# setup-go2rtc.sh — install go2rtc on a Raspberry Pi 3B+ (aarch64 / arm64)
# Run as root (or via sudo).
# Usage: sudo /opt/nvr/setup-go2rtc.sh

set -euo pipefail

INSTALL_DIR="/opt/nvr"
BINARY="${INSTALL_DIR}/go2rtc"
CONFIG="${INSTALL_DIR}/go2rtc.yaml"
SERVICE_SRC="${INSTALL_DIR}/go2rtc.service"
SERVICE_DEST="/etc/systemd/system/go2rtc.service"

# ---------------------------------------------------------------------------
# Detect latest go2rtc release
LATEST=$(curl -fsSL "https://api.github.com/repos/AlexxIT/go2rtc/releases/latest" \
  | python3 -c "import sys,json; print(json.load(sys.stdin)['tag_name'])")

echo "Installing go2rtc ${LATEST} for linux/arm64 in ${INSTALL_DIR}"

TMPFILE=$(mktemp)
curl -fsSL "https://github.com/AlexxIT/go2rtc/releases/download/${LATEST}/go2rtc_linux_arm64" \
  -o "${TMPFILE}"

install -m 0755 "${TMPFILE}" "${BINARY}"
rm -f "${TMPFILE}"
chown admin:admin "${BINARY}"

echo "go2rtc binary installed at ${BINARY}"

# ---------------------------------------------------------------------------
# Write a minimal config if none exists
if [[ ! -f "${CONFIG}" ]]; then
  cat > "${CONFIG}" <<'YAML'
# go2rtc config — streams are managed dynamically by the NVR daemon via API.
# Manual additions here are also supported.
api:
  listen: :1984

webrtc:
  # ICE servers — add a TURN server here for access outside the local network.
  ice_servers:
    - urls: [stun:stun.l.google.com:19302]
YAML
  echo "Config written to ${CONFIG}"
else
  echo "Config already exists at ${CONFIG}, skipping."
fi

# ---------------------------------------------------------------------------
# Install systemd service
if [[ -f "${SERVICE_SRC}" ]]; then
  cp "${SERVICE_SRC}" "${SERVICE_DEST}"
else
  cat > "${SERVICE_DEST}" <<'UNIT'
[Unit]
Description=go2rtc RTSP-to-WebRTC relay
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=admin
ExecStart=/opt/nvr/go2rtc -config /opt/nvr/go2rtc.yaml
WorkingDirectory=/opt/nvr
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
UNIT
fi

systemctl daemon-reload
systemctl enable go2rtc
systemctl restart go2rtc

sleep 2

if systemctl is-active --quiet go2rtc; then
  echo "go2rtc is running."
else
  echo "WARNING: go2rtc did not start. Check: journalctl -u go2rtc -n 30"
fi
