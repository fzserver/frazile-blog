---
title: Exposing a homelab with a Cloudflare Tunnel and zero open ports
slug: cloudflare-tunnel-zero-open-ports
summary: Every public service on my server, including this blog, reaches the internet through a single outbound tunnel. No port forwards, no dynamic DNS, no certificates to renew. How it is wired, and the two things that bit me.
tags: homelab, cloudflare, networking, security
days_ago: 29
---
My router has no port forwards. The server behind it serves a dozen public hostnames, all with valid TLS, and the only thing it ever does is open an outbound connection to Cloudflare. That connection is a **Cloudflare Tunnel**, and it has quietly replaced the entire "how do I expose this safely" problem.

![Close-up of server racks in a data center highlighting modern technology infrastructure.](/media/2026/09/91489ed1b9ef9096.jpg)
*Photo by [panumas nikhomkhai](https://www.pexels.com/@cookiecutter) on [Pexels](https://www.pexels.com)*

## The moving parts

- A `cloudflared` container on the server, joined to the same Docker networks as the services it fronts.
- A `config.yml` that maps hostnames to container addresses.
- One CNAME per hostname in Cloudflare DNS pointing at the tunnel's ID.

That is it. Cloudflare terminates TLS at the edge, forwards requests down the tunnel, and `cloudflared` hands them to the right container over the internal network.

## The config file

```yaml
tunnel: <tunnel-id>
credentials-file: /etc/cloudflared/<tunnel-id>.json

ingress:
  - hostname: media.example.com
    service: http://jellyfin:8096
  - hostname: blog.example.com
    service: http://frazile-blog:8080
  - hostname: example.com
    path: ^/api/pay/webhook$
    service: http://wallpaper-api:8080
  - hostname: example.com
    service: http://site:8080
  - service: http_status:404
```

Rules match top to bottom. A hostname can appear more than once with a `path` regex, which is how the payment webhook for the main site is routed to the API container while everything else on that hostname goes to the static site. The last rule is a catch-all so an unknown hostname gets a 404 rather than the first service in the list.

Because the services are addressed by **container name**, none of them publish ports to the host at all. The only port I bind is `127.0.0.1:xxxx` for local debugging.

## What you get for free

- **TLS everywhere**, renewed by Cloudflare. No certbot, no DNS-01 challenges.
- **A dynamic IP does not matter.** The tunnel is outbound; if the home IP changes, the tunnel reconnects.
- **The origin is invisible.** Nothing is listening on the public IP, so port scanners find nothing.
- **Real client IPs** arrive in the `CF-Connecting-IP` header, which the apps use for logging and rate limiting.
- **Access policies** can be layered on top with Zero Trust for the admin-only hosts.

![Sleek white wireless router with four antennas emitting soft blue and pink light.](/media/2026/09/f8cddc8594d373c8.jpg)
*Photo by [Jakub Zerdzicki](https://www.pexels.com/@jakubzerdzicki) on [Pexels](https://www.pexels.com)*

## The two things that bit me

**1. The config file is not hot-reloaded here.** The docs suggest `cloudflared` watches its config, but with the file on a bind mount from the NAS it never noticed a change. Adding this blog's hostname required a `docker restart cloudflared`, which drops every tunnel connection for a few seconds. It is quick, but it means "add a hostname" is a small planned event rather than a silent edit.

**2. Ingress validation lags the real config.** Running `cloudflared tunnel ingress validate` against my file complains about `preserveClientIP` and per-rule `headers`, both of which the running tunnel accepts happily. Treat the validator as a hint, not a gate, and test with `cloudflared tunnel ingress rule https://host/path` instead, which tells you exactly which rule would match.

## What it is not good for

Anything that is not HTTP. Mail, game servers, and raw TCP either need Cloudflare Spectrum or a different approach. Mail in particular needs a real, non-proxied A record and a working reverse DNS entry, which a residential connection usually cannot give you. For everything web-shaped, though, the tunnel is the first thing I set up on any new box.

---
*Cover photo by [Brett Sayles](https://www.pexels.com/@brett-sayles) on [Pexels](https://www.pexels.com).*
