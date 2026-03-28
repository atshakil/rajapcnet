# Rajshahi Apartment Complex 1 - Network

Primary router running OpenWrt 24.10.5 on Cudy WR3000S v1 with PPPoE WAN and dual-band WiFi 6.

## Network Topology

```
                         Netgear GS108Ev3 (8-port managed)
                        ┌──────────────────────────────┐
ISP ONT ──► [Port 1]    │  VLAN 50 (isolated WAN)      │    [Port 2] ──► Cudy WAN
                        │                              │
                        │  Ports 3-8: LAN (VLAN 1)     │
                        │           + IoT (VLAN 10)     │
                        └──────────────────────────────┘
                                      │
                              Cudy WR3000S v1
                        ┌──────────────────────────────┐
                        │  [WAN]          [LAN 1-4]    │──► Trunk to switches
                        │   PPPoE          VLAN 1 (u*) │    (VLAN 1 untagged +
                        │                  VLAN 10 (t)  │     VLAN 10 tagged)
                        │                              │
                        │  br-lan.1  (192.168.1.0/24)  │ — LAN
                        │   ├── 2.4GHz: "Md Abdullah"  │
                        │   └── 5GHz: "Md Abdullah 5G" │
                        │                              │
                        │  br-lan.10 (192.168.10.0/24) │ — IoT
                        │   ├── 2.4GHz: "Md Abdullah - IOT"    │
                        │   └── 5GHz: "Md Abdullah - IOT 5G"   │
                        └──────────────────────────────┘
```

## Hardware Inventory

### Primary Router
- **Cudy WR3000S v1** (EU1.0) — Serial: WR3000S251201933
  - SoC: MediaTek MT7981BA (Filogic 820), Dual-core ARM Cortex-A53 @ 1300MHz
  - RAM: 256MB | Flash: 128MB SPI NAND
  - Switch: MediaTek MT7531AE (1x WAN + 4x LAN, Gigabit)
  - WiFi: 802.11ax (WiFi 6) — 2.4GHz 2x2 + 5GHz 2x3:2
  - Firmware: **OpenWrt 24.10.5** (r29087-d9c5716d1d), kernel 6.6.119, target mediatek/filogic
  - Role: **Primary router** (PPPoE, DHCP server, firewall, NAT)
  - Location: 2nd floor
  - Price: ~5000 BDT

### Access Points (future)
- Cudy WR3000 — Check revision for OpenWRT compatibility (v2 not compatible) (~5000 BDT)
- TP-Link Archer C6 AC1200 (WiFi 5) (~3000 BDT)

### Managed Switches
- **Netgear GS108Ev3** 8-port Gigabit managed switch
  - Role: TBD (verified: WAN isolation via VLAN 50)
  - Location: TBD
  - Price: ~4200 BDT
- TP-Link TL-SG105E 5-port (~1800 BDT) — available for expansion

## IP Addressing Scheme

| Subnet | Device | VLAN | Purpose | Gateway | DHCP Range |
|--------|--------|------|---------|---------|------------|
| 192.168.1.0/24 | br-lan.1 | 1 (untagged) | LAN (default) | 192.168.1.1 | 192.168.1.100 – 192.168.1.249 |
| 192.168.10.0/24 | br-lan.10 | 10 (tagged) | IoT (isolated) | 192.168.10.1 | 192.168.10.100 – 192.168.10.249 |
| 192.168.20.0/24 | br-lan.20 | 20 (tagged) | Cameras (RouterF2) | 192.168.20.1 | 192.168.20.100 – 192.168.20.249 |

- DNS: 1.1.1.1, 8.8.8.8 (or ISP-provided)

<!-- TODO: Define additional subnets for management, guests -->

## Network Segmentation (802.1Q VLANs via DSA)

IoT isolation is implemented via **802.1Q bridge VLANs** on the DSA bridge (`br-lan`). The MT7531AE switch handles VLAN tagging in hardware. All 4 LAN ports carry VLAN 1 (untagged) and VLAN 10 (tagged), making them 802.1Q trunk ports compatible with managed switches.

### VLAN Table

