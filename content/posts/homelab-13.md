---
title: qBittorrent behind a VPN: the download client that cannot leak
slug: qbittorrent-behind-vpn
summary: qBittorrent is the workhorse client for the arr stack. Running it inside a VPN container's network namespace, with categories the arr apps understand and settings that keep seeding from eating the disk. The full configuration, and the two mistakes that quietly break it.
tags: homelab, qbittorrent, vpn, docker, arr stack
days_ago: 0
---
Every download in the media stack goes through qBittorrent, and qBittorrent goes through a VPN. Not "uses a VPN": the client has no network interface of its own. This post is the whole setup, from the compose file to the settings inside the web UI, and the reasoning behind each piece.

![Detailed view of blue ethernet cables connected to a network switch in a data center.](/media/2026/09/625b5a915d6db441.jpg)
*Photo by [Brett Sayles](https://www.pexels.com/@brett-sayles) on [Pexels](https://www.pexels.com)*

## The container pair

```yaml
services:
  gluetun:
    image: qmcgaw/gluetun
    container_name: gluetun
    cap_add: [NET_ADMIN]
    devices: [/dev/net/tun]
    environment:
      VPN_SERVICE_PROVIDER: ${VPN_PROVIDER}
      VPN_TYPE: wireguard
      WIREGUARD_PRIVATE_KEY: ${WG_KEY}
      SERVER_COUNTRIES: Netherlands
      FIREWALL_OUTBOUND_SUBNETS: 172.16.0.0/12
      FIREWALL_VPN_INPUT_PORTS: 6881
      HEALTH_VPN_DURATION_INITIAL: 30s
    ports:
      - "127.0.0.1:8080:8080"     # qBittorrent web UI, published by gluetun
    networks: [frazilemedia]
    restart: always

  qbittorrent:
    image: lscr.io/linuxserver/qbittorrent:latest
    container_name: qbittorrent
    network_mode: "service:gluetun"
    depends_on:
      gluetun: { condition: service_healthy }
    environment:
      PUID: 1000
      PGID: 1000
      TZ: Asia/Kolkata
      WEBUI_PORT: 8080
    volumes:
      - ./config:/config
      - /mnt/media:/media
    restart: unless-stopped
```

Three lines carry all the weight:

- `network_mode: "service:gluetun"` gives qBittorrent gluetun's network stack. There is no other route out.
- `FIREWALL_OUTBOUND_SUBNETS` lets qBittorrent answer requests *from* the Docker network, which is how Sonarr and Radarr talk to it. Without it the arr apps can send torrents but never get status back.
- `ports` is on **gluetun**, not on qBittorrent. A container in another container's namespace cannot publish ports itself.

If the VPN drops, gluetun's firewall blocks everything. qBittorrent's transfers simply stall until the tunnel comes back. No setting inside qBittorrent can bypass that, which is why the same protection is not done with qBittorrent's own "bind to interface" option: that one is a setting, and settings get reset.

## The two mistakes that break it quietly

**1. Restarting gluetun without restarting qBittorrent.** When gluetun is recreated, its network namespace is new, and the qBittorrent container is still attached to the old one. Nothing errors; qBittorrent just has no network. `depends_on` with a health condition handles the boot order, but a manual `docker compose up -d` on gluetun alone needs a follow-up restart of the client. A small wrapper script restarts both together.

**2. Path mismatches with the arr apps.** qBittorrent saves to `/media/downloads/tv`; Sonarr must see the finished file at exactly `/media/downloads/tv` too. Same mount, same path, in every container. The moment one container mounts the download folder at a different path, imports fail with "path does not exist" and the fix is a remote path mapping nobody remembers a year later.

## Web UI settings that matter

**Downloads**
- Default save path: `/media/downloads`
- **Torrent content layout: Original**, so the arr apps see the structure they expect.
- Pre-allocate disk space: on (avoids fragmentation on the HDD pool).
- Keep incomplete torrents in: `/media/downloads/incomplete`, so a half-finished file is never imported.

**Connection**
- Listening port: 6881, matched with `FIREWALL_VPN_INPUT_PORTS` on gluetun. Most VPN providers do not forward ports, so incoming connections are rare; the port still needs to be open in gluetun's firewall for the ones that arrive.
- UPnP / NAT-PMP: **off**. Nothing should be opening ports on the router.

**Speed**
- Global upload limit set to about a third of the upstream bandwidth, so a busy seeding day does not stall video calls.
- Alternative rate limits scheduled for daytime hours.

**BitTorrent**
- Seeding limits: **ratio 2 or 14 days, then pause**. Seeding is the polite part; unlimited seeding is how a disk fills with things nobody watches.
- Encryption: prefer.
- Anonymous mode: on.

**Categories** are the contract with the arr apps. One per app: `tv`, `movies`, `music`, `books`, each with its own save path under `/media/downloads/`. Sonarr uses category `tv`, Radarr `movies`, and so on. Categories are also how the client knows to keep a completed torrent seeding after the arr app has hardlinked it into the library.

## Authentication

The web UI is only reachable on localhost and through the tunnel, so the built-in login is the second layer, not the first. Still: change the default password on first boot, and enable **"Bypass authentication for clients on localhost"** for the arr apps only if they actually connect from the same namespace, which in this layout they do not, so it stays off. Sonarr and Radarr use a dedicated qBittorrent user with the password stored in their own config.

![From above of modern portable computer with open analytical program on screen on white table](/media/2026/09/ceddb98af8c3fb22.jpg)
*Photo by [Василь Вовк](https://www.pexels.com/@2874318) on [Pexels](https://www.pexels.com)*

## Proving the VPN works

Every few days a job runs:

```sh
docker exec qbittorrent curl -s https://ipinfo.io/ip
```

and compares the result against the home IP. If they ever match, ntfy fires a high-priority alert and a script pauses all torrents. It has never triggered. It exists so that "never" is a measured fact rather than an assumption.

## Deluge?

There is a second client, Deluge, in the same layout. Why two is [its own post](/p/deluge-second-client).

---
*Cover photo by [Dan  Nelson](https://www.pexels.com/@dan-nelson-1667453) on [Pexels](https://www.pexels.com).*
