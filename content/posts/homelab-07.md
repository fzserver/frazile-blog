---
title: Sonarr: how I keep a TV library that fills itself
slug: sonarr-tv-library
summary: Sonarr watches for new episodes, grabs the version that matches your rules, and files it where Jellyfin expects it. The settings that matter, the ones that do not, and the habits that stop it from downloading the same show four times.
tags: homelab, sonarr, arr stack, jellyfin, docker
days_ago: 0
---
Of all the automation in the media stack, Sonarr is the piece that earns its keep every single week. A series is added once. From then on, every new episode shows up in Jellyfin a few hours after it airs, named correctly, in the right folder, without anyone touching anything.

Here is how it is set up, and what three years of running it have taught me.

![Minimalist desk calendar displaying the month of January with clean design.](/media/2026/09/b4fc887360574e68.jpg)
*Photo by [Matheus Bertelli](https://www.pexels.com/@bertellifotografia) on [Pexels](https://www.pexels.com)*

## What Sonarr actually does

1. You add a series and choose a **quality profile** and a **root folder**.
2. Sonarr pulls the episode list and air dates from TheTVDB and keeps a calendar.
3. When an episode is "wanted", it searches your **indexers** (through Jackett in my case) for a release that matches the profile.
4. It hands the release to a **download client** (qBittorrent, running behind the VPN container).
5. When the download completes, Sonarr **imports** it: renames the file to a clean pattern, hardlinks it into the library folder, and tells Jellyfin to rescan.

Every step is visible in the Activity and History tabs, which is the first place to look when something feels off.

## The container

```yaml
services:
  sonarr:
    image: lscr.io/linuxserver/sonarr:latest
    container_name: sonarr
    environment:
      PUID: 1000
      PGID: 1000
      TZ: Asia/Kolkata
    volumes:
      - ./config:/config
      - /mnt/media:/media          # library AND downloads on one filesystem
    networks: [frazilemedia]
    restart: unless-stopped
```

The single most important line is the media mount. **Downloads and the library must live under the same mount** so that importing is a hardlink, not a copy. With `/downloads` and `/tv` as separate mounts, Sonarr cannot hardlink, copies every file, and you pay for each episode twice on disk until seeding finishes.

Inside the container, the paths look like `/media/downloads/...` and `/media/tv/...`. The download client sees the same `/media` mount at the same path. Sonarr's **Remote Path Mapping** feature exists to fix mismatches, but the easiest fix is to never have any.

## Quality profiles worth having

I keep three:

| Profile | Cutoff | Used for |
|---|---|---|
| **HD** | WEB-DL 1080p | almost everything |
| **Any** | HDTV 720p | old shows that only exist in bad rips |
| **UHD** | WEB-DL 2160p | a handful of shows where it matters |

"Cutoff" means Sonarr stops upgrading once it has that quality. Without a sensible cutoff it will happily replace a fine 1080p file with a 40 GB remux the moment one appears.

**Size limits per quality** live in *Settings → Quality*. Capping 1080p episodes at around 4 GB per hour keeps the ridiculous encodes out without needing custom formats.

## Custom formats, but only a few

Custom formats let you score releases by properties: codec, release group, language, HDR type. It is easy to build a huge scoring system. Mine has four rules:

- **Prefer x265/HEVC** for anything above 720p (smaller files, hardware decoded on every client here).
- **Reject** anything tagged as a cam, screener or upscaled.
- **Reject** releases that are not in English or do not include English audio.
- **Prefer** a short list of release groups that are consistently good.

That is enough to get good files without turning release selection into a hobby.

## Naming

*Settings → Media Management → Episode Naming.* Rename episodes: on. The format:

```
{Series TitleYear}/Season {season:00}/{Series TitleYear} - S{season:00}E{episode:00} - {Episode CleanTitle} [{Quality Full}]
```

The year in the series folder is what stops two shows with the same title from colliding, which happens more often than you would think with reboots.

## The habits that prevent duplicates

- **One root folder per profile type**, not per genre. Sonarr does not care about genres; Jellyfin sorts that out.
- **Monitor only what you want.** When adding a series, choose "future episodes" for something you are catching up on elsewhere, or "all" only if you actually want the back catalogue.
- **Let Sonarr do the moving.** If you drop files into the library by hand, use *Manual Import* so Sonarr knows about them. Otherwise it will download the episode again because, as far as it knows, it is still missing.
- **Set up the recycle bin.** *Settings → Media Management → Recycling Bin* keeps replaced files for a few days, so an upgrade that turns out to be worse can be undone.

![A collection of dismantled hard disk drives displayed on a white surface, showcasing internal components.](/media/2026/09/bb436b4046a66535.jpg)
*Photo by [Marta Branco](https://www.pexels.com/@martabranco) on [Pexels](https://www.pexels.com)*

## Where it talks to the rest

- Jackett provides the Torznab feeds. Sonarr tests each with one click.
- qBittorrent is reached through the gluetun container's name; Sonarr never sees the VPN.
- **Connect → Jellyfin** triggers a library refresh on import, so new episodes appear within a minute.
- **Connect → ntfy** (via a webhook) posts a line to the phone when an episode is imported or a download fails. The failure ones are the ones worth reading.

## If I could only tell someone one thing

Get the paths right before you add a single show. Everything else can be changed later; a library that was imported by copying is a chore to fix.

---
*Cover photo by [Jakub Zerdzicki](https://www.pexels.com/@jakubzerdzicki) on [Pexels](https://www.pexels.com).*