| VLAN ID | Device | Ports | Port Mode | Purpose |
|---------|--------|-------|-----------|----------|
| 1 | br-lan.1 | lan1–lan4 | `u*` (untagged, PVID) | LAN (default) |
| 10 | br-lan.10 | lan1–lan4 | `t` (tagged) | IoT (isolated) |
| 20 | br-lan.20 | lan1–lan4 | `t` (tagged) | Cameras / IoT (RouterF2) |

### Network Zones

| Network | Device | SSIDs | Firewall Zone | Internet | LAN Access |
|---------|--------|-------|---------------|----------|------------|
| LAN | br-lan.1 | Md Abdullah, Md Abdullah 5G | lan (accept all) | Yes | Full |
| IoT | br-lan.10 | Md Abdullah - IOT, Md Abdullah - IOT 5G | iot (reject input, reject forward) | Yes | **Blocked** |

**WiFi VLAN assignment**: netifd automatically adds WiFi interfaces to the bridge and assigns them to the correct VLAN based on their `option network` setting. No manual bridge-vlan port entries needed for WiFi.

**IoT isolation**: WiFi client isolation enabled (`isolate=1`). Firewall blocks IoT→LAN. DHCP and DNS allowed from IoT to router.

### Managed Switch Configuration

#### Netgear GS108Ev3 — VLAN Assignments (Example, not final)

> **Note**: This configuration was used to verify VLAN-based WAN isolation over a single hop. Final switch placement and port assignments are TBD.

| Port | Device | VLAN 1 | VLAN 10 | VLAN 50 | PVID | Role |
|------|--------|--------|---------|---------|------|------|
| 1 | ISP ONT | — | — | **U** | 50 | WAN ingress (isolated) |
| 2 | Cudy WAN | — | — | **U** | 50 | WAN egress (isolated) |
| 3 | Cudy LAN | **U** | **T** | — | 1 | Trunk (LAN + IoT) |
| 4–8 | Devices | **U** | — | — | 1 | LAN access |

**VLAN 50 (WAN isolation)**: Ports 1↔2 only. PPPoE traffic between ISP ONT and Cudy WAN is completely isolated from all other switch ports. Untagged on both sides — transparent to connected devices.

**Verified**: PPPoE WAN connectivity confirmed through VLAN 50 single-hop path.

#### General Switch Port Templates

For any managed switch on the LAN side:

| Switch Port | VLAN 1 | VLAN 10 | Connect To |
|-------------|--------|---------|------------|
| Uplink to Cudy | Untagged (PVID) | Tagged | Cudy LAN port |
| LAN devices | Untagged (PVID) | — | PCs, printers |
| IoT devices | — | Untagged (PVID=10) | Smart home, sensors |
| Trunk to other switch | Tagged | Tagged | Switch interconnect |

### DSA Configuration Notes

- **Do NOT** manually set `vlan_filtering=1` — netifd enables it automatically when `bridge-vlan` entries exist
- Interfaces must use `br-lan.<vid>` (e.g., `br-lan.1`, `br-lan.10`), NOT bare `br-lan`
- Using bare `br-lan` as interface device with bridge-vlan entries **will kill all connectivity**
- Always use a rollback timer when modifying VLAN config remotely

## Wireless Network

### Radio Configuration

| Parameter | radio0 (2.4GHz) | radio1 (5GHz) |
|-----------|------------------|----------------|
| Channel | **11** (2.462 GHz) | **149** (5.745 GHz) |
| Width | **HE40** (center ch 9) | **HE80** (center ch 155) |
| TX Power | 20 dBm (max) | 28 dBm (max) |
| Country | BD (Bangladesh) | BD (Bangladesh) |
| Cell Density | 1 (low, max range) | 1 (low, max range) |
| Noscan | Yes (2.4GHz only) | N/A |

### SSIDs

| SSID | Radio | Network | VLAN | Encryption | Client Isolation |
|------|-------|---------|------|------------|------------------|
| Md Abdullah | radio0 (2.4GHz) | lan | 1 (br-lan.1) | WPA2/WPA3 SAE-Mixed | No |
| Md Abdullah 5G | radio1 (5GHz) | lan | 1 (br-lan.1) | WPA2/WPA3 SAE-Mixed | No |
| Md Abdullah - IOT | radio0 (2.4GHz) | iot | 10 (br-lan.10) | WPA2/WPA3 SAE-Mixed | Yes |
| Md Abdullah - IOT 5G | radio1 (5GHz) | iot | 10 (br-lan.10) | WPA2/WPA3 SAE-Mixed | Yes |

