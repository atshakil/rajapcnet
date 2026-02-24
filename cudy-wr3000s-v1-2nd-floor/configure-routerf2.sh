#!/bin/bash
# RouterF2 VLAN + Multi-WAN Configuration Script
# Cudy WR3000S v1 (2nd Floor) - OpenWrt 24.10.5
#
# VLAN Layout:
#   VLAN 1  = Switch management (192.168.0.0/24)
#   VLAN 10 = Generic WiFi/LAN  (192.168.10.0/24)
#   VLAN 20 = IoT devices        (192.168.20.0/24)
#   VLAN 50 = Local ISP (PPPoE)  - on LAN1 trunk
#   VLAN 51 = Starlink ISP (DHCP) - on LAN2 trunk
#
# Port Layout:
#   LAN1 = trunk (VLAN 1,10,20,50 tagged)
#   LAN2 = trunk (VLAN 1,10,20,51 tagged)
#   LAN3,LAN4 = unused (removed from bridge)
#   WAN  = unused (ISP now via VLAN 50/51)
#
# Multi-WAN: Starlink(51) primary, Local ISP(50) fallback
#
# Usage: scp this to router, then: sh /tmp/configure-routerf2.sh

set -e

echo "============================================="
echo " RouterF2 VLAN + Multi-WAN Configuration"
echo "============================================="
echo ""

# Safety: create pre-config backup
TS=$(date +%Y%m%d%H%M%S)
mkdir -p /etc/config/pre-vlan-$TS
cp /etc/config/network /etc/config/wireless /etc/config/firewall /etc/config/dhcp /etc/config/pre-vlan-$TS/
echo "[+] Pre-config backup saved to /etc/config/pre-vlan-$TS/"

# ============================================================
# 1. INSTALL MWAN3
# ============================================================
echo ""
echo "[1/7] Installing mwan3..."
opkg update >/dev/null 2>&1
opkg install mwan3 luci-app-mwan3 2>&1 | grep -E "Installing|already"
echo "[+] mwan3 installed"

# ============================================================
# 2. NETWORK CONFIGURATION
# ============================================================
echo ""
echo "[2/7] Configuring network..."

# Clear existing bridge-vlans
while uci -q delete network.@bridge-vlan[-1]; do :; done

# Bridge device: only lan1 and lan2 as trunk ports
uci set network.cfg030f15=device
uci set network.cfg030f15.name='br-lan'
uci set network.cfg030f15.type='bridge'
uci -q delete network.cfg030f15.ports
uci add_list network.cfg030f15.ports='lan1'
uci add_list network.cfg030f15.ports='lan2'

# --- Bridge VLANs ---

# VLAN 1 (Management) - tagged on both trunks
uci add network bridge-vlan
uci set network.@bridge-vlan[-1].device='br-lan'
uci set network.@bridge-vlan[-1].vlan='1'
uci add_list network.@bridge-vlan[-1].ports='lan1:t'
uci add_list network.@bridge-vlan[-1].ports='lan2:t'

# VLAN 10 (LAN/WiFi) - tagged on both trunks
uci add network bridge-vlan
uci set network.@bridge-vlan[-1].device='br-lan'
uci set network.@bridge-vlan[-1].vlan='10'
uci add_list network.@bridge-vlan[-1].ports='lan1:t'
uci add_list network.@bridge-vlan[-1].ports='lan2:t'

# VLAN 20 (IoT) - tagged on both trunks
uci add network bridge-vlan
uci set network.@bridge-vlan[-1].device='br-lan'
uci set network.@bridge-vlan[-1].vlan='20'
uci add_list network.@bridge-vlan[-1].ports='lan1:t'
uci add_list network.@bridge-vlan[-1].ports='lan2:t'

# VLAN 50 (Local ISP) - tagged on LAN1 only
uci add network bridge-vlan
uci set network.@bridge-vlan[-1].device='br-lan'
uci set network.@bridge-vlan[-1].vlan='50'
uci add_list network.@bridge-vlan[-1].ports='lan1:t'

# VLAN 51 (Starlink ISP) - tagged on LAN2 only
uci add network bridge-vlan
uci set network.@bridge-vlan[-1].device='br-lan'
uci set network.@bridge-vlan[-1].vlan='51'
uci add_list network.@bridge-vlan[-1].ports='lan2:t'

# --- Interfaces ---

# Management (VLAN 1)
uci set network.mgmt=interface
uci set network.mgmt.device='br-lan.1'
uci set network.mgmt.proto='static'
uci set network.mgmt.ipaddr='192.168.0.1'
uci set network.mgmt.netmask='255.255.255.0'

# LAN / Generic WiFi (VLAN 10)
uci set network.lan=interface
uci set network.lan.device='br-lan.10'
uci set network.lan.proto='static'
uci set network.lan.ipaddr='192.168.10.1'
uci set network.lan.netmask='255.255.255.0'
uci -q delete network.lan.ip6assign

