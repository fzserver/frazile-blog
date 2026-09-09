---
title: Uptime Kuma and ntfy: monitoring that ends on my phone
slug: uptime-kuma-ntfy-monitoring
summary: A homelab without monitoring is a homelab you find out about from the person who wanted to watch a film. Uptime Kuma watches every service, ntfy delivers the news, and a handful of scripts use the same topic layout so every alert lands in one place.
tags: homelab, monitoring, ntfy, uptime kuma, docker
days_ago: 0
---
For the first year the monitoring strategy was "someone will tell me". Jellyfin down? A message arrives. Tunnel down? The blog stops getting hits. It works, in the sense that a smoke alarm made of neighbours works. This post is the setup that replaced it: Uptime Kuma as the thing that watches, ntfy as the thing that talks, and a convention for topics so alerts from scripts, containers and the monitor all look the same on the phone.

![Hand holding smartphone indoors with blurred office background, focus on screen notifications.](/media/2026/09/299f1ff7a452fc18.jpg)
*Photo by [Tranmautritam](https://www.pexels.com/@tranmautritam) on [Pexels](https://www.pexels.com)*

## Two small containers

Both live in the same repo as everything else, one folder each, with their own compose file.

```yaml
services:
  uptime-kuma:
    image: louislam/uptime-kuma:1
    container_name: fzmonitor
    volumes:
      - ./data:/app/data
      - /var/run/docker.sock:/var/run/docker.sock:ro
    ports:
      - "127.0.0.1:3001:3001"
    networks: [frazileserver]
    restart: unless-stopped
```

```yaml
services:
  ntfy:
    image: binwiederhier/ntfy
    container_name: fzntfy
    command: serve
    environment:
      NTFY_BASE_URL: https://fzntfy.frazile.app
      NTFY_BEHIND_PROXY: "true"
      NTFY_CACHE_FILE: /var/cache/ntfy/cache.db
      NTFY_ATTACHMENT_CACHE_DIR: /var/cache/ntfy/attachments
    volumes:
      - ./cache:/var/cache/ntfy
      - ./config:/etc/ntfy
    networks: [frazileserver]
    restart: unless-stopped
```

Neither publishes a port to the internet. Uptime Kuma is reachable on localhost and through the tunnel; ntfy is only reachable through the tunnel, which is how the phone gets to it. The Docker socket is mounted read-only into Uptime Kuma so it can do container monitors: "is `gluetun` running" is a different question from "does port 8080 answer", and the container monitor answers the first one.

## What gets a monitor

The rule is one monitor per thing that would make me sad, not one per container. Around twenty in total:

- **HTTP** monitors for every public hostname, through the public URL, so the check covers DNS, the tunnel and the app. A keyword monitor on the blog looks for the site name in the body, because a Cloudflare error page also returns 200 sometimes.
- **HTTP** monitors on the *internal* address for the same services, so I can tell "the app is down" from "the tunnel is down". When both fire it is the app; when only the public one fires it is the tunnel.
- **Docker container** monitors for the ones that die quietly: gluetun, the two torrent clients behind it, cloudflared.
- **TCP** monitors for Jellyfin's port and the NAS mounts' SMB port, because a stale mount is the most common cause of Jellyfin "working" with an empty library.
- **Push** monitors for cron jobs. The job calls a URL at the end of a successful run; if the URL is not called within the window, the monitor goes red. This catches the nightly backup silently not running, which no other type of check can see.

Intervals are 60 seconds for the public checks, five minutes for the rest, with three retries before anything is declared down. The retries matter: the tunnel reconnects in under a minute a few times a week, and every one of those used to be a notification.

## ntfy as the only door out

Every alert goes through ntfy, including the ones from Uptime Kuma. The notification setup in Kuma is a single ntfy entry pointing at the topic `Monitor`, with priority mapped so "down" is high and "up" is default. The phone app subscribes to the topics and that is the whole delivery chain. No Slack, no e-mail, no Telegram, one app with one badge.

Topics are how the noise is sorted. A short list, each with a purpose:

- `Monitor`: Uptime Kuma only.
- `Builds`: app builds and container rebuilds, so a long build can be started and forgotten.
- `Wallpapers`: the scraper that fills the wallpaper library, which runs for hours and reports per batch.
- `ServerIP`: the public IP changed, or the connection got slow enough that the speed check noticed.
- `frazileblog`: new comments and members on this blog.

Publishing needs nothing more than curl:

```sh
curl -H "Title: Backup finished" -H "Tags: white_check_mark" \
     -d "142 GB, 11 minutes" https://fzntfy.frazile.app/Builds
```

Because the server sits behind Cloudflare, a script using Python's `urllib` gets a 403 unless it sets its own `User-Agent`. That cost an afternoon once and now lives in a comment at the top of every script that sends a notification.

![Close-up of server racks in a data center highlighting modern technology infrastructure.](/media/2026/09/c38faf6bef85f0a9.jpg)
*Photo by [panumas nikhomkhai](https://www.pexels.com/@cookiecutter) on [Pexels](https://www.pexels.com)*

## The status page

Uptime Kuma can publish a status page, and mine is public: a single page listing the public services with their 30-day uptime. It is mostly for me, on the phone, when something feels slow. It also turns "is it down for everyone or just me" into a link I can send instead of a conversation.

## What it caught

In the months since this went in, the monitor has been the first to know about: a torrent client sitting in a fresh namespace with no network after gluetun was recreated, a NAS mount that went stale after a power blip, an expired VPN subscription (gluetun unhealthy, both clients red), and cloudflared needing a restart after its config changed. Every one of those was found at the time it happened rather than at the time someone wanted to use it, which is the whole point.

The part I would do first if starting again is the push monitors for scheduled jobs. Services announce their own failures loudly; a cron job that stops running says nothing at all.

---
*Cover photo by [AlphaTradeZone](https://www.pexels.com/@alphatradezone) on [Pexels](https://www.pexels.com).*