### Channel Selection Rationale

**2.4GHz — Channel 11** chosen based on site survey (2026-02-20):
| Channel | Neighbor APs | Strongest Signal |
|---------|-------------|-----------------|
| 1 | Piyas | -62 dBm |
| 2 | Md Emdadul Haque, WhoaEpic, Sagor | -48 dBm |
| 4 | Imran | -78 dBm |
| 6 | Clear | — |
| **11** | **Clear** | — |
| 12 | WhoaEpic, Redmi Note 13 (hotspot) | -78 dBm |

Channel 11 selected over 6 because the HE40 secondary channel (below, ch 7-10) has no significant interference, whereas ch 6 with HE40 would overlap with the congested ch 2-4 neighbors.

**5GHz — Channel 149**: Only UNII-3 band (149-165) available under BD regulatory. Minimal interference (1 AP at -91 dBm). 80MHz block: 149-161.

### Theoretical Max Throughput
| Band | Width | Streams | MCS | Max PHY Rate |
|------|-------|---------|-----|-------------|
| 2.4GHz HE40 | 40MHz | 2x2 | MCS 11 | ~286 Mbps |
| 5GHz HE80 | 80MHz | 2x2 | MCS 11 | ~1201 Mbps |

## Internet Connectivity

- **Protocol**: PPPoE
- **WAN IP**: Dynamic (currently 10.135.15.29)
- **MTU**: 1492 (PPPoE standard)
- **IPv6**: wan6 (DHCPv6, auto — depends on ISP support)

<!-- TODO: Document ISP name, plan, bandwidth -->

## Firewall & Security

- OpenWrt default firewall (nftables) active
- NAT/masquerade: enabled on wan zone
- WiFi: WPA2/WPA3 SAE-Mixed (CCMP) on all SSIDs

### Firewall Zones

| Zone | Networks | Input | Output | Forward | Masquerade |
|------|----------|-------|--------|---------|------------|
| lan | lan | ACCEPT | ACCEPT | ACCEPT | No |
| wan | wan, wan6 | REJECT | ACCEPT | REJECT | Yes |
| iot | iot | REJECT | ACCEPT | REJECT | No |

### Forwarding Rules

| Source | Destination | Action |
|--------|-------------|--------|
| lan | wan | ACCEPT |
| iot | wan | ACCEPT |
| iot | lan | **REJECT** |

### IoT-specific Rules

| Rule | Source | Proto | Port | Action |
|------|--------|-------|------|--------|
| Allow-IoT-DHCP | iot | UDP | 67-68 | ACCEPT |
| Allow-IoT-DNS | iot | TCP/UDP | 53 | ACCEPT |
| Block-IoT-to-LAN | iot → lan | all | all | REJECT |

<!-- TODO: Outline additional firewall rules and access control policies -->

## NVR / Command & Control — Raspberry Pi 3B+

### Hardware

| Component | Detail |
|-----------|--------|
| **Model** | Raspberry Pi 3 Model B Plus Rev 1.3 |
| **Hostname** | cyg |
| **SoC** | BCM2837B0, Quad-core Cortex-A53 @ 1.4GHz (aarch64) |
| **RAM** | 905 MB (1 GB nominal) |
| **Boot disk** | 512 GB KIOXIA EXCERIA SATA SSD (via USB) — mounted at `/` |
| **NVR storage** | 2 TB Seagate ST2000VX017 HDD (via USB) — mounted at `/mnt/nvr` |
| **OS** | Raspberry Pi OS (Debian), kernel 6.12.62+rpt-rpi-v8 |
| **Network** | WiFi (wlan0) on VLAN 10, IP `192.168.10.250/24` |
| **Remote access** | Tailscale (`100.106.53.79`), SSH (`ssh admin@100.106.53.79`) |

### System Configuration Changes

#### 2026-03-03: Disabled Desktop Environment

**Purpose**: Free ~80 MB RAM for NVR (Shinobi) headless operation.

| Change | Command | Revert |
|--------|---------|--------|
| Set boot target to headless | `sudo systemctl set-default multi-user.target` | `sudo systemctl set-default graphical.target` |
| Stop LightDM (immediate) | `sudo systemctl stop lightdm` | `sudo systemctl start lightdm` |