# IoT (VLAN 20)
uci set network.iot=interface
uci set network.iot.device='br-lan.20'
uci set network.iot.proto='static'
uci set network.iot.ipaddr='192.168.20.1'
uci set network.iot.netmask='255.255.255.0'

# WAN - Local ISP via PPPoE on VLAN 50
uci set network.wan=interface
uci set network.wan.device='br-lan.50'
uci set network.wan.proto='pppoe'
uci set network.wan.username='aa.abdullah'
uci set network.wan.password='12345'
uci set network.wan.ipv6='auto'
uci set network.wan.metric='20'

# WAN2 - Starlink via DHCP on VLAN 51
uci set network.wan2=interface
uci set network.wan2.device='br-lan.51'
uci set network.wan2.proto='dhcp'
uci set network.wan2.metric='10'

# Remove wan6 (no longer using physical wan port)
uci -q delete network.wan6

echo "[+] Network configured"

# ============================================================
# 3. WIRELESS CONFIGURATION
# ============================================================
echo ""
echo "[3/7] Configuring wireless..."

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
# 4. FIREWALL CONFIGURATION
# ============================================================
echo ""
echo "[4/7] Configuring firewall..."

# Clear existing zones and forwardings, rebuild
while uci -q delete firewall.@forwarding[-1]; do :; done
while uci -q delete firewall.@zone[-1]; do :; done

# Zone: mgmt (VLAN 1)
uci add firewall zone
uci set firewall.@zone[-1].name='mgmt'
uci add_list firewall.@zone[-1].network='mgmt'
uci set firewall.@zone[-1].input='ACCEPT'
uci set firewall.@zone[-1].output='ACCEPT'
uci set firewall.@zone[-1].forward='ACCEPT'

# Zone: lan (VLAN 10)
uci add firewall zone
uci set firewall.@zone[-1].name='lan'
uci add_list firewall.@zone[-1].network='lan'
uci set firewall.@zone[-1].input='ACCEPT'
uci set firewall.@zone[-1].output='ACCEPT'
uci set firewall.@zone[-1].forward='ACCEPT'

# Zone: iot (VLAN 20)
uci add firewall zone
uci set firewall.@zone[-1].name='iot'
uci add_list firewall.@zone[-1].network='iot'
uci set firewall.@zone[-1].input='REJECT'
uci set firewall.@zone[-1].output='ACCEPT'
uci set firewall.@zone[-1].forward='REJECT'

# Zone: wan (both ISPs)
uci add firewall zone
uci set firewall.@zone[-1].name='wan'
uci add_list firewall.@zone[-1].network='wan'
uci add_list firewall.@zone[-1].network='wan2'
uci set firewall.@zone[-1].input='REJECT'
uci set firewall.@zone[-1].output='ACCEPT'
uci set firewall.@zone[-1].forward='REJECT'
uci set firewall.@zone[-1].masq='1'
uci set firewall.@zone[-1].mtu_fix='1'

# Forwardings: all internal zones -> wan
uci add firewall forwarding
uci set firewall.@forwarding[-1].src='mgmt'
uci set firewall.@forwarding[-1].dest='wan'

uci add firewall forwarding
uci set firewall.@forwarding[-1].src='lan'
uci set firewall.@forwarding[-1].dest='wan'

uci add firewall forwarding
uci set firewall.@forwarding[-1].src='iot'
uci set firewall.@forwarding[-1].dest='wan'

echo "[+] Firewall configured"

# ============================================================
# 5. DHCP CONFIGURATION
# ============================================================
echo ""
echo "[5/7] Configuring DHCP..."

# Management VLAN DHCP (small pool)
uci set dhcp.mgmt=dhcp
uci set dhcp.mgmt.interface='mgmt'
uci set dhcp.mgmt.start='100'
uci set dhcp.mgmt.limit='50'
uci set dhcp.mgmt.leasetime='12h'
uci set dhcp.mgmt.dhcpv4='server'

# LAN DHCP (VLAN 10)
uci set dhcp.lan=dhcp
uci set dhcp.lan.interface='lan'
uci set dhcp.lan.start='100'
uci set dhcp.lan.limit='150'
uci set dhcp.lan.leasetime='12h'
uci set dhcp.lan.dhcpv4='server'
# Remove IPv6 RA/DHCPv6 for now
uci -q delete dhcp.lan.dhcpv6
uci -q delete dhcp.lan.ra
uci -q delete dhcp.lan.ra_slaac
uci -q delete dhcp.lan.ra_flags

