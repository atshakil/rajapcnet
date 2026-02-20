# Rajshahi Apartment Complex 1 - Network

Primary router running OpenWrt 24.10.5 on Cudy WR3000S v1 with PPPoE WAN and dual-band WiFi 6.

## Network Topology

```
ISP ONT ──► [WAN] Cudy WR3000S v1 [LAN 1-4] ──► Managed Switches
                   │                                    (VLAN 1 untagged + VLAN 10 tagged)
                   │
                   ├── br-lan.1  (192.168.1.0/24)  — LAN (VLAN 1)
                   │   ├── 2.4GHz: "Md Abdullah"
                   │   └── 5GHz:   "Md Abdullah 5G"
                   │
                   └── br-lan.10 (192.168.10.0/24) — IoT (VLAN 10)
                       ├── 2.4GHz: "Md Abdullah - IOT"
                       └── 5GHz:   "Md Abdullah - IOT 5G"
```

<!-- TODO: Add managed switches and additional APs to topology -->

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
- TP-Link TL-SG105E 5-port (~1800 BDT)
- Netgear GS108E 8-port (~4200 BDT)

## IP Addressing Scheme

| Subnet | Device | VLAN | Purpose | Gateway | DHCP Range |
|--------|--------|------|---------|---------|------------|
| 192.168.1.0/24 | br-lan.1 | 1 (untagged) | LAN (default) | 192.168.1.1 | 192.168.1.100 – 192.168.1.249 |
| 192.168.10.0/24 | br-lan.10 | 10 (tagged) | IoT (isolated) | 192.168.10.1 | 192.168.10.100 – 192.168.10.249 |

- DNS: 1.1.1.1, 8.8.8.8 (or ISP-provided)

<!-- TODO: Define additional subnets for management, guests -->

## Network Segmentation (802.1Q VLANs via DSA)

IoT isolation is implemented via **802.1Q bridge VLANs** on the DSA bridge (`br-lan`). The MT7531AE switch handles VLAN tagging in hardware. All 4 LAN ports carry VLAN 1 (untagged) and VLAN 10 (tagged), making them 802.1Q trunk ports compatible with managed switches.

### VLAN Table

| VLAN ID | Device | Ports | Port Mode | Purpose |
|---------|--------|-------|-----------|----------|
| 1 | br-lan.1 | lan1–lan4 | `u*` (untagged, PVID) | LAN (default) |
| 10 | br-lan.10 | lan1–lan4 | `t` (tagged) | IoT (isolated) |

### Network Zones

| Network | Device | SSIDs | Firewall Zone | Internet | LAN Access |
|---------|--------|-------|---------------|----------|------------|
| LAN | br-lan.1 | Md Abdullah, Md Abdullah 5G | lan (accept all) | Yes | Full |
| IoT | br-lan.10 | Md Abdullah - IOT, Md Abdullah - IOT 5G | iot (reject input, reject forward) | Yes | **Blocked** |

**WiFi VLAN assignment**: netifd automatically adds WiFi interfaces to the bridge and assigns them to the correct VLAN based on their `option network` setting. No manual bridge-vlan port entries needed for WiFi.

**IoT isolation**: WiFi client isolation enabled (`isolate=1`). Firewall blocks IoT→LAN. DHCP and DNS allowed from IoT to router.

### Managed Switch Configuration

All LAN ports are trunk ports (VLAN 1 untagged + VLAN 10 tagged). Configure managed switches accordingly:

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
- **WAN IP**: Dynamic (currently 10.135.14.234)
- **MTU**: 1492 (PPPoE standard)
- **IPv6**: wan6 (DHCPv6, auto — depends on ISP support)

<!-- TODO: Document ISP name, plan, bandwidth -->
<!-- TODO: Describe failover or load-balancing setup if applicable -->

## Firewall & Security

- OpenWrt default firewall (nftables) active
- NAT/masquerade: enabled on wan
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