**Impact**: SSH and Tailscale are unaffected (both are `multi-user.target` dependencies). Memory freed: ~80 MB (351 MB → 268 MB used). Desktop can be re-enabled at any time with the revert commands above.

**Verification**: After change, confirmed:
- Default target: `multi-user.target`
- SSH: active
- Tailscale: active
- LAN connectivity: OK
- Internet connectivity: OK
- Available RAM: 637 MB (up from 554 MB)

#### 2026-03-09: WiFi Connection Cleanup

**Purpose**: Remove conflicting NetworkManager WiFi connections that caused unreliable boot.

| Change | Command | Revert |
|--------|---------|--------|
| Deleted duplicate `Md Abdullah 5G` connection | `sudo nmcli con delete uuid 3543bc18-4c58-4eab-8802-53756b9af660` | Re-create if needed |
| Deleted stale `netplan-wlan0-Zigby` connection | `sudo nmcli con delete uuid 0973ccb4-b526-3131-988b-c9882962feb4` | Re-create if needed |
| Set autoconnect priority on `Md-Abdullah-5G` | `sudo nmcli con modify Md-Abdullah-5G connection.autoconnect-priority 100` | `sudo nmcli con modify Md-Abdullah-5G connection.autoconnect-priority 0` |

#### 2026-03-10: Enabled Persistent Journal

**Purpose**: Preserve boot logs across reboots for diagnostics. RPi OS defaults to volatile (RAM-only) journal via `/usr/lib/systemd/journald.conf.d/40-rpi-volatile-storage.conf`.

**Config**: Drop-in at `/etc/systemd/journald.conf.d/persistent.conf`:
```ini
[Journal]
Storage=persistent
SystemMaxUse=1G
SystemMaxFileSize=128M
MaxRetentionSec=15day
Compress=yes
```

| Limit | Value | Effect |
|-------|-------|--------|
| Max total size | 1 GB | Oldest entries purged when exceeded |
| Max file size | 128 MB | Rotates individual journal files |
| Max retention | 15 days | Entries older than 15 days deleted |

**Revert**: `sudo rm /etc/systemd/journald.conf.d/persistent.conf && sudo systemctl restart systemd-journald`

### Known Issues

#### Pi 3B+ Fails to Boot After `sudo reboot` (USB Boot Hang)

On warm reboot (`sudo reboot`), the Pi firmware resets the SoC but does NOT fully power-cycle the USB bus. The JMS583 USB-to-SATA bridge (SSD enclosure) fails to re-enumerate, so the firmware cannot find the boot partition. **Only a full power cycle recovers the system.**

- **Root cause**: Pi 3B+ dwc_otg USB controller + JMS583 bridge incompatibility on warm reset
- **Workaround**: Never use `sudo reboot`. Use `sudo poweroff` + physical power cycle, or configure a USB hub with per-port power switching.
- **Additional factor**: Under-voltage detected (`throttled=0xd0008`). PSU may be marginal for two 500mA USB devices. `max_usb_current=1` in config.txt is not set.

### VLAN 20 (IoT) Camera Access

The Pi on VLAN 10 can reach VLAN 20 (192.168.20.0/24) via inter-VLAN routing on RouterF2. Firewall rules on RouterF2 permit this traffic.

| IP | Hostname | Type | Ports | ONVIF |
|----|----------|------|-------|-------|
| 192.168.20.134 | Dahua-MainGate | Dahua DH-IPC-HDBW1230DE-SW | HTTP (80), RTSP (554) | Enabled |
| 192.168.20.162 | Hikvision-Cam1 | Hikvision DS-2CD1323G2-LIU | HTTP (80), HTTPS (443), RTSP (554) | Enabled |
| 192.168.20.179 | Hikvision-Cam2 | Hikvision DS-2CD1323G2-LIU | HTTP (80), HTTPS (443), RTSP (554) | Enabled |
| 192.168.20.198 | Reolink-Garage | Reolink E1 Outdoor | HTTP (80), RTSP (554), ONVIF (8000) | Enabled |
### Static DHCP Leases (RouterF2)

