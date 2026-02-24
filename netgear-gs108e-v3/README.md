# Netgear GS108Ev3 — Switch Configuration

**Location:** 2nd Floor, between ISP ONU and Cudy WR3000S (RouterF2)  
**Management IP:** 192.168.0.239 (factory default) on VLAN 1  
**Management URL:** http://192.168.0.239 (password: `password`)  
**Last updated:** 2026-02-24

---

## Network Overview

```
┌─────────┐                 ┌─────────────────┐                ┌────────────────┐
│ ISP ONU │── Port 1 ──────▶│  GS108Ev3       │── Port 2 ────▶│  RouterF2      │
│ (PPPoE) │   PVID 50       │  (this switch)  │   trunk        │  Cudy WR3000S  │
└─────────┘                 │                 │                │                │
                            │  Port 3-4: IoT  │                │  LAN1 ← trunk  │
                            │  Port 5-7: LAN  │                │  LAN2 ← Starlink│
                            │  Port 8:  Mgmt  │                └────────────────┘
                            └─────────────────┘
```

---

## VLANs

| VLAN | Name        | Subnet            | Purpose                            |
|------|-------------|-------------------|------------------------------------|
| 1    | Management  | 192.168.0.0/24    | Switch/router management only      |
| 10   | LAN         | 192.168.10.0/24   | General devices, WiFi clients      |
| 20   | IoT         | 192.168.20.0/24   | IP cameras, smart home devices     |
| 50   | ISP         | —                 | PPPoE passthrough to router WAN    |

> VLAN 51 (Starlink) is **not on this switch** — it connects directly to router LAN2.

---

## Port Assignments

| Port | Label        | Mode   | PVID | Tagged VLANs       | Untagged VLAN | Device              |
|------|-------------|--------|------|--------------------|--------------|--------------------|
| 1    | ISP Uplink  | Access | 50   | —                  | 50           | ISP fiber ONU       |
| 2    | Router Trunk| Trunk  | 1    | 1, 10, 20, 50     | —            | RouterF2 LAN1       |
| 3    | IoT         | Access | 20   | —                  | 20           | IP camera / IoT     |
| 4    | IoT         | Access | 20   | —                  | 20           | IP camera / IoT     |
| 5    | LAN         | Access | 10   | —                  | 10           | General device      |
| 6    | LAN         | Access | 10   | —                  | 10           | General device      |
| 7    | LAN         | Access | 10   | —                  | 10           | General device      |
| 8    | Emergency   | Access | 1    | —                  | 1            | Mgmt laptop (break-glass) |

---

## How to Configure (Web UI)

### Step 1 — Connect to the switch

1. Plug your laptop into **Port 8** (or any port before config)
2. Set laptop IP to `192.168.0.100 / 255.255.255.0`
3. Open http://192.168.0.239 → login with `password`

### Step 2 — Create VLANs

Go to **VLAN → 802.1Q → Advanced → VLAN Configuration**

Add these VLAN IDs (one at a time, click Add each time):

| VLAN ID | Name |
|---------|------|
| 10      | LAN  |
| 20      | IoT  |
| 50      | ISP  |

> VLAN 1 already exists by default.

### Step 3 — VLAN Membership

Go to **VLAN → 802.1Q → Advanced → VLAN Membership**

Select each VLAN ID and set port membership:

**VLAN 1 (Management):**

| Port | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 |
|------|---|---|---|---|---|---|---|---|
| Mode | — | T | — | — | — | — | — | U |

**VLAN 10 (LAN):**

| Port | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 |
|------|---|---|---|---|---|---|---|---|
| Mode | — | T | — | — | U | U | U | — |

**VLAN 20 (IoT):**

| Port | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 |
|------|---|---|---|---|---|---|---|---|
| Mode | — | T | U | U | — | — | — | — |

**VLAN 50 (ISP):**

| Port | 1 | 2 | 3 | 4 | 5 | 6 | 7 | 8 |
|------|---|---|---|---|---|---|---|---|
| Mode | U | T | — | — | — | — | — | — |

Legend: **T** = Tagged, **U** = Untagged, **—** = Not a member

### Step 4 — Port PVIDs

Go to **VLAN → 802.1Q → Advanced → Port PVID**

| Port | PVID |
|------|------|
| 1    | 50   |
| 2    | 1    |
| 3    | 20   |
| 4    | 20   |
| 5    | 10   |
| 6    | 10   |
| 7    | 10   |
| 8    | 1    |

### Step 5 — Apply

Click **Apply** on each page. The switch will save to NVRAM.

---

## Quick Reference Card

```
 ┌─────────────────────────────────────────────────────┐
 │          NETGEAR GS108Ev3 — REAR PANEL              │
 │                                                      │
 │  ┌──┐  ┌──┐  ┌──┐  ┌──┐  ┌──┐  ┌──┐  ┌──┐  ┌──┐  │
 │  │P1│  │P2│  │P3│  │P4│  │P5│  │P6│  │P7│  │P8│  │
 │  └──┘  └──┘  └──┘  └──┘  └──┘  └──┘  └──┘  └──┘  │
 │   ISP   RTR   CAM   CAM   LAN   LAN   LAN   MGMT  │
 │  VL50  TRUNK  VL20  VL20  VL10  VL10  VL10  VL1   │
 └─────────────────────────────────────────────────────┘
```

---

## Corresponding Router Config

The router (RouterF2, Cudy WR3000S) expects these VLANs on its **LAN1** port:

| Router Interface | VLAN | Subnet          | Service      |
|-----------------|------|-----------------|-------------|
| br-lan.1        | 1    | 192.168.0.1/24  | Management   |
| br-lan.10       | 10   | 192.168.10.1/24 | LAN + WiFi   |
| br-lan.20       | 20   | 192.168.20.1/24 | IoT          |
| br-lan.50       | 50   | —               | PPPoE WAN    |

Full router config: `../cudy-wr3000s-v1-2nd-floor/configure-routerf2.sh`

---

## Troubleshooting

**Can't reach switch management UI?**
- Plug into Port 8, set laptop to `192.168.0.100/24`
- Browse to http://192.168.0.239
- If switch IP was changed, do a factory reset: hold reset button 7+ seconds

**Device on Port 5-7 can't get IP?**
- Verify PVID is set to 10 for that port
- Verify port is Untagged member of VLAN 10
- Check router DHCP on VLAN 10: `ssh root@192.168.10.1 'cat /tmp/dhcp.leases'`

**Camera on Port 3-4 can't connect?**
- Verify PVID is set to 20 for that port
- Verify port is Untagged member of VLAN 20
- IoT devices get IPs from 192.168.20.100–249

**ISP internet down?**
- Check ONU link light on Port 1
- Verify PPPoE on router: `ssh root@192.168.10.1 'ifstatus wan | jsonfilter -e @.up'`

**Factory reset the switch:**
- Hold the reset button on the front panel for 7+ seconds until all LEDs blink
- All VLANs will be cleared, all ports become untagged VLAN 1, PVID 1
