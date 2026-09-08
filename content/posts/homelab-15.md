---
title: Jackett: one indexer setup for every arr app
slug: jackett-indexer-proxy
summary: Jackett translates dozens of tracker sites into a single Torznab API that Sonarr, Radarr and Lidarr all understand. Configure each indexer once, hand out one URL, and never touch it again. The setup, the tagging that keeps adult indexers away from the family apps, and how it runs behind the VPN.
tags: homelab, jackett, arr stack, indexers, docker
days_ago: 0
---
Every arr app needs somewhere to search. Without a proxy, that means configuring the same trackers in Sonarr, then Radarr, then Lidarr, then again when a site changes its URL. Jackett does the configuring once. Each tracker becomes a **Torznab** feed: a standard XML search API that the arr apps consume identically regardless of what the site behind it looks like.

![A vintage card catalog drawer in an archive library setting, highlighting organization and history.](/media/2026/09/d113d5f1673da6ee.jpg)
*Photo by [Tima Miroshnichenko](https://www.pexels.com/@tima-miroshnichenko) on [Pexels](https://www.pexels.com)*

## Where it runs

Jackett makes outbound requests to tracker sites, so it lives in the same VPN namespace as the download clients:

```yaml
services:
  jackett:
    image: lscr.io/linuxserver/jackett:latest
    container_name: jackett
    network_mode: "service:gluetun"
    depends_on:
      gluetun: { condition: service_healthy }
    environment:
      PUID: 1000
      PGID: 1000
      TZ: Asia/Kolkata
      AUTO_UPDATE: "true"
    volumes:
      - ./config:/config
    restart: unless-stopped
```

With gluetun publishing `9117`. The arr apps reach it as `http://gluetun:9117`, the same way they reach the download clients. Jackett has no access to media folders at all; it only ever handles search results.

`AUTO_UPDATE` is on deliberately. Trackers change their HTML constantly and Jackett's indexer definitions are updated to match several times a week. A Jackett that is a month old is a Jackett with half its indexers broken.

## Adding indexers

In the web UI, *Add indexer* lists a few hundred definitions. For each one you use:

1. Fill in credentials or cookies if the site needs them (private trackers).
2. **Test**. Jackett runs a real search and shows the result count.
3. Copy the **Torznab feed URL** and the **API key** from the top of the page.

Then in Sonarr or Radarr: *Settings → Indexers → Add → Torznab*, paste the URL and key, and pick categories. That is the entire integration.

The alternative is Jackett's aggregate **"all indexers" feed** (`/api/v2.0/indexers/all/results/torznab`). Convenient, and I do not use it. When one indexer is slow or down, the aggregate feed waits for it, and every search in every arr app slows down together. One Torznab entry per indexer lets each app time out on the bad one and carry on.

## Tags: keeping indexers in their lane

Not every indexer should be visible to every app. Two things make that work:

**Jackett's indexer selection** is per feed, so an arr app only ever knows about the feeds it was given. Sonarr gets the TV-capable ones, Lidarr gets the music ones, and the indexers used for the [restricted library](/p/whisparr-adult-library) are configured in Whisparr only. Sonarr cannot stumble on results from them because it has never been told they exist.

**Categories** on each Torznab entry in the arr app restrict which result types are considered. Sonarr's entry is limited to the TV categories (5000 range), Radarr to Movies (2000), Lidarr to Audio (3000). A tracker that returns everything still only feeds each app what it is for.

## Settings inside the arr apps that make Jackett behave

- **RSS sync interval**: 30 minutes is plenty. Each arr app polls every Torznab feed on this interval, and Jackett turns each poll into a request to the tracker. Private trackers ban aggressive polling.
- **Minimum seeders**: 2 or more. Jackett reports seeder counts; a release with one seeder will sit at 99% for a week.
- **Seed ratio / seed time** per indexer, which the arr app passes to qBittorrent as the per-torrent seeding rule. Private trackers with ratio requirements get a higher number here than public ones.
- **Indexer priority**, when one source is consistently better; results from higher-priority indexers win ties.

## Cloudflare-protected trackers

Many trackers sit behind bot protection that Jackett cannot solve on its own. **FlareSolverr** is a small container that drives a headless browser to pass those checks; Jackett is pointed at it under *Settings → FlareSolverr API URL*. It is heavy (a full Chromium), so it runs only when something actually needs it and is stopped otherwise. Most of my indexers do not.

![Close-up of server racks in a data center highlighting modern technology infrastructure.](/media/2026/09/8254104a553d5fed.jpg)
*Photo by [panumas nikhomkhai](https://www.pexels.com/@cookiecutter) on [Pexels](https://www.pexels.com)*

## Monitoring

Jackett's dashboard shows per-indexer errors, but nobody looks at dashboards. Instead:

- Uptime Kuma checks Jackett's `/health` endpoint.
- Sonarr and Radarr report "indexer unavailable" health warnings, which are forwarded to ntfy. When one appears, the fix is usually a Jackett update, occasionally new cookies.

## Prowlarr?

Prowlarr is the arr team's own indexer manager and does what Jackett does plus the syncing: add an indexer once in Prowlarr and it pushes the configuration into every arr app automatically. It is the better choice for a new setup. Jackett stayed here because it was already configured, its definitions cover a few sites Prowlarr does not, and the manual Torznab step happens about twice a year. If I rebuilt the stack today, it would be Prowlarr with Jackett kept as one of Prowlarr's sources for the stragglers.

---
*Cover photo by [Markus Winkler](https://www.pexels.com/@markus-winkler-1430818) on [Pexels](https://www.pexels.com).*
