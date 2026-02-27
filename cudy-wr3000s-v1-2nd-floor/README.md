# Cudy WR3000S v1 — 2nd Floor Router (RouterF2)

## Hardware

| Field | Value |
|-------|-------|
| Model | Cudy WR3000S v1 |
| SoC | MediaTek MT7981 (Filogic 820), dual-core ARM Cortex-A53 |
| RAM | 256 MB DDR4 |
| Flash | 128 MB NAND (UBIFS overlay) |
| Serial | WR3000S251201933 |
| Firmware | OpenWrt 24.10.5 (r29087-d9c5716d1d) |
| Kernel | 6.6.119 |
| Hostname | RouterF2 |
| Location | 2nd floor, primary apartment |

---

## Network Architecture

```
                          ┌─────────────────────────────┐
    ISP ONU ──────────────┤  Netgear GS108Ev3 Switch    │
    (fiber, PPPoE)        │  Port 1=ISP  Port 2=Trunk   │
                          └──────────┬──────────────────┘
                                     │ trunk (VLAN 1,10,20,50 tagged)
                                     │
                              ┌──────┴──────┐
                              │  RouterF2   │
                              │  LAN1       │ ← switch trunk
                              │  LAN2       │ ← future Starlink (VLAN 51)
                              │  LAN3       │ ← unused (not in bridge)
                              │  LAN4       │ ← emergency access (VLAN 10 untagged)
                              │  WAN port   │ ← unused (ISP via VLAN 50)
                              └─────────────┘
                                │       │
                           WiFi 2.4G  WiFi 5G
```

---

## IP Addresses

| Interface | VLAN | Subnet | Router IP | Purpose |
|-----------|------|--------|-----------|---------|
| br-lan.1 | 1 | 192.168.0.0/24 | 192.168.0.1 | Management (switch + router admin) |
| br-lan.10 | 10 | 192.168.10.0/24 | 192.168.10.1 | LAN + WiFi (main network) |
| br-lan.20 | 20 | 192.168.20.0/24 | 192.168.20.1 | IoT devices (cameras, sensors) |
| br-lan.50 | 50 | — (PPPoE) | — | Local ISP WAN |
| br-lan.51 | 51 | — (DHCP) | — | Starlink WAN (disabled) |

**Primary SSH access:** `ssh root@192.168.10.1` (from any device on VLAN 10 WiFi)  
**Web UI:** http://192.168.10.1 (LuCI)

---

## WiFi Networks

| Band | SSID | VLAN | Interface | Encryption |
|------|------|------|-----------|------------|
| 2.4 GHz (HE40, Ch 11) | Md Abdullah | 10 (lan) | phy0-ap0 | SAE-mixed (WPA2/WPA3) |
| 5 GHz (HE80, Ch 149) | Md Abdullah 5G | 10 (lan) | phy1-ap0 | SAE-mixed (WPA2/WPA3) |
| 2.4 GHz | Md Abdullah - IoT | 20 (iot) | phy0-ap1 | SAE-mixed (WPA2/WPA3) |
| 5 GHz | Md Abdullah - IoT 5G | 20 (iot) | phy1-ap1 | SAE-mixed (WPA2/WPA3) |

WiFi key: `pomeranian24` (all SSIDs)

---

## WAN Configuration

### Active: Local ISP (PPPoE on VLAN 50)

| Setting | Value |
|---------|-------|
| Protocol | PPPoE |
| Username | aa.abdullah |
| Password | 12345 |
| Interface | br-lan.50 |
| MTU | 1492 (negotiated 1484) |
| Metric | 20 |

### Standby: Starlink (DHCP on VLAN 51) — Currently Disabled

| Setting | Value |
|---------|-------|
| Protocol | DHCP |
| Interface | br-lan.51 |
| Metric | 10 (higher priority) |
| Status | `auto=0`, mwan3 `enabled=0` |

