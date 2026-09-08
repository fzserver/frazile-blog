---
title: Readarr: running an arr after its upstream retired
slug: readarr-after-retirement
summary: Readarr's developers retired the project and its metadata service went dark. The container still runs, existing libraries still work, and there are forks and alternatives. What that means for a homelab that depended on it, and what I moved to.
tags: homelab, readarr, arr stack, ebooks, calibre
days_ago: 0
---
Readarr was the arr for books: author tracking, release monitoring, ebook and audiobook downloads, and a bridge into Calibre for conversion and library management. It was also the least loved sibling in the family, and in 2025 the Servarr team announced it was being retired. The metadata backend that Readarr depends on stopped being maintained, and without it, adding authors or refreshing existing ones fails.

This post is a snapshot of running it through that, and what a sensible setup looks like now.

![Close-up of hands holding an e-reader on a wooden floor, depicting a casual reading moment.](/media/2026/09/7185e9942d34f7ae.jpg)
*Photo by [Letícia Alvares](https://www.pexels.com/@leticia-alvares-1805702) on [Pexels](https://www.pexels.com)*

## What still works, and what does not

If you have a Readarr instance with an existing library:

- **Importing, renaming and organising** files you already have or add by hand keeps working. That logic is local.
- **Searching indexers** for already-known books keeps working, because the search needs only the title and author already in the database.
- **Adding new authors or refreshing metadata** depends on the metadata server. When it is unavailable, this fails or returns stale data, and that is the part that retirement broke.
- **Updates** stop. The container image freezes at the last release, which means any security fix is on you to mitigate by keeping it off the internet.

Community forks appeared to keep the project going, with their own metadata infrastructure. They are worth a look if you want to keep the Readarr workflow, with the usual caution about trusting a young fork with write access to your library.

## The setup I ran

```yaml
services:
  readarr:
    image: lscr.io/linuxserver/readarr:develop
    container_name: readarr
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

Root folders: `/media/books` for ebooks and `/media/audiobooks` for audio, with separate quality profiles so an audiobook is never "upgraded" into an EPUB. The download client and indexers were the same shared ones as the rest of the stack, behind the VPN container.

Readarr was configured to hand finished ebooks to **Calibre** through the Calibre Content Server integration, which handled conversion to EPUB and kept the metadata (series, tags, covers) that Readarr itself was thin on.

![White headphones resting on colorful books, symbolizing modern audio technology.](/media/2026/09/f2e7ec2a8c329674.jpg)
*Photo by [Sound On](https://www.pexels.com/@sound-on) on [Pexels](https://www.pexels.com)*

## What I moved to

The honest lesson is that the book workflow never needed as much automation as TV. New books arrive a few times a month, not nightly. The replacement is smaller:

- **Calibre** remains the library. Books go in through Calibre's own tools, which handle conversion, metadata and covers better than Readarr ever did.
- **Calibre-Web** serves the library to phones and e-readers with a proper reading interface, OPDS feed and per-user shelves. It runs as a container alongside Jellyfin.
- **Audiobookshelf** took over audiobooks entirely. It has its own metadata matching, progress sync across devices, and a good app. Jellyfin's audiobook handling was always an afterthought.
- Discovery is a **Goodreads or StoryGraph list** and a manual search when something looks good. That is a five-minute task a couple of times a month.

The Readarr container is still defined in the repo, unmonitored and stopped, in case a fork stabilises enough to be worth re-enabling. Its config directory is backed up with everything else.

## Lessons for the rest of the stack

1. **Every arr has a metadata dependency you do not control.** Sonarr has TheTVDB, Radarr has TMDB, Lidarr has MusicBrainz. Those are healthy, but the failure mode is the same: the app keeps running while the thing that makes it useful quietly stops.
2. **Pinning images matters.** `:latest` on a retired project is a frozen image with an unknown expiry. Know which of your containers are still maintained.
3. **Keep the library independent of the tool.** Files named sanely in plain folders survive any app's retirement. A library that only makes sense inside one database does not.
4. **Match the automation to the arrival rate.** A trickle of books did not need an always-on watcher. Neither did a lot of other things I automated because I could.

---
*Cover photo by [Erik Mclean](https://www.pexels.com/@introspectivedsgn) on [Pexels](https://www.pexels.com).*
