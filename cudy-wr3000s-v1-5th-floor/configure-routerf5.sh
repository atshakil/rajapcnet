#!/bin/sh
# RouterF5 VLAN Bridge + AP Configuration Script
# Cudy WR3000S v1 (5th Floor) - OpenWrt 24.10.5
#
# Converts RouterF5 from a standalone router into a VLAN-aware bridge + AP.
# All routing, DHCP, NAT, and firewall are handled by RouterF2 (2nd floor).
#
# VLAN Layout (matching RouterF2):
#   VLAN 10 = Generic WiFi/LAN  (192.168.10.0/24) - DHCP from RouterF2
#   VLAN 20 = IoT devices        (192.168.20.0/24) - DHCP from RouterF2
#   VLAN 51 = Starlink WAN       (DHCP from Starlink modem)  - bridged to RouterF2
#
# Port Layout:
#   WAN  = Starlink access (VLAN 51 untagged)
#   LAN1 = Trunk to RouterF2 LAN2 (VLAN 10,20,51 tagged)
#   LAN2 = IoT access port (VLAN 20 untagged)
#   LAN3 = IoT access port (VLAN 20 untagged)
#   LAN4 = IoT access + management (VLAN 20 untagged, VLAN 10 tagged)
#
# Management: 192.168.10.2 on VLAN 10 (reachable via LAN4 with VLAN tagging)
#
# WiFi SSIDs (same as RouterF2):
#   "Md Abdullah"        -> VLAN 10 (lan)
#   "Md Abdullah 5G"     -> VLAN 10 (lan)
#   "Md Abdullah - IoT"  -> VLAN 20 (iot)
#   "Md Abdullah - IoT 5G" -> VLAN 20 (iot)
#
# Usage: scp this to router, then: sh /tmp/configure-routerf5.sh

set -e

echo "============================================="
echo " RouterF5 VLAN Bridge + AP Configuration"
echo "============================================="
echo ""

# Safety: create pre-config backup
TS=$(date +%Y%m%d%H%M%S)
mkdir -p /etc/config/pre-bridge-$TS
cp /etc/config/network /etc/config/wireless /etc/config/firewall /etc/config/dhcp /etc/config/system /etc/config/pre-bridge-$TS/
echo "[+] Pre-config backup saved to /etc/config/pre-bridge-$TS/"

# ============================================================
# 1. NETWORK CONFIGURATION
# ============================================================
echo ""
echo "[1/5] Configuring network..."

# Clear existing bridge-vlans
while uci -q delete network.@bridge-vlan[-1]; do :; done

# Bridge device: wan + lan1-4 (wan joins the bridge as a trunk port)
uci set network.cfg030f15=device
uci set network.cfg030f15.name='br-lan'
uci set network.cfg030f15.type='bridge'
uci -q delete network.cfg030f15.ports
uci add_list network.cfg030f15.ports='wan'
uci add_list network.cfg030f15.ports='lan1'
uci add_list network.cfg030f15.ports='lan2'
uci add_list network.cfg030f15.ports='lan3'
uci add_list network.cfg030f15.ports='lan4'

# --- Bridge VLANs ---

# VLAN 10 (LAN/WiFi/Management)
#   - lan1: tagged (trunk to RouterF2)
#   - lan4: tagged (management access via sub-interface)
uci add network bridge-vlan
uci set network.@bridge-vlan[-1].device='br-lan'
uci set network.@bridge-vlan[-1].vlan='10'
uci add_list network.@bridge-vlan[-1].ports='lan1:t'
uci add_list network.@bridge-vlan[-1].ports='lan4:t'

# VLAN 20 (IoT)
#   - lan1: tagged (trunk)
#   - lan2: untagged+PVID (IoT access)
#   - lan3: untagged+PVID (IoT access)
#   - lan4: untagged+PVID (IoT access + management via VLAN 10 tag)
uci add network bridge-vlan
uci set network.@bridge-vlan[-1].device='br-lan'
uci set network.@bridge-vlan[-1].vlan='20'
uci add_list network.@bridge-vlan[-1].ports='lan1:t'
uci add_list network.@bridge-vlan[-1].ports='lan2:u*'
uci add_list network.@bridge-vlan[-1].ports='lan3:u*'
uci add_list network.@bridge-vlan[-1].ports='lan4:u*'

# VLAN 51 (Starlink)
#   - lan1: tagged (trunk to RouterF2)
#   - wan: untagged+PVID (Starlink modem plugs in here)
uci add network bridge-vlan
uci set network.@bridge-vlan[-1].device='br-lan'
uci set network.@bridge-vlan[-1].vlan='51'
uci add_list network.@bridge-vlan[-1].ports='lan1:t'
uci add_list network.@bridge-vlan[-1].ports='wan:u*'

# --- Interfaces ---

# LAN (VLAN 10) - management IP for this router
uci set network.lan=interface
uci set network.lan.device='br-lan.10'
uci set network.lan.proto='static'
uci set network.lan.ipaddr='192.168.10.2'
uci set network.lan.netmask='255.255.255.0'
uci set network.lan.gateway='192.168.10.1'
uci add_list network.lan.dns='192.168.10.1'
uci -q delete network.lan.ip6assign