**To enable Starlink when available:**
```sh
uci set network.wan2.auto="1"
uci set mwan3.wan2.enabled="1"
uci commit network && uci commit mwan3
ifup wan2 && /etc/init.d/mwan3 restart
```

### Multi-WAN Failover (mwan3)

- **Policy:** `failover` — Starlink (wan2) primary, local ISP (wan) fallback
- **Tracking:** wan pings 8.8.8.8 + 1.1.1.1, wan2 pings 8.8.4.4 + 1.0.0.1
- **Failover:** 5 failures (50s) to mark down, 3 successes (30s) to recover

---

## Firewall

### Zones

| Zone | Networks | Input | Forward | Masquerade |
|------|----------|-------|---------|------------|
| mgmt | VLAN 1 | ACCEPT | ACCEPT | No |
| lan | VLAN 10 | ACCEPT | ACCEPT | No |
| iot | VLAN 20 | REJECT | REJECT | No |
| wan | wan + wan2 | REJECT | REJECT | Yes |

### Forwardings

| Source | Destination |
|--------|-------------|
| mgmt | wan |
| lan | wan |
| iot | wan |

### Custom Rules

| Rule | Source | Action | Purpose |
|------|--------|--------|---------|
| Allow-IoT-DHCP | iot | ACCEPT (udp 67-68) | IoT devices get DHCP leases |
| Allow-IoT-DNS | iot | ACCEPT (tcp/udp 53) | IoT devices resolve DNS |
| Block-IoT-to-LAN | iot→lan | REJECT | Isolate IoT from main LAN |
| Allow-Mgmt-DHCP | mgmt | ACCEPT (udp 67-68) | Management DHCP |

> IoT devices can reach the internet and talk to each other, but cannot access LAN (192.168.10.x) or router services.

---

## DHCP Pools

| Pool | Interface | Range | Lease |
|------|-----------|-------|-------|
| mgmt | VLAN 1 | 192.168.0.100–149 | 12h |
| lan | VLAN 10 | 192.168.10.100–249 | 12h |
| iot | VLAN 20 | 192.168.20.100–249 | 12h |

---

## Performance Optimizations

| Setting | Value | Purpose |
|---------|-------|---------|
| flow_offloading | 1 | Software flow offload — bypass nftables for established connections |
| flow_offloading_hw | 1 | Hardware offload via MT7981 PPE — wire-speed forwarding |
| packet_steering | 1 | Distribute packet processing across both CPU cores |
| syn_flood | 1 | SYN flood protection with syncookies |
| conloglevel | 5 (notice) | Suppress debug kernel messages |
| cronloglevel | 5 (notice) | Suppress cron execution spam |
| log_size | 256 KB | Larger ring buffer for post-crash debugging |
| urandom_seed | 512 | Persist entropy across reboots — faster crypto init |
| odhcpd | disabled | No IPv6 in use — saves ~2MB RAM |

### Conntrack Tuning (`/etc/sysctl.d/`)

| Timeout | Value | Default |
|---------|-------|---------|
| tcp_established | 7440s (~2h) | 432000s (5 days) |
| udp | 60s | 30s |
| udp_stream | 180s | 180s |

---

## Port Layout

```
 ┌──────────────────────────────────────────────┐
 │          RouterF2 — REAR PANEL               │
 │                                              │
 │  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌───┐ │
 │  │ WAN │  │LAN 1│  │LAN 2│  │LAN 3│  │L 4│ │
 │  └─────┘  └─────┘  └─────┘  └─────┘  └───┘ │
 │  unused   trunk     future   unused   EMRG  │
 │           to GS108  Starlink          VL10  │
 │           VL 1,10,  VL 1,10,          untag │
 │           20,50     20,51                   │
 └──────────────────────────────────────────────┘
```

---

## Common Tasks

### SSH into the router
```sh
ssh root@192.168.10.1
```

### Check WAN status
```sh
ssh root@192.168.10.1 'ifstatus wan | jsonfilter -e @.up -e @[\"ipv4-address\"][0].address'
```

