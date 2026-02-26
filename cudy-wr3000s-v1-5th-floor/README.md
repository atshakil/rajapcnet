# Cudy WR3000S v1 — 5th Floor Bridge + AP (RouterF5)

## Hardware

| Field | Value |
|-------|-------|
| Model | Cudy WR3000S v1 |
| SoC | MediaTek MT7981 (Filogic 820), dual-core ARM Cortex-A53 |
| RAM | 256 MB DDR4 |
| Flash | 128 MB NAND (UBIFS overlay, 44.7 MB) |
| Serial | WR3000S251203116 |
| Firmware | OpenWrt 24.10.5 (r29087-d9c5716d1d) |
| Kernel | 6.6.119 |
| Hostname | RouterF5 |
| Location | 5th floor |
| Role | VLAN-aware bridge + access point (no routing/NAT/DHCP) |
| Base MAC | 80:AF:CA:B0:09:7B |

---

## Network Architecture

```
                 RouterF2 (2nd floor)
                 192.168.10.1 — all routing, NAT, DHCP, firewall
                        │
                    LAN2 │  trunk (VLAN 10, 20, 51 tagged)
                        │
                ┌───────┴───────┐
                │   RouterF5    │
                │   LAN1        │ ← trunk to RouterF2 (VLAN 10,20,51 tagged)
                │   WAN port    │ ← Starlink modem (VLAN 51 untagged)
                │   LAN2        │ ← IoT access (VLAN 20 untagged)
                │   LAN3        │ ← IoT access (VLAN 20 untagged)
                │   LAN4        │ ← IoT access (VLAN 20 untagged)
                │               │   + management (VLAN 10 tagged)
                └───────────────┘
                   │         │
              WiFi 2.4G   WiFi 5G
```

RouterF5 is a **bridge**, not a router. All traffic is switched at Layer 2 through the VLAN-aware bridge. RouterF2 handles all routing, DHCP, NAT, and firewall enforcement.

---

## IP Addresses

| Interface | VLAN | Subnet | IP | Purpose |
|-----------|------|--------|----|---------|  
| br-lan.10 | 10 | 192.168.10.0/24 | 192.168.10.2 | Management (SSH, LuCI) |
| br-lan.20 | 20 | 192.168.20.0/24 | — (none) | IoT bridge — no local IP |
| br-lan.51 | 51 | — | — (none) | Starlink bridge — no local IP |

- **Default gateway:** 192.168.10.1 (RouterF2)
- **DNS:** 192.168.10.1 (RouterF2)

**SSH access:** `ssh root@192.168.10.2` (from any device on VLAN 10)
**Web UI:** http://192.168.10.2 (LuCI)

---

## VLAN Configuration

### Bridge Members

All ports share a single VLAN-filtering bridge (`br-lan`):

| Member | Type |
|--------|------|
| wan | DSA port (Starlink access) |
| lan1 | DSA port (trunk to RouterF2) |
| lan2 | DSA port (IoT access) |
| lan3 | DSA port (IoT access) |
| lan4 | DSA port (IoT access + management) |
| phy0-ap0 | WiFi 2.4G (Md Abdullah) |
| phy0-ap1 | WiFi 2.4G (Md Abdullah - IoT) |
| phy1-ap0 | WiFi 5G (Md Abdullah 5G) |
| phy1-ap1 | WiFi 5G (Md Abdullah - IoT 5G) |

### Bridge VLANs

| VLAN ID | Purpose | Tagged Ports | Untagged + PVID |
|---------|---------|-------------|-----------------|
| 10 | LAN / WiFi / Management | lan1, lan4 | — |
| 20 | IoT | lan1 | lan2, lan3, lan4 |
| 51 | Starlink WAN | lan1 | wan |

> WiFi interfaces are implicitly tagged via their network assignment (lan → VLAN 10, iot → VLAN 20).
> LAN4 carries both VLAN 20 (untagged, PVID) and VLAN 10 (tagged). Use a VLAN sub-interface (e.g., `eth0.10`) for management access from LAN4.