# IoT (VLAN 20) - no IP needed, just bridged
uci set network.iot=interface
uci set network.iot.device='br-lan.20'
uci set network.iot.proto='none'

# Starlink (VLAN 51) - no IP, just L2 bridge pass-through to RouterF2
uci set network.starlink=interface
uci set network.starlink.device='br-lan.51'
uci set network.starlink.proto='none'

# Remove WAN interfaces (RouterF2 handles all routing)
uci -q delete network.wan
uci -q delete network.wan6

echo "[+] Network configured"

# ============================================================
# 2. WIRELESS CONFIGURATION
# ============================================================
echo ""
echo "[2/5] Configuring wireless..."

# 2.4G main -> VLAN 10 (lan)
uci set wireless.default_radio0.network='lan'
uci set wireless.default_radio0.ssid='Md Abdullah'

# 5G main -> VLAN 10 (lan)
uci set wireless.default_radio1.network='lan'
uci set wireless.default_radio1.ssid='Md Abdullah 5G'

# 2.4G IoT -> VLAN 20 (iot)
uci set wireless.wifinet0.network='iot'
uci set wireless.wifinet0.ssid='Md Abdullah - IoT'

# 5G IoT -> VLAN 20 (iot)
uci set wireless.wifinet1.network='iot'
uci set wireless.wifinet1.ssid='Md Abdullah - IoT 5G'

echo "[+] Wireless configured"

# ============================================================
# 3. FIREWALL — MINIMAL (bridge mode)
# ============================================================
echo ""
echo "[3/5] Configuring firewall (bridge-mode passthrough)..."

# Clear all zones, forwardings, rules
while uci -q delete firewall.@forwarding[-1]; do :; done
while uci -q delete firewall.@rule[-1]; do :; done
while uci -q delete firewall.@zone[-1]; do :; done

# Single permissive zone for the bridge
uci add firewall zone
uci set firewall.@zone[-1].name='lan'
uci add_list firewall.@zone[-1].network='lan'
uci add_list firewall.@zone[-1].network='iot'
uci add_list firewall.@zone[-1].network='starlink'
uci set firewall.@zone[-1].input='ACCEPT'
uci set firewall.@zone[-1].output='ACCEPT'
uci set firewall.@zone[-1].forward='ACCEPT'

# Defaults — no NAT, no SYN flood (RouterF2 handles security)
uci set firewall.@defaults[0].syn_flood='0'
uci set firewall.@defaults[0].input='ACCEPT'
uci set firewall.@defaults[0].output='ACCEPT'
uci set firewall.@defaults[0].forward='ACCEPT'
uci -q delete firewall.@defaults[0].flow_offloading
uci -q delete firewall.@defaults[0].flow_offloading_hw

echo "[+] Firewall configured (permissive bridge mode)"

# ============================================================
# 4. DHCP — DISABLED (RouterF2 serves all DHCP)
# ============================================================
echo ""
echo "[4/5] Disabling DHCP..."

uci set dhcp.lan.ignore='1'
uci -q delete dhcp.lan.dhcpv6
uci -q delete dhcp.lan.ra
uci -q delete dhcp.lan.ra_slaac
uci -q delete dhcp.lan.ra_flags

# Disable DHCP on IoT and Starlink interfaces
uci set dhcp.iot=dhcp
uci set dhcp.iot.interface='iot'
uci set dhcp.iot.ignore='1'
uci set dhcp.starlink=dhcp
uci set dhcp.starlink.interface='starlink'
uci set dhcp.starlink.ignore='1'

# Disable odhcpd
uci set dhcp.odhcpd.maindhcp='0'
uci set dhcp.odhcpd.loglevel='4'

# Keep dnsmasq running for local DNS cache but no DHCP
uci set dhcp.@dnsmasq[0].port='0'

echo "[+] DHCP disabled (RouterF2 serves DHCP for all VLANs)"

# ============================================================
# 5. SYSTEM TWEAKS
# ============================================================
echo ""
echo "[5/5] System tweaks..."

uci set system.@system[0].hostname='RouterF5'
uci set system.@system[0].conloglevel='5'
uci set system.@system[0].cronloglevel='5'
uci set system.@system[0].log_size='256'
uci set system.@system[0].urandom_seed='512'

echo "[+] System configured"

# ============================================================
# COMMIT AND APPLY
# ============================================================
echo ""
echo "Committing all changes..."

uci commit network
uci commit wireless
uci commit firewall
uci commit dhcp
uci commit system

echo "[+] All changes committed"
echo ""
echo "============================================="
echo " Configuration complete!"
echo "============================================="
echo ""
echo "Rollback command (if needed):"
echo "  cp /etc/config/pre-bridge-${TS}/* /etc/config/"
echo "  /etc/init.d/network restart"
echo ""
echo "To apply now, run:"
echo "  /etc/init.d/network restart"
echo "  /etc/init.d/firewall restart"
echo "  /etc/init.d/dnsmasq restart"
echo ""
echo "WARNING: After restart, router IP changes to:"
echo "  VLAN 10: 192.168.10.2 (management)"
echo ""
echo "Reconnect via:"
echo "  ssh root@192.168.10.2 (from VLAN 10 / WiFi)"
echo "  Or plug into LAN4 and use eth0.10 VLAN sub-interface for management"