# IoT DHCP (VLAN 20)
uci set dhcp.iot=dhcp
uci set dhcp.iot.interface='iot'
uci set dhcp.iot.start='100'
uci set dhcp.iot.limit='150'
uci set dhcp.iot.leasetime='12h'
uci set dhcp.iot.dhcpv4='server'

# WAN - no DHCP server
uci set dhcp.wan=dhcp
uci set dhcp.wan.interface='wan'
uci set dhcp.wan.ignore='1'

echo "[+] DHCP configured"

# ============================================================
# 6. MWAN3 CONFIGURATION
# ============================================================
echo ""
echo "[6/7] Configuring mwan3 failover..."

# Clear any existing mwan3 config
> /etc/config/mwan3

# --- Interfaces ---

# wan (Local ISP) - lower priority
uci set mwan3.wan=interface
uci set mwan3.wan.enabled='1'
uci set mwan3.wan.initial_state='online'
uci set mwan3.wan.family='ipv4'
uci add_list mwan3.wan.track_ip='8.8.8.8'
uci add_list mwan3.wan.track_ip='1.1.1.1'
uci set mwan3.wan.track_method='ping'
uci set mwan3.wan.reliability='1'
uci set mwan3.wan.count='3'
uci set mwan3.wan.size='56'
uci set mwan3.wan.max_ttl='60'
uci set mwan3.wan.check_quality='0'
uci set mwan3.wan.timeout='4'
uci set mwan3.wan.interval='10'
uci set mwan3.wan.failure_interval='5'
uci set mwan3.wan.recovery_interval='5'
uci set mwan3.wan.down='5'
uci set mwan3.wan.up='3'

# wan2 (Starlink) - higher priority
uci set mwan3.wan2=interface
uci set mwan3.wan2.enabled='1'
uci set mwan3.wan2.initial_state='online'
uci set mwan3.wan2.family='ipv4'
uci add_list mwan3.wan2.track_ip='8.8.4.4'
uci add_list mwan3.wan2.track_ip='1.0.0.1'
uci set mwan3.wan2.track_method='ping'
uci set mwan3.wan2.reliability='1'
uci set mwan3.wan2.count='3'
uci set mwan3.wan2.size='56'
uci set mwan3.wan2.max_ttl='60'
uci set mwan3.wan2.check_quality='0'
uci set mwan3.wan2.timeout='4'
uci set mwan3.wan2.interval='10'
uci set mwan3.wan2.failure_interval='5'
uci set mwan3.wan2.recovery_interval='5'
uci set mwan3.wan2.down='5'
uci set mwan3.wan2.up='3'

# --- Members ---

# Starlink primary (weight 1, metric 1)
uci set mwan3.wan2_m1=member
uci set mwan3.wan2_m1.interface='wan2'
uci set mwan3.wan2_m1.metric='1'
uci set mwan3.wan2_m1.weight='1'

# Local ISP fallback (weight 1, metric 2)
uci set mwan3.wan_m2=member
uci set mwan3.wan_m2.interface='wan'
uci set mwan3.wan_m2.metric='2'
uci set mwan3.wan_m2.weight='1'

# --- Policy ---

# Failover policy: Starlink first, Local ISP second
uci set mwan3.failover=policy
uci add_list mwan3.failover.use_member='wan2_m1'
uci add_list mwan3.failover.use_member='wan_m2'
uci set mwan3.failover.last_resort='default'

# --- Rules ---

# Default rule: all traffic uses failover policy
uci set mwan3.default_rule=rule
uci set mwan3.default_rule.dest_ip='0.0.0.0/0'
uci set mwan3.default_rule.use_policy='failover'
uci set mwan3.default_rule.proto='all'

echo "[+] mwan3 failover configured"

# ============================================================
# 7. COMMIT AND APPLY
# ============================================================
echo ""
echo "[7/7] Committing changes..."

uci commit network
uci commit wireless
uci commit firewall
uci commit dhcp
uci commit mwan3

echo "[+] All changes committed"
echo ""
echo "============================================="
echo " Configuration complete!"
echo "============================================="
echo ""
echo "Rollback command (if needed):"
echo "  cp /etc/config/pre-vlan-${TS}/* /etc/config/"
echo "  /etc/init.d/network restart"
echo ""
echo "To apply now, run:"
echo "  /etc/init.d/network restart"
echo "  /etc/init.d/firewall restart"
echo "  /etc/init.d/dnsmasq restart"
echo "  /etc/init.d/mwan3 restart"
echo ""
echo "WARNING: After restart, router LAN IP changes to:"
echo "  VLAN 1  (mgmt): 192.168.0.1"
echo "  VLAN 10 (LAN):  192.168.10.1"
echo "  VLAN 20 (IoT):  192.168.20.1"
echo ""
echo "You will need to reconnect WiFi and get a new"
echo "DHCP lease on the 192.168.10.0/24 subnet."
