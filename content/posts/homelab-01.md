---
title: How my homelab is laid out: one repo, one compose file per service
slug: homelab-layout-compose-per-service
summary: Around thirty containers, a media stack, a VPN, a tunnel and a few web apps, all managed as a single git repo where every service is its own folder and its own docker-compose.yml. Here is why that shape has survived three years.
tags: homelab, docker, self-hosting, git
days_ago: 34
---
People often ask what I use to "manage" the homelab, expecting Portainer or Kubernetes. The honest answer is a git repository, a folder per service, and `docker compose`. It is not glamorous. It has also never once been the thing that broke.

![From below of long thin blue cables connected to row of small white connectors on system block in data center](/media/2026/09/373b22b9d73d79b1.jpg)
*Photo by [Brett Sayles](https://www.pexels.com/@brett-sayles) on [Pexels](https://www.pexels.com)*

## The shape

```
frazileserver/
├── fzmedia/          docker-compose.yml   (Jellyfin)
├── fzsonarr/         docker-compose.yml
├── fzradarr/         docker-compose.yml
├── fzvpn/            docker-compose.yml   (gluetun + the things behind it)
├── fzntfy/           docker-compose.yml
├── fzmonitor/        docker-compose.yml   (Uptime Kuma)
├── cloudflare/       docker-compose.yml + cloudflared/config.yml
├── config/           nginx conf.d, shared bits
└── .env
```

Every service is a self-contained directory with its own compose file, its own config folder, and nothing else. Starting a service is `cd fzsonarr && docker compose up -d`. Reading how a service is wired is opening one short file.

## Shared networks, declared once

The one piece of glue is a handful of **external Docker networks** created by hand: one for the media stack, one for the general services, one for the "cloud" apps. Each compose file joins the networks it needs:

```yaml
networks:
  frazileserver:
    external: true
```

Because the networks are external, no compose project owns them, and bringing one stack down never tears the network out from under another. The tunnel container sits on all of them and can reach any service by its container name.

## Why not one giant compose file?

I tried it. A single 800-line file has three problems:

1. `docker compose up -d` recreates anything whose config hash changed, and one typo in the Jellyfin block can restart the torrent client.
2. Diffs in git become unreadable; every change touches "the file".
3. You cannot hand a friend "the Sonarr setup" without handing them everything.

Per-service folders fix all three. The cost is that cross-service dependencies (`depends_on`) do not work across files, so start order is handled by `restart: always` and healthchecks rather than orchestration. In practice everything just retries until its neighbour is up.

## Submodules for the things I build

The apps I write myself (the wallpaper API, the marketing site, this blog) live in their own repositories with their own history. They are pulled into the server repo as **git submodules** so a fresh checkout of the server has everything, but the app code keeps its own commits and its own CI.

The rule of thumb: *third-party service* → a folder with a compose file; *my own code* → a submodule with a compose file inside it.

![Vibrant and engaging code displayed on a computer screen, showcasing programming concepts.](/media/2026/09/b4104f569a9729d6.jpg)
*Photo by [Syirwan Ainu](https://www.pexels.com/@syirwan) on [Pexels](https://www.pexels.com)*

## What lives outside git

- **Secrets.** Every service that needs a key reads it from a `.env` next to its compose file, and `.env` is ignored.
- **Data.** Databases, media, and config directories are bind-mounted from the NAS pools. The repo describes the services; it never contains their state.
- **Certificates.** Issued by the tunnel, not stored on the box.

## What I would change

If I started again I would name things by what they are (`jellyfin/`, not `fzmedia/`), and I would write the healthcheck for every service on day one instead of retrofitting them after the first mystery outage. Everything else has aged well enough that "how do I manage it" still has the same boring answer.

---
*Cover photo by [panumas nikhomkhai](https://www.pexels.com/@cookiecutter) on [Pexels](https://www.pexels.com).*