```
 ┌──────────────────────────────────────────────┐
 │          RouterF5 — REAR PANEL               │
 │                                              │
 │  ┌─────┐  ┌─────┐  ┌─────┐  ┌─────┐  ┌───┐ │
 │  │ WAN │  │LAN 1│  │LAN 2│  │LAN 3│  │L 4│ │
 │  └─────┘  └─────┘  └─────┘  └─────┘  └───┘ │
 │  Starlink trunk    IoT      IoT      IoT   │
 │  VL51     to       VL20     VL20     VL20  │
 │  untagged RouterF2 untagged untagged +VL10t │
 │           VL10,20,                   mgmt  │
 │           51 tag                            │
 └──────────────────────────────────────────────┘
```

---

## WiFi Networks

| Band | SSID | VLAN | Interface | Encryption | Client Isolation |
|------|------|------|-----------|------------|-----------------|
| 2.4 GHz (HE40, Ch 11) | Md Abdullah | 10 (lan) | phy0-ap0 | SAE-mixed (WPA2/WPA3) | No |
| 5 GHz (HE80, Ch 149) | Md Abdullah 5G | 10 (lan) | phy1-ap0 | SAE-mixed (WPA2/WPA3) | No |
| 2.4 GHz | Md Abdullah - IoT | 20 (iot) | phy0-ap1 | SAE-mixed (WPA2/WPA3) | Yes |
| 5 GHz | Md Abdullah - IoT 5G | 20 (iot) | phy1-ap1 | SAE-mixed (WPA2/WPA3) | Yes |

WiFi key: `pomeranian24` (all SSIDs)
Country: BD (Bangladesh)

> Same SSIDs and keys as RouterF2 — devices roam seamlessly between floors.

### WiFi BSSIDs

| Interface | BSSID |
|-----------|-------|
| phy0-ap0 | 80:AF:CA:B0:09:7A |
| phy0-ap1 | 82:AF:CA:B0:09:7A |
| phy1-ap0 | 82:AF:CA:B0:09:7B |
| phy1-ap1 | 86:AF:CA:B0:09:7B |

---

## Firewall

Minimal / permissive — RouterF2 enforces all security policies.

### Defaults

| Setting | Value |
|---------|-------|
| Input | ACCEPT |
| Output | ACCEPT |
| Forward | ACCEPT |
| SYN flood protection | Off |
| Flow offloading | Off (bridge mode, no routing) |
| Masquerading (NAT) | Off |

### Zone

| Zone | Networks | Input | Output | Forward |
|------|----------|-------|--------|---------|
| lan | lan, iot, starlink | ACCEPT | ACCEPT | ACCEPT |

---

## DHCP

All DHCP is disabled. RouterF2 serves DHCP for all VLANs.

| Setting | Value |
|---------|-------|
| dhcp.lan.ignore | 1 |
| dhcp.iot.ignore | 1 |
| dhcp.starlink.ignore | 1 |
| dnsmasq | disabled (service stopped, not just port=0) |
| odhcpd | disabled |

---

## System Tweaks

| Setting | Value | Purpose |
|---------|-------|---------|
| packet_steering | 1 | Distribute IRQs across both CPU cores |
| conloglevel | 5 (notice) | Suppress debug kernel messages |
| cronloglevel | 5 (notice) | Suppress cron execution spam |
| log_size | 64 KB | Bounded ring buffer — no unbounded growth |
| urandom_seed | 512 | Persist entropy across reboots |
| odhcpd | disabled | No IPv6 in use |
| dnsmasq | disabled | No DNS/DHCP needed — saves ~3 MB RAM |

## Common Tasks

### SSH into the router
```sh
ssh root@192.168.10.2
```

### Via jump host (from Mac, through Pi)
```sh
ssh -o ProxyCommand="ssh -W %h:%p admin@cyg.local" root@192.168.10.2
```

### Check WiFi clients
```sh
ssh root@192.168.10.2 'for i in phy0-ap0 phy0-ap1 phy1-ap0 phy1-ap1; do
  E=$(iwinfo $i info 2>/dev/null | grep ESSID | awk -F\" "{print \$2}")
  C=$(iwinfo $i assoclist 2>/dev/null | grep dBm | wc -l)
  echo "$i ($E): $C clients"
done'
```

### Check bridge VLAN status
```sh
ssh root@192.168.10.2 'cat /sys/class/net/br-lan/bridge/vlan_filtering; cat /proc/net/vlan/config'
```

### Check interface status
```sh
ssh root@192.168.10.2 'for p in wan lan1 lan2 lan3 lan4; do
  echo "$p: $(cat /sys/class/net/$p/operstate)"
done'
```

### View recent logs
```sh
ssh root@192.168.10.2 'logread | tail -30'
```

