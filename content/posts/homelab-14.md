---
title: Deluge as a second client: why the stack has two torrent clients
slug: deluge-second-client
summary: qBittorrent handles the automated queue. Deluge handles everything a human starts by hand, on its own category, its own limits and its own seeding rules. Why splitting them was worth a second container, and how Deluge is set up behind the same VPN.
tags: homelab, deluge, vpn, docker, arr stack
days_ago: 0
---
For a long time there was one torrent client, and every download went through it: the arr apps' automated grabs, plus anything a person added by hand. That worked until it did not. A manually added torrent would land in the wrong category, get picked up by Radarr's import, and either fail loudly or get renamed into the film library as something it was not. Ratio rules meant for TV episodes were applied to Linux ISOs. Speed limits tuned for overnight batches throttled a download someone was waiting for.

The fix was not more configuration. It was a second client.

![Detailed view of a black data storage unit highlighting modern technology and data management.](/media/2026/09/3aff8dab70f900ac.jpg)
*Photo by [Jakub Zerdzicki](https://www.pexels.com/@jakubzerdzicki) on [Pexels](https://www.pexels.com)*

## The split

| | **qBittorrent** | **Deluge** |
|---|---|---|
| Who adds torrents | Sonarr, Radarr, Lidarr, Readarr | people |
| Save path | `/media/downloads/<category>` | `/media/manual` |
| Seeding rule | ratio 2 or 14 days | ratio 1, then stop |
| Speed limits | scheduled, conservative | none by default |
| Watched by arr apps | yes | **never** |
| Network | gluetun namespace | gluetun namespace |

Both live inside the same VPN container, so the protection is identical. Everything else differs, and because they are separate processes with separate configs, the differences cannot bleed into each other.

## The container

```yaml
services:
  deluge:
    image: lscr.io/linuxserver/deluge:latest
    container_name: deluge
    network_mode: "service:gluetun"
    depends_on:
      gluetun: { condition: service_healthy }
    environment:
      PUID: 1000
      PGID: 1000
      TZ: Asia/Kolkata
      DELUGE_LOGLEVEL: warning
    volumes:
      - ./config:/config
      - /mnt/media:/media
    restart: unless-stopped
```

And on the gluetun side, two more published ports:

```yaml
    ports:
      - "127.0.0.1:8080:8080"   # qBittorrent
      - "127.0.0.1:8112:8112"   # Deluge web UI
      - "127.0.0.1:58846:58846" # Deluge daemon, for thin clients
```

Deluge's daemon (`deluged`) and web UI are separate processes, which is one of the reasons to keep it around: the desktop **thin client** connects to the daemon directly, giving a native app with drag-and-drop, columns and per-torrent file selection that no web UI quite matches. Enable remote connections in `core.conf` (`"allow_remote": true`) and add a user to `auth` for that.

## Settings worth changing

**Downloads**: download to `/media/manual/incomplete`, move completed to `/media/manual`. The arr apps have no root folder anywhere near it.

**Bandwidth**: global limits off, per-torrent limits on (a few MB/s each), so one large download does not starve the others. Max connections around 200; Deluge's defaults are low.

**Queue**: seeding stops at ratio 1.0, remove torrent (keep data) on completion of that. This is what keeps `/media/manual` from becoming a graveyard.

**Plugins**: two are worth enabling.
- **Label**: lets a person tag a torrent "iso", "archive", "watch-later" and have each label go to its own folder.
- **AutoAdd**: watches a folder for `.torrent` files. A shared folder on the NAS that anyone in the house can drop a file into, and it starts downloading. This is the feature that made the second client popular with people who will never open a web UI.

**Interface**: change the default web UI password (`deluge`) on first run.

## What the arr apps think about it

Nothing. Deluge is not configured as a download client in any of them. The one time a manually started series was added to Sonarr later, the files were imported with Sonarr's *Manual Import* tool from the Deluge folder, and Sonarr took it from there.

![Person in casual attire using laptop on cozy grey sofa, creating a home workspace vibe.](/media/2026/09/506705715a956811.jpg)
*Photo by [https://kaboompics.com/](https://www.pexels.com/@karola-g) on [Pexels](https://www.pexels.com)*

## Why not one client with two profiles?

qBittorrent could be made to behave like this with categories, per-category limits and some discipline. Discipline is the part that does not survive contact with other users. Two clients means the rules are structural: a torrent added through the AutoAdd folder physically cannot end up in Radarr's import path. That is worth a container that uses about 60 MB of memory.

## The checklist

- Both clients inside the VPN namespace, both ports published by gluetun.
- Separate save paths, no overlap with any arr root folder.
- Separate seeding rules matched to the content.
- Deluge's AutoAdd folder shared on the NAS for the household.
- Web UI passwords changed, thin-client access only over the local network.

---
*Cover photo by [Tranmautritam](https://www.pexels.com/@tranmautritam) on [Pexels](https://www.pexels.com).*
