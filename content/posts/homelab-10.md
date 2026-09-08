---
title: Lidarr: music automation that respects a curated library
slug: lidarr-music-library
summary: Lidarr can pull an artist's entire discography the moment you add them, which is exactly what you do not want. How I run it for a library that was tagged by hand over fifteen years, and the metadata settings that stop it from rewriting the past.
tags: homelab, lidarr, arr stack, music, jellyfin
days_ago: 0
---
Music is the media type where automation has to be most polite. A film library is disposable; a music library is usually the product of years of ripping, tagging and sorting, and one careless import can undo an afternoon of work. Lidarr is capable of doing that. It is also capable of being a quiet assistant that just fills in the gaps. The difference is entirely in the settings.

![Minimalist image of black over-ear headphones on a white background.](/media/2026/09/95ba5ae6ada082ce.jpg)
*Photo by [Aleksandar Spasojevic](https://www.pexels.com/@spal) on [Pexels](https://www.pexels.com)*

## What Lidarr is for

Lidarr tracks **artists** and their **releases** (albums, EPs, singles) using MusicBrainz as the source of truth. For each monitored release it searches indexers, sends the result to the download client, then imports and renames the tracks into your library. It also handles **upgrades**: replacing a 128 kbps rip with a lossless one when it turns up.

The MusicBrainz dependency is the thing to understand first. If an album is not in MusicBrainz, Lidarr does not know it exists. For mainstream releases this is a non-issue; for bootlegs, local bands and your own recordings, Lidarr should never touch those folders at all.

## The container

```yaml
services:
  lidarr:
    image: lscr.io/linuxserver/lidarr:latest
    container_name: lidarr
    environment:
      PUID: 1000
      PGID: 1000
      TZ: Asia/Kolkata
    volumes:
      - ./config:/config
      - /mnt/media:/media
    networks: [frazilemedia]
    restart: unless-stopped
```

The root folder is `/media/music`. The important part is what is **not** in that folder: everything that was ripped or bought and tagged by hand lives in `/media/music-archive`, which Lidarr has never been pointed at. Two folders, two Jellyfin libraries merged into one view, and no way for automation to wander into the curated half.

## Monitoring: the setting that saves you from yourself

When you add an artist, Lidarr asks what to monitor. The default is **all albums**, which for a prolific artist means dozens of releases, compilations and live albums arriving overnight.

What I use:

- **Monitor: None** on add, then tick the specific albums I actually want.
- **Future albums: yes** for artists I follow, so a new release is grabbed on release day.
- **Metadata profile** set to *Standard* with **compilations, live, remix and demo** types unchecked. This is in *Settings → Profiles → Metadata Profiles*, and it is the single change that turns Lidarr from a hoarder into a curator.

## Quality profile

Two profiles:

| Profile | Allowed | Cutoff |
|---|---|---|
| **Lossless** | FLAC, ALAC | FLAC |
| **Portable** | MP3 320, AAC 256, FLAC | MP3 320 |

Most artists sit on *Lossless*. The *Portable* profile is for things that will only ever be heard in a car.

Lidarr can also be told to **prefer** a release with a specific number of tracks or a specific country of release, which matters for albums with regional bonus tracks. I leave that alone; MusicBrainz's "official" release usually wins.

## Tagging: let Lidarr write tags, but only its own

*Settings → Metadata* has a **Write Metadata to Audio Files** option. For files that Lidarr imports, I let it write tags and embed cover art, because those files are arriving untagged or badly tagged anyway. Because the archive folder is outside Lidarr's root, its careful hand-written tags are never rewritten.

Scrub existing tags: **off**. If a release arrives with useful tags Lidarr does not know about (composer, lyrics), they stay.

## Naming

```
{Artist Name}/{Album Title} ({Release Year})/{track:00} - {Track Title}
```

Multi-disc albums get `Disc {medium:00}` inserted before the track number automatically when the release has more than one medium. Jellyfin reads this structure cleanly, and so does every other player I have tried.

## Where it struggles

- **Classical music.** MusicBrainz models works, performers and conductors; Lidarr flattens it to artist and album. Classical stays in the archive folder.
- **Various-artists compilations.** Lidarr handles them, but the naming needs a separate format with `{Album Artist}` in the path, or you get a folder per contributing artist with one track each.
- **Indexer coverage.** Music indexers are thinner than TV and film. Expect more "not found" for anything outside the charts.

![A cluttered pile of CD cases showcasing a nostalgic collection in a dimly lit room.](/media/2026/09/2c1c6c970ce182a2.jpg)
*Photo by [Eylül Kuşdili](https://www.pexels.com/@eylulkusdili) on [Pexels](https://www.pexels.com)*

## Connections

Jackett for indexers with only the music-capable ones assigned, qBittorrent behind the VPN container, and a Jellyfin library refresh on import. Lidarr's calendar feeds a small "new music this week" ntfy message on Fridays, which is the only automated notification I look forward to.

## The principle

Automation gets a folder of its own and clear instructions about what to want. The curated library stays curated. Lidarr is very good at its job when the job is defined that narrowly.

---
*Cover photo by [Magda Ehlers](https://www.pexels.com/@magda-ehlers-pexels) on [Pexels](https://www.pexels.com).*
