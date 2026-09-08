---
title: Inside the Frazile media server
slug: frazile-media-server
summary: Jellyfin in front, Sonarr, Radarr, Lidarr and Readarr behind it, an indexer proxy, and two torrent clients that can only see the network through a VPN container. How the media half of the homelab fits together, and the decisions that keep it boring.
tags: homelab, jellyfin, media server, docker, vpn
days_ago: 0
---
The media server is the part of the homelab that other people in the house actually use, which makes it the part that is not allowed to be interesting. A film should play on the living-room TV, on a phone in another city, and on a laptop on hotel Wi-Fi, without anyone knowing what a container is. Here is the stack that makes that true, and why each piece is where it is.

![A collection of dismantled hard disk drives displayed on a white surface, showcasing internal components.](/media/2026/09/b7a96f63b7476f93.jpg)
*Photo by [Marta Branco](https://www.pexels.com/@martabranco) on [Pexels](https://www.pexels.com)*

## The shape of it

```
            ┌──────────────┐
  TV/phone ─┤   Jellyfin   │◄── reads media, read-only
            └──────────────┘
                    ▲
   library folders on the NAS pools (movies, shows, music, books)
                    ▲
┌────────┐ ┌────────┐ ┌────────┐ ┌─────────┐
│ Sonarr │ │ Radarr │ │ Lidarr │ │ Readarr │   ← decide what to fetch, rename, sort
└───┬────┘ └───┬────┘ └───┬────┘ └────┬────┘
    └──────────┴─────┬────┴───────────┘
              ┌──────▼──────┐
              │   Jackett   │  ← one place that talks to indexers
              └──────┬──────┘
         ┌───────────▼───────────┐
         │  gluetun (VPN)        │
         │  ├─ qBittorrent       │  ← download clients only exist inside the tunnel
         │  └─ Deluge            │
         └───────────────────────┘
```

Every box is its own container with its own compose file, all joined to a shared `frazilemedia` Docker network so they can address each other by name.

## Jellyfin: the only thing users see

Jellyfin serves the libraries and handles every client: the TV app, the phone apps, and the web UI through the tunnel for anyone outside the house. Three choices matter here:

- **Media is mounted read-only.** Jellyfin can never rename, move or delete a file. Bugs, plugins and mis-clicks cannot touch the library.
- **Hardware transcoding is on**, so a 4K HEVC file can be turned into something a phone on a weak connection can play without maxing the CPU. Direct play is still the goal; transcoding is the fallback.
- **Its database lives on the SSD**, not with the media. That move deserved [its own post](/p/jellyfin-database-ssd).

Users get their own accounts with per-library access, so the kids' profile sees the kids' library and nothing else.

## The arr stack: automation that stays out of the way

Sonarr (series), Radarr (films), Lidarr (music) and Readarr (books) do the same job for different media: keep a wanted list, watch for releases that match a quality profile, hand the download to a client, then rename and move the finished file into the library folder Jellyfin watches.

The part people underestimate is the **naming and folder structure**. Once every file lands as `Show Name/Season 02/Show Name - S02E05 - Title.mkv`, Jellyfin identifies it correctly on the first scan and never needs manual fixes. The arr apps are worth running for that alone, even if you added every file by hand.

Quality profiles are deliberately modest. 1080p for most things, 4K only for a short list of films that earn it, and a hard cap on file size per episode. Storage is finite and nobody can tell the difference on a phone.

## Jackett: one door to the indexers

Each arr app could be configured with every indexer separately, and would be, four times over. Jackett sits in the middle: indexers are configured once, and the arr apps each get a single Torznab endpoint. Adding or replacing an indexer is a change in one place.

## The VPN box: the rule that has no exceptions

The download clients run **inside the network namespace of a gluetun container**. In compose terms:

```yaml
services:
  gluetun:
    image: qmcgaw/gluetun
    cap_add: [NET_ADMIN]
    environment:
      VPN_SERVICE_PROVIDER: ...
      FIREWALL_OUTBOUND_SUBNETS: 172.16.0.0/12
    ports:
      - "8080:8080"   # qBittorrent web UI, published *by gluetun*

  qbittorrent:
    image: lscr.io/linuxserver/qbittorrent
    network_mode: "service:gluetun"
```

`network_mode: service:gluetun` means the torrent client has no network interface of its own. Every packet it sends goes through the tunnel or nowhere. If the VPN drops, gluetun's firewall blocks all traffic rather than failing open, and the client just stalls until the tunnel is back. There is no configuration on the torrent client that can undo this, which is the whole point.

The web UIs are published by the gluetun container, and the arr apps reach the clients through gluetun's name on the shared network. The `FIREWALL_OUTBOUND_SUBNETS` line is what lets the client talk back to the arr apps on the Docker network without leaking anything else.

## Storage

Media lives on large spinning disks in a few pools, mounted into the containers at consistent paths. The one rule that avoids most arr-stack grief: **downloads and the library must be on the same filesystem**, so the final "move" is an instant rename rather than a copy across disks. Hardlinks then let a file be seeded and be in the library at the same time without using space twice.

![Two friends enjoying a movie on a laptop with popcorn from a cozy couch perspective.](/media/2026/09/75e25e20797fe879.jpg)
*Photo by [Ron Lach](https://www.pexels.com/@ron-lach) on [Pexels](https://www.pexels.com)*

## Monitoring and notifications

- **Uptime Kuma** checks every web UI and Jellyfin's health endpoint. When something is down, it says which.
- **ntfy** is where everything reports: a finished download, a failed import, a container restart. One app on the phone, one topic per concern.
- Container logs are capped with the `json-file` driver limits so a chatty service cannot fill the system disk.

## What I would tell someone starting out

1. Start with Jellyfin and a well-named folder. Automation comes after you know what you want it to do.
2. Put the download clients behind a VPN container from day one. Retrofitting it is harder than starting there.
3. Mount media read-only into Jellyfin and read-write only where the arr apps need it.
4. Keep the database on fast storage and the media on cheap storage.
5. Write the healthcheck before the first outage, not after.

None of this is clever. That is what makes it the part of the homelab nobody has to think about.

---
*Cover photo by [freestocks.org](https://www.pexels.com/@freestocks) on [Pexels](https://www.pexels.com).*