---

## Backup & Restore

### Create a backup
```sh
./backup.sh 192.168.10.2 root
```
Saves timestamped backup to `backups/YYYYMMDD-HHMMSS/`.

### Restore from backup
```sh
./restore.sh YYYYMMDD-HHMMSS
```

### Backups on record

| Timestamp | Notes |
|-----------|-------|
| 20260224-021201 | Original factory + initial setup (pre-bridge config) |
| 20260224-112758 | VLAN bridge + AP config (WAN=trunk, LAN4=emergency) |
| 20260224-114329 | New port layout (WAN=Starlink, LAN1=trunk, LAN4=IoT+mgmt) |
| 20260224-122545 | Optimizations (packet_steering, log_size=64, dnsmasq disabled) |
| 20260226-224425 | LAN4 changed to VLAN 10 access port — current |

---

## Recovery Procedures

### If you lose WiFi access

1. Plug a laptop directly into **LAN4** (rightmost LAN port)
2. LAN4 is VLAN 20 untagged — IoT devices get DHCP automatically on 192.168.20.x
3. For management access, create a VLAN 10 sub-interface:
```sh
# Linux
sudo ip link add link eth0 name eth0.10 type vlan id 10
sudo ip addr add 192.168.10.100/24 dev eth0.10
sudo ip link set eth0.10 up
ssh root@192.168.10.2
```

> If the trunk cable to RouterF2 is disconnected, no DHCP is available. Set your laptop IP manually.

### If LAN4 doesn't work (Pi is occupying it)

Unplug the Pi from LAN4 and plug in your laptop. LAN2/LAN3 are also VLAN 20 access ports — same procedure applies.

### Rollback to pre-bridge config

```sh
ssh root@192.168.10.2
cp /etc/config/pre-bridge-20251218080100/* /etc/config/
/etc/init.d/network restart
```
> Warning: This changes the router back to standalone mode at 192.168.1.1.

### Full factory reset (nuclear option)

Hold the reset button on the router for 10+ seconds. All config lost — returns to OpenWrt defaults (192.168.1.1).

---

## How It Works (Bridge Mode Explained)

RouterF5 operates as a **VLAN-aware Layer 2 bridge**. It does NOT:
- Route packets between subnets
- Perform NAT/masquerading
- Serve DHCP leases
- Enforce firewall rules (beyond allowing all)

Instead, it **extends** VLANs from RouterF2:
1. The LAN1 port carries a trunk with VLAN 10, 20, and 51 (all tagged) to/from RouterF2
2. The WAN port receives Starlink (VLAN 51 untagged) and bridges it through the trunk
3. The bridge strips VLAN tags for untagged access ports (LAN2-4 → VLAN 20)
4. LAN4 also carries VLAN 10 tagged for management access (requires VLAN sub-interface)
5. WiFi SSIDs are associated with their respective VLANs (lan → VLAN 10, iot → VLAN 20)
6. All DHCP requests, DNS queries, and internet traffic traverse the trunk back to RouterF2

The router keeps a management IP (192.168.10.2) on VLAN 10 for SSH/LuCI access.

---

## Physical Connectivity (Current)

| From | To | Cable | Status |
|------|----|-------|--------|
| RouterF2 LAN2 | RouterF5 LAN1 | Ethernet (trunk) | **Not yet connected** |
| Starlink modem | RouterF5 WAN | Ethernet | **Not yet connected** |
| Raspberry Pi eth0 | RouterF5 LAN4 | Ethernet | Connected |

Pi network config:
- `eth0` → VLAN 20 untagged → 192.168.20.250/24 (IoT)
- `eth0.10` → VLAN 10 tagged → 192.168.10.250/24 (management)

> Once the trunk cable is connected, RouterF5 will be fully online — WiFi clients get DHCP from RouterF2 and have internet access. Starlink connectivity requires the modem plugged into the WAN port.

---

## Files in This Directory

| File | Purpose |
|------|---------|
| `README.md` | This documentation |
| `backup.sh` | Backup script (usage: `./backup.sh 192.168.10.2 root`) |
| `restore.sh` | Restore script (usage: `./restore.sh YYYYMMDD-HHMMSS`) |
| `configure-routerf5.sh` | VLAN bridge + AP setup script (already applied) |
| `serial.txt` | Router serial number (WR3000S251203116) |
| `backups/` | Timestamped config backups |
