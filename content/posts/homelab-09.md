---
id: 15
title: Whisparr: the arr for content that needs its own rules
slug: whisparr-adult-library
summary: The arr family has a member for adult content. Running it responsibly is mostly about isolation: a separate library, a separate root, accounts that cannot see it, and metadata sources that behave. The technical notes, nothing else.
tags: homelab, whisparr, arr stack, jellyfin, privacy
days_ago: 0
---
Whisparr is a fork of the Sonarr/Radarr codebase aimed at adult content, with metadata sources that fit that world (studios and performers rather than networks and seasons). The interface is the one you already know; the thing that changes is how carefully it needs to be walled off from the rest of a shared media server.

This post is about that wall. It stays technical.

![Close-up of server racks in a data center highlighting modern technology infrastructure.](/media/2026/09/083d615bddc79881.jpg)
*Photo by [panumas nikhomkhai](https://www.pexels.com/@cookiecutter) on [Pexels](https://www.pexels.com)*

## Why a separate app at all

You could point Radarr at this content and it would mostly fail: TMDB does not index it, so nothing matches, nothing gets named, and Jellyfin shows a folder of hashes. Whisparr talks to sources that do index it, which means files get real titles, dates and artwork, and the library becomes browsable instead of a pile.

The other reason is the one that matters: keeping this library separate at every layer means the rest of the household never encounters it by accident.

## The container

```yaml
services:
  whisparr:
    image: ghcr.io/hotio/whisparr
    container_name: whisparr
    environment:
      PUID: 1000
      PGID: 1000
      TZ: Asia/Kolkata
    volumes:
      - ./config:/config
      - /mnt/media-restricted:/media     # NOT the shared media mount
    networks: [frazilemedia]
    restart: unless-stopped
```

The key decision is in the volumes. The restricted library lives on its **own mount**, not a subfolder of the shared one. The download client gets that mount at the same path so hardlinks work, but Sonarr, Radarr and the general Jellyfin libraries never see it.

## Isolation, layer by layer

**Filesystem.** Separate pool or dataset. Separate download category in qBittorrent (`whisparr`) with its own save path, so completed files never sit next to a TV episode.

**Jellyfin.** A dedicated library pointing at the restricted mount, with **no access for any account except one**. In *Dashboard → Users → (user) → Access*, uncheck the library for everyone else. Also turn off *Display missing episodes*, and set the library to not be included in *Latest Media* on the home screen, so it does not surface in the "recently added" rows even for the account that can see it.

**Metadata and images.** Whisparr fetches posters and fanart. Jellyfin stores its own copies under the library's metadata folder. Both of those directories are inside the restricted mount, and Jellyfin's image cache for that library is excluded from any backup that leaves the house.

**Network.** Same as every download client here: behind the VPN container, nothing else.

**Notifications.** Whisparr does **not** get the household ntfy topic. It has its own, with no previews in the notification text. An `imported: <title>` line appearing on a shared phone is exactly the kind of leak this whole setup exists to prevent.

**Search and indexing.** Jackett's indexers are shared, but the ones used for this library are tagged and assigned only to Whisparr, so Sonarr and Radarr do not search them.

## Settings that differ from Sonarr

- **Quality profiles** are simpler. Resolution matters; release groups mostly do not. One profile, 1080p cutoff, sensible size caps.
- **Naming** uses the studio and date rather than season and episode: `{Studio}/{Release Date} - {Title}`. Jellyfin's default metadata providers do nothing useful here, so the folder names *are* the browsing experience.
- **Monitoring** is per-performer or per-studio rather than per-series. Be conservative: monitoring a studio can mean hundreds of items. Add, do not auto-search, and pick by hand.

![Person using a TV remote control with a blurred television screen in the background.](/media/2026/09/974f522ed66e6126.jpg)
*Photo by [https://kaboompics.com/](https://www.pexels.com/@karola-g) on [Pexels](https://www.pexels.com)*

## Backups and the "what if" list

- The restricted mount is **excluded** from the off-site backup. If the disk dies, it is gone, and that is an acceptable trade.
- Whisparr's `config/` (which holds the database with titles and artwork paths) lives with the restricted data, not with the other app configs.
- The account that can see this library uses a **PIN** in the Jellyfin TV apps. Client apps that remember credentials on a shared device are the weakest link.
- A note in the homelab repo's README says which mounts are restricted and why, so a future rebuild does not accidentally attach one to the wrong container.

## Is it worth running?

If the content is in the house anyway, running it through the same automation as everything else, with the isolation above, is safer than a folder someone manages by hand. The tooling does the naming and sorting; the discipline is in the mounts and the Jellyfin permissions. Get those two right and it is just another quiet container.

---
*Cover photo by [Nathan Thomas](https://www.pexels.com/@nathanthomas) on [Pexels](https://www.pexels.com).*