| Hostname | MAC | IP | VLAN | Device |
|----------|-----|-----|------|--------|
| RaspberryPi-CNC | b8:27:eb:0d:26:5b | 192.168.10.250 | 10 | Pi 3B+ (wlan0) |
| RaspberryPi-CNC-eth0 | b8:27:eb:58:73:0e | 192.168.10.251 | 10 | Pi 3B+ (eth0) |
| Hikvision-Cam1 | 84:94:59:9d:4f:71 | 192.168.20.162 | 20 | Hikvision DS-2CD1323G2-LIU |
| Hikvision-Cam2 | 84:94:59:a5:c2:69 | 192.168.20.179 | 20 | Hikvision DS-2CD1323G2-LIU |
| Reolink-Garage | ec:71:db:ed:ec:54 | 192.168.20.198 | 20 | Reolink E1 Outdoor |
| Dahua-MainGate | fc:b6:9d:e4:ee:b6 | 192.168.20.134 | 20 | Dahua DH-IPC-HDBW1230DE-SW |
## Monitoring & Management

- **LuCI Web UI**: http://192.168.1.1 (from LAN/WiFi)
- **SSH**: root@192.168.1.1 (from LAN/WiFi)

<!-- TODO: Describe monitoring tools and alerting setup -->

## Resident Access

<!-- TODO: Define onboarding process for new residents -->
<!-- TODO: Document acceptable use policy -->

## Maintenance & Support

- **Firmware**: OpenWrt 24.10.5 — check https://firmware-selector.openwrt.org/ for updates
- **Flash chip**: Original (serial 2512 < 2543 cutoff), compatible with all 24.10.x releases

<!-- TODO: List maintenance schedules and contact information -->

## Verified Experiments (2026-02-20)

Capabilities tested and confirmed working. These are **not active** in the current config but can be re-applied as needed.

### 1. Dual-WAN (LAN1 as WAN2)

**Verified**: Any LAN port can be repurposed as a second WAN port.

| Step | Command |
|------|---------|
| Remove lan1 from bridge | `uci del_list network.@device[0].ports='lan1'` |
| Remove from VLAN 1 | `uci del_list network.@bridge-vlan[0].ports='lan1:u*'` |
| Remove from VLAN 10 | `uci del_list network.@bridge-vlan[1].ports='lan1:t'` |
| Create wan2 interface | `uci set network.wan2=interface; uci set network.wan2.device='lan1'; uci set network.wan2.proto='pppoe'; uci set network.wan2.metric='20'` |
| Set wan metric | `uci set network.wan.metric='10'` |
| Add to firewall zone | `uci add_list firewall.@zone[1].network='wan2'` |

**Behavior**:
- With one ISP: Only the connected WAN carries traffic. The other stays down harmlessly.
- With two ISPs: Lower metric (wan=10) is preferred. No automatic failover without **mwan3**.
- **mwan3** package enables proper failover and/or load balancing.

**Reverted**: LAN1 restored to LAN role with full VLAN 1+10 trunk.

### 2. WAN Isolation via Managed Switch (VLAN 50)

**Verified**: PPPoE WAN traffic can be isolated through a managed switch using a dedicated VLAN.

| Switch Port | VLAN 50 | PVID | Device |
|-------------|---------|------|--------|
| Port 1 | Untagged | 50 | ISP ONT |
| Port 2 | Untagged | 50 | Cudy WAN |
| Ports 3–8 | Not member | 1 | LAN devices |

PPPoE frames travel exclusively between Port 1↔2 inside VLAN 50. All other switch ports cannot see WAN traffic. Both ports are access mode (untagged) — transparent to Cudy and ONT, no router-side changes needed.

### 3. DSA Bridge-VLAN Lessons Learned

| Mistake | Consequence | Fix |
|---------|-------------|-----|
| Setting `vlan_filtering=1` manually | Not needed — netifd does it automatically | Remove manual setting |
| Keeping `network.lan.device='br-lan'` after adding bridge-vlan entries | **All LAN/WiFi connectivity dies** | Must change to `br-lan.1` |
| Not including WiFi in bridge-vlan port lists | Not a problem — netifd auto-handles WiFi VLAN assignment | No action needed |
| Modifying bridge/VLAN config without rollback timer | Risk of permanent lockout | Always use: `(sleep 120 && cp backup && /etc/init.d/network restart) &` |
