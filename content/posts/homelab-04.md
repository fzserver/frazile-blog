---
title: Moving Jellyfin's database off spinning disk
slug: jellyfin-database-ssd
summary: Library scans had started timing out with SQLite lock errors, and the culprit was a 1.8 GB database living on a hard drive. Moving just the config directory to the system SSD fixed it, plus one .ignore file that stopped Jellyfin indexing a quarter of a million thumbnails.
tags: homelab, jellyfin, sqlite, storage
days_ago: 15
---
Jellyfin had been getting slower for months without ever being *broken*. The web UI would hang for a few seconds on library pages. Scans took hours. Then one evening the log filled with:

```
SQLite Error 5: 'database is locked'
... 30000ms timeout expired
```

Thirty-second lock timeouts on a single-user media server are not normal. Time to look at where the database actually lived.

![Contrasting data storage technologies: NVMe SSD, HDD, and CD.](/media/2026/09/a818b3bb055aced7.jpg)
*Photo by [Andrey Matveev](https://www.pexels.com/@zeleboba) on [Pexels](https://www.pexels.com)*

## The problem: everything on the HDD

The whole `config` directory, database included, was bind-mounted from the same 3.6 TB spinning disk that holds the media. That made sense when the library was small. It stopped making sense once `jellyfin.db` grew to **1.8 GB** and every scan meant thousands of small random writes to a disk that was also streaming video.

SQLite in WAL mode handles concurrent readers fine, but a writer that is waiting on a slow disk holds the lock for the whole time. Scans and playback were fighting over the same platter.

## The fix: config on SSD, media stays put

Jellyfin keeps three kinds of data:

| Directory | Contents | Where it should live |
|---|---|---|
| `config/` | database, settings, plugin data | **SSD** (small, hot, latency-sensitive) |
| `cache/` | transcodes, image cache | SSD or tmpfs |
| media libraries | the actual files | HDD, read-only mount is fine |

The migration is unglamorous:

```sh
docker compose down
rsync -a --info=progress2 /mnt/hdd/jellyfin/config/ /home/user/jellyfin/config/
mv /mnt/hdd/jellyfin/config /mnt/hdd/jellyfin/config.hdd-backup-$(date +%Y%m%d)
# edit docker-compose.yml: - /home/user/jellyfin/config:/config
docker compose up -d
```

Keep the old copy until a couple of scans have run clean, then delete it. The rsync of 2 GB of config takes a minute; the difference is immediate. Library pages open instantly and a full scan dropped from hours to about twenty minutes.

![Smart TV displaying streaming content in modern living room setting with exposed brick wall.](/media/2026/09/622502220c9a4f46.jpg)
*Photo by [https://kaboompics.com/](https://www.pexels.com/@karola-g) on [Pexels](https://www.pexels.com)*

## The other problem: 248,000 thumbnails

While watching the scan I noticed it was spending most of its time in a directory that was not media at all. A wallpaper scraper I run writes its thumbnail cache next to the originals, and Jellyfin was dutifully crawling all 248k of them as if they were a photo library.

Jellyfin honours a `.ignore` file: drop an empty one in any directory and the scanner skips that tree entirely.

```sh
touch /mnt/wallpapers/thumbs/.ignore
```

That alone removed most of the remaining scan time and, I suspect, a good chunk of the database growth.

## Checklist for anyone with the same symptoms

- Is `config/` on the same disk as the media? Move it.
- How big is `jellyfin.db`? Over a few hundred MB on a hard drive is a warning sign.
- Are there directories in your libraries that are not media (caches, thumbnails, extracted archives)? `.ignore` them.
- Is `cache/` on fast storage? Transcode segments are written constantly during playback.
- Are hardware transcodes actually being used? Check the dashboard during playback; software transcoding on a small server will also make everything else slow.

None of this is specific to Jellyfin. Any app that keeps a SQLite database, which is most self-hosted apps, wants that file on the fastest disk you have.

---
*Cover photo by [William Warby](https://www.pexels.com/@wwarby) on [Pexels](https://www.pexels.com).*
