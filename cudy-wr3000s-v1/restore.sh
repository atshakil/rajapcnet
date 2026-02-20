#!/bin/bash
# Restore OpenWrt configuration to Cudy WR3000S v1
# Usage: ./restore.sh <timestamp> [router_ip] [ssh_user]
#
# Restores UCI config files from a timestamped backup.
# Requires SSH key-based access to the router.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BACKUP_BASE="$SCRIPT_DIR/backups"

TIMESTAMP="${1:-}"
ROUTER_IP="${2:-192.168.1.1}"
SSH_USER="${3:-root}"
SSH_OPTS="-o ConnectTimeout=5 -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o LogLevel=ERROR"

# UCI configs that require a network restart after restore
NETWORK_CONFIGS=(network wireless dhcp firewall)

# --- Functions ---

log() { echo "[restore] $*"; }
die() { echo "[restore] ERROR: $*" >&2; exit 1; }

ssh_cmd() {
    ssh $SSH_OPTS "${SSH_USER}@${ROUTER_IP}" "$@"
}

# --- Validation ---

if [[ -z "$TIMESTAMP" ]]; then
    echo "Usage: $0 <timestamp> [router_ip] [ssh_user]"
    echo ""
    echo "Available backups:"
    if [[ -d "$BACKUP_BASE" ]]; then
        ls -1 "$BACKUP_BASE" | sort -r | head -20
    else
        echo "  (none)"
    fi
    exit 1
fi

BACKUP_DIR="$BACKUP_BASE/$TIMESTAMP"
[[ -d "$BACKUP_DIR" ]] || die "Backup not found: $BACKUP_DIR"
[[ -d "$BACKUP_DIR/etc/config" ]] || die "No config directory in backup: $BACKUP_DIR/etc/config"

# --- Connectivity check ---

log "Checking SSH connectivity to ${SSH_USER}@${ROUTER_IP}..."
ssh_cmd 'echo ok' >/dev/null 2>&1 || die "Cannot SSH to ${ROUTER_IP}. Check connectivity and SSH keys."

# --- Show what will be restored ---

log ""
log "=== Restore Plan ==="
log "  Backup:  $TIMESTAMP"
log "  Router:  ${SSH_USER}@${ROUTER_IP}"
log "  Configs:"
for cfg_file in "$BACKUP_DIR"/etc/config/*; do
    [[ -f "$cfg_file" ]] || continue
    cfg_name="$(basename "$cfg_file")"
    log "    /etc/config/$cfg_name"
done
log ""

# --- Confirmation ---

read -rp "[restore] Proceed with restore? This will overwrite router config. [y/N] " confirm
[[ "$confirm" =~ ^[Yy]$ ]] || { log "Aborted."; exit 0; }

# --- Pre-restore: backup current config on router ---

log "Creating pre-restore backup on router..."
RESTORE_TS="$(date +%Y%m%d%H%M%S)"
ssh_cmd "mkdir -p /etc/config/pre-restore-$RESTORE_TS"
ssh_cmd "cp /etc/config/network /etc/config/wireless /etc/config/dhcp /etc/config/firewall /etc/config/system /etc/config/pre-restore-$RESTORE_TS/ 2>/dev/null || true"
log "  Saved to /etc/config/pre-restore-$RESTORE_TS/"

# --- Restore UCI configs ---

log "Restoring UCI configs..."
for cfg_file in "$BACKUP_DIR"/etc/config/*; do
    [[ -f "$cfg_file" ]] || continue
    cfg_name="$(basename "$cfg_file")"
    cat "$cfg_file" | ssh_cmd "cat > /etc/config/$cfg_name"
    log "  /etc/config/$cfg_name -> restored"
done

# --- Restore SSH authorized keys if present ---

if [[ -f "$BACKUP_DIR/etc/dropbear/authorized_keys" ]]; then
    log "Restoring SSH authorized keys..."
    cat "$BACKUP_DIR/etc/dropbear/authorized_keys" | ssh_cmd "cat > /etc/dropbear/authorized_keys && chmod 600 /etc/dropbear/authorized_keys"
fi

# --- Apply changes ---

log ""
read -rp "[restore] Restart network and apply changes now? [y/N] " apply
if [[ "$apply" =~ ^[Yy]$ ]]; then
    log "Reloading services..."
    ssh_cmd '/etc/init.d/network restart; /etc/init.d/firewall restart; /etc/init.d/dnsmasq restart' 2>/dev/null || true
    log "Services restarted. You may lose connectivity briefly."
    log ""
    log "If connectivity is lost, wait 60 seconds then try again."
    log "Router-side rollback: ssh root@$ROUTER_IP 'cp /etc/config/pre-restore-$RESTORE_TS/* /etc/config/ && /etc/init.d/network restart'"
else
    log "Configs written but NOT applied. Run on the router:"
    log "  /etc/init.d/network restart && /etc/init.d/firewall restart && /etc/init.d/dnsmasq restart"
fi

log ""
log "=== Restore Complete ==="