### Check WiFi clients
```sh
ssh root@192.168.10.1 'for i in phy0-ap0 phy0-ap1 phy1-ap0 phy1-ap1; do
  E=$(iwinfo $i info 2>/dev/null | grep ESSID | awk -F\" "{print \$2}")
  C=$(iwinfo $i assoclist 2>/dev/null | grep dBm | wc -l)
  echo "$i ($E): $C clients"
done'
```

### Check DHCP leases
```sh
ssh root@192.168.10.1 'cat /tmp/dhcp.leases'
```

### Check flow offload entries (under load)
```sh
ssh root@192.168.10.1 'cat /sys/kernel/debug/mtk_ppe/entries | grep "=" | wc -l'
```

### Restart PPPoE WAN
```sh
ssh root@192.168.10.1 'ifdown wan && sleep 2 && ifup wan'
```

### View recent logs
```sh
ssh root@192.168.10.1 'logread | tail -30'
```

---

## Backup & Restore

### Create a backup
```sh
./backup.sh 192.168.10.1 root
```
Saves timestamped backup to `backups/YYYYMMDD-HHMMSS/` with:
- All UCI config files (network, wireless, dhcp, firewall, system, etc.)
- Full UCI export, installed packages list, network state snapshot
- Native sysupgrade tar.gz

### Restore from backup
```sh
./restore.sh YYYYMMDD-HHMMSS
```

### Backups on record

| Timestamp | Notes |
|-----------|-------|
| 20260220-185910 | Original factory + initial setup |
| 20260224-091516 | Full VLAN + multi-WAN + optimizations |
| 20260226-235639 | Post-restore + DHCP reservation (Pi CNC, wlan0 MAC) + CNC→IoT firewall rule |
| 20260227-092514 | IoT SSID password changed — current |

---

## Recovery Procedures

### If you lose WiFi access

1. Connect a laptop to **switch Port 8** (management VLAN 1)
2. Set laptop IP to `192.168.0.100/24`
3. SSH: `ssh root@192.168.0.1`

### If you lose all access (no WiFi, no switch)

1. **Plug a laptop directly into router LAN4** (rightmost LAN port)
2. LAN4 is untagged VLAN 10 — your laptop gets a DHCP address automatically on 192.168.10.x
3. Access: http://192.168.10.1 or `ssh root@192.168.10.1`

> No VLAN configuration needed on the laptop. Just plug in and go.

If LAN4 doesn't work, try LAN1/LAN2 with a VLAN sub-interface:
```sh
# Linux
sudo ip link add link eth0 name eth0.10 type vlan id 10
sudo ip addr add 192.168.10.250/24 dev eth0.10
sudo ip link set eth0.10 up
ssh root@192.168.10.1
```

### Full factory reset (nuclear option)

Hold the reset button on the router for 10+ seconds. All config lost — returns to OpenWrt defaults (192.168.1.1).

### Rollback to pre-optimization config

```sh
ssh root@192.168.10.1
cp /etc/config/pre-optimize-20260224*/* /etc/config/
/etc/init.d/firewall restart
/etc/init.d/system restart
/etc/init.d/odhcpd enable && /etc/init.d/odhcpd start
```

### Rollback to pre-VLAN config

```sh
ssh root@192.168.10.1
cp /etc/config/pre-vlan-20260224073010/* /etc/config/
/etc/init.d/network restart
```
> Warning: This changes the router IP back to 192.168.1.1

---

## Files in This Directory

| File | Purpose |
|------|---------|
| `README.md` | This documentation |
| `backup.sh` | Backup script (usage: `./backup.sh 192.168.10.1 root`) |
| `restore.sh` | Restore script (usage: `./restore.sh YYYYMMDD-HHMMSS`) |
| `configure-routerf2.sh` | Full VLAN + multi-WAN setup script (already applied, note: LAN4 added later) |
| `serial.txt` | Router serial number (WR3000S251201933) |
| `backups/` | Timestamped config backups |
