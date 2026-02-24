#!/bin/bash
# Backup OpenWrt configuration from Cudy WR3000S v1 (5th Floor)
# Usage: ./backup.sh [router_ip] [ssh_user]
#        JUMP_HOST=admin@cyg.local ./backup.sh
#
# Creates a timestamped backup under backups/
# Requires SSH key-based access to the router.
# Set JUMP_HOST env var for ProxyJump (e.g., when Pi is the gateway).

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKUP_BASE="$SCRIPT_DIR/backups"

ROUTER_IP="${1:-192.168.10.2}"
SSH_USER="${2:-root}"
JUMP_HOST="${JUMP_HOST:-}"

SSH_OPTS=(-o ConnectTimeout=10 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR)
[[ -n "$JUMP_HOST" ]] && SSH_OPTS+=(-o "ProxyCommand=ssh -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -W %h:%p $JUMP_HOST")

# UCI config files to back up (core routing/firewall/wireless)
UCI_CONFIGS=(
    network
    wireless
    dhcp
    firewall
    system
    dropbear
    luci
    uhttpd
    rpcd
)

# Additional files to capture for reference
EXTRA_FILES=(
    /etc/openwrt_release
    /etc/openwrt_version
    /etc/hosts
    /etc/passwd
    /etc/dropbear/authorized_keys
)

# --- Functions ---

log() { echo "[backup] $*"; }
die() { echo "[backup] ERROR: $*" >&2; exit 1; }

ssh_cmd() {
    ssh "${SSH_OPTS[@]}" "${SSH_USER}@${ROUTER_IP}" "$@"
}

check_connectivity() {
    log "Checking SSH connectivity to ${SSH_USER}@${ROUTER_IP}..."
    ssh_cmd 'echo ok' >/dev/null 2>&1 || die "Cannot SSH to ${ROUTER_IP}. Check connectivity and SSH keys."
}

get_router_info() {
    ssh_cmd 'cat /etc/openwrt_release 2>/dev/null; echo "---"; uname -a'
}

# --- Main ---

check_connectivity

TIMESTAMP="$(date +%Y%m%d-%H%M%S)"
BACKUP_DIR="$BACKUP_BASE/$TIMESTAMP"

mkdir -p "$BACKUP_DIR/etc/config"
mkdir -p "$BACKUP_DIR/etc/dropbear"

log "Backup target: $BACKUP_DIR"
log "Timestamp: $TIMESTAMP"

# 1. Save router metadata
log "Saving router metadata..."
{
    echo "# Backup: $TIMESTAMP"
    echo "# Router: ${SSH_USER}@${ROUTER_IP}"
    echo ""
    get_router_info
} > "$BACKUP_DIR/metadata.txt"

# 2. Back up UCI config files
log "Backing up UCI configs..."
for cfg in "${UCI_CONFIGS[@]}"; do
    if ssh_cmd "test -f /etc/config/$cfg" 2>/dev/null; then
        ssh_cmd "cat /etc/config/$cfg" > "$BACKUP_DIR/etc/config/$cfg"
        log "  /etc/config/$cfg -> OK"
    else
        log "  /etc/config/$cfg -> SKIPPED (not found)"
    fi
done

# 3. Back up extra files
log "Backing up extra files..."
for f in "${EXTRA_FILES[@]}"; do
    local_path="$BACKUP_DIR$f"
    mkdir -p "$(dirname "$local_path")"
    if ssh_cmd "test -f $f" 2>/dev/null; then
        ssh_cmd "cat $f" > "$local_path"
        log "  $f -> OK"
    else
        log "  $f -> SKIPPED (not found)"
    fi
done

# 4. Save installed packages list
log "Saving installed packages list..."
ssh_cmd 'opkg list-installed' > "$BACKUP_DIR/packages.txt" 2>/dev/null || log "  WARN: could not list packages"

# 5. Save full UCI export (machine-readable, for reference)
log "Saving full UCI export..."
ssh_cmd 'uci export' > "$BACKUP_DIR/uci-export.txt" 2>/dev/null || log "  WARN: could not export UCI"

# 6. Save network state snapshot
log "Saving network state snapshot..."
{
    echo "=== ip addr ==="
    ssh_cmd 'ip addr'
    echo ""
    echo "=== ip route ==="
    ssh_cmd 'ip route'
    echo ""
    echo "=== bridge vlan show ==="
    ssh_cmd 'bridge vlan show' 2>/dev/null || true
    echo ""
    echo "=== iwinfo ==="
    ssh_cmd 'for iface in $(iwinfo | grep ESSID | awk "{print \$1}"); do iwinfo $iface info; echo; done' 2>/dev/null || true
    echo ""
    echo "=== ifstatus wan ==="
    ssh_cmd 'ifstatus wan' 2>/dev/null || true
} > "$BACKUP_DIR/network-state.txt" 2>/dev/null

# 7. Generate sysupgrade backup (OpenWrt native tar.gz)
log "Generating sysupgrade backup..."
ssh_cmd 'sysupgrade -b - 2>/dev/null' > "$BACKUP_DIR/sysupgrade-backup.tar.gz" || log "  WARN: sysupgrade backup failed"

# Summary
FILE_COUNT=$(find "$BACKUP_DIR" -type f | wc -l | tr -d ' ')
TOTAL_SIZE=$(du -sh "$BACKUP_DIR" | awk '{print $1}')
log ""
log "=== Backup Complete ==="
log "  Directory: $BACKUP_DIR"
log "  Files:     $FILE_COUNT"
log "  Size:      $TOTAL_SIZE"
log ""
log "To restore, run: ./restore.sh $TIMESTAMP"
