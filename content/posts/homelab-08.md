---
title: Radarr: a film library on rules instead of impulse
slug: radarr-film-library
summary: Radarr is Sonarr for films, but films are not episodes: no schedule, one file, and the whole decision rests on quality profiles and what you are willing to wait for. The setup that keeps my library small and watchable.
tags: homelab, radarr, arr stack, jellyfin, docker
days_ago: 0
---
Radarr and Sonarr share a codebase and a design, so if you have set up one, the other feels familiar. The difference is in the content. A series drips new episodes on a schedule. A film is one file, released once, and the interesting question is not *when* but *which version*: the early web rip, the proper release two months later, or the 4K disc a year on.

Radarr's job is to answer that question according to rules you set once.

![Glass bowl filled with popcorn next to TV remotes on a gray sofa. Perfect for cozy movie nights.](/media/2026/09/3c34d879f80222da.jpg)
*Photo by [Srattha Nualsate](https://www.pexels.com/@srattha-nualsate-2695613) on [Pexels](https://www.pexels.com)*

## The setup

```yaml
services:
  radarr:
    image: lscr.io/linuxserver/radarr:latest
    container_name: radarr
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

Same rule as Sonarr: one mount that contains both the download folder and the film library, so imports are hardlinks. Root folder is `/media/movies`.

## Availability: the setting people miss

When you add a film, Radarr asks for its **minimum availability**:

- **Announced**: search as soon as the film exists on TMDB. You will download trailers and fakes.
- **In Cinemas**: search from the theatrical release. You will download cam recordings.
- **Released**: search from the digital or disc release date. This is the correct answer for almost everyone.

I set the default to *Released* and never think about it again. Films appear in Jellyfin the week they come to streaming, which is when a decent copy exists.

## Quality profile: pick the cutoff on purpose

Films are large, and a profile without a cutoff will keep upgrading forever. My default profile:

- Allowed: WEB-DL 1080p, Bluray 1080p, WEB-DL 2160p, Bluray 2160p
- **Cutoff: Bluray 1080p**
- Upgrade until cutoff: yes

So a film arrives as a 1080p web rip on release week and quietly upgrades to the disc rip when one appears, then stops. A second profile, *UHD*, exists for the short list of films where I want 4K HDR on the living-room TV.

Size limits are stricter than for TV: a 1080p film is capped at roughly 12 GB. Anything larger is a remux, and remuxes are for people with a lot more storage than me.

## Custom formats that matter for films

- **HDR vs SDR**: score HDR10 and Dolby Vision *up* in the UHD profile and *down* in the default one, because a 1080p HDR file looks wrong on an SDR screen.
- **Audio**: prefer releases with a surround track; reject releases with only commentary or foreign audio.
- **Reject** cams, telesyncs, screeners and "upscaled 4K".
- **Prefer** the same handful of release groups as Sonarr.

TRaSH Guides publish maintained custom formats and profiles that many people import wholesale. They are excellent, and also far more than a home library needs. I imported the rejection rules and wrote the rest by hand.

## Lists: the part that makes it feel automatic

Radarr can subscribe to **lists** and add everything on them. A few that have worked well:

- A personal **Trakt** watchlist: add a film on the phone, it appears in Radarr within an hour, and in Jellyfin when it is released.
- **TMDB "Popular"** with a vote-count threshold, monitored but *not* auto-searched, so it fills the "Discover" section without downloading a hundred films at once.
- A collection list for a director or franchise when a new entry is announced.

The important toggle on every list is **Monitor: yes, Search on add: no** for anything you are not sure you will watch. Radarr then knows about the film, shows it in the calendar, and waits for you to click search.

## Naming and folders

```
{Movie CleanTitle} ({Release Year})/{Movie CleanTitle} ({Release Year}) [{Quality Full}]{[MediaInfo VideoDynamicRange]}
```

One folder per film, year in both the folder and the file name. Jellyfin matches this without a single manual identification. The dynamic-range tag at the end lets you see at a glance in the file browser which films are HDR.

![Framed posters of upcoming movies in a cinema hallway under 'Coming Soon' sign.](/media/2026/09/44eea087f77db874.jpg)
*Photo by [𝗛&𝗖𝗢 　](https://www.pexels.com/@hngstrm) on [Pexels](https://www.pexels.com)*

## Keeping the library small

A film library grows without limit if you let it. Two things keep mine honest:

1. **Unmonitor after watching.** A tiny script asks Jellyfin which films were watched more than 90 days ago and unmonitors them in Radarr, so they stop upgrading. Deleting is still a human decision.
2. **Disk-space thresholds.** *Settings → Media Management → Minimum Free Space* stops Radarr from grabbing when the pool is nearly full, which is far better than finding out from Jellyfin failing to write a thumbnail.

## Connections

Same as Sonarr: Jackett for indexers, qBittorrent behind the VPN container as the client, a Jellyfin refresh on import, and an ntfy webhook for imports and failures. Radarr also gets a **Discord/ntfy notification on "Movie Added"** from lists, so a list that suddenly adds fifty films is noticed before it downloads them.

---
*Cover photo by [Sami TÜRK](https://www.pexels.com/@trksami) on [Pexels](https://www.pexels.com).*
