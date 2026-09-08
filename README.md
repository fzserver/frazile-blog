# frazile-blog

The blog behind **https://blog.frazile.com**: a single Go binary with SQLite,
embedded templates and assets, and an admin panel. No JavaScript build step,
no external services beyond Resend (e-mail codes) and ntfy (pings).

## Features

- Posts in Markdown (GFM, footnotes, code blocks) with a live-preview editor,
  drag-and-drop / paste uploads for **images and videos**, YouTube embeds,
  cover images, tags, drafts, **scheduled publishing**, pinned posts.
- Accounts: register with **e-mail OTP verification**, log in, password reset
  by code, change e-mail (code to the new address), avatar upload (auto
  256px crop), display name / bio / website, public profile pages
  (`/u/<username>`), sign out everywhere.
- Roles: `reader` (comment, like), `author` (write own posts), `admin`.
- Threaded comments (one reply level), plain text only, edit/delete own,
  optional moderation queue, flood control, ban/unban.
- Likes, view counts (deduped per IP/hour), reading time, related posts,
  recent-posts sidebar, tag cloud, month archive, full-text search (FTS5),
  RSS feed, sitemap, OpenGraph tags, dark/light theme.
- **Admin panel** `/admin`: traffic dashboard (page views, unique visitors,
  landings, timeline + hour-of-day charts, top posts/pages, referrers,
  sources/mediums incl. utm_*, countries/cities via DB-IP, browsers/OS/
  devices, active members, latest visits), full **traffic log** with filters
  and CSV export, posts, comments moderation, users (roles, verify, ban,
  delete), media library, site settings (name, tagline, about page, accent
  colour, registration/comments toggles, moderation, social links, ntfy).
- Security: PBKDF2-SHA256 passwords, hashed session tokens, hashed OTPs with
  attempt caps and resend throttling, same-origin checks on every POST,
  per-IP rate limits on auth endpoints, sanitised HTML (bluemonday), CSP,
  content sniffing on uploads, scratch container with read-only rootfs.

## Run

```sh
cp .env.example .env   # fill in RESEND_API_KEY, ADMIN_EMAIL, ADMIN_PASSWORD
docker compose up -d --build
```

Data lives in `/mnt/fzhnas1/frazileserver/frazile-blog` (`blog.db`,
`stats.db`, `traffic/*.jsonl`, `media/`). Public traffic reaches the container
through the shared cloudflared tunnel (`blog.frazile.com → frazile-blog:8080`).

Local run without Docker:

```sh
ADMIN_EMAIL=a@b.c ADMIN_PASSWORD=secret go run ./cmd/blogd
```

Without `RESEND_API_KEY` the site runs, but registration and password reset
are disabled (nobody could receive a code). Admins can still create/verify
accounts from `/admin/users`.

## Layout

```
cmd/blogd            entrypoint, config, graceful shutdown, -healthcheck
internal/config      env → Config
internal/store       SQLite: users, sessions, codes, posts, tags, comments, likes, media, settings
internal/stats       visit recorder (async, journaled) + dashboard queries
internal/geoip       DB-IP .mmdb lookups
internal/mail        Resend client, OTP templates, ntfy
internal/render      goldmark + bluemonday, excerpts, reading time
internal/web         handlers, middleware, templates/, static/
```

## Tools

- `tools/publish.py content/posts/*.md` publishes markdown files with a
  small front matter (title, slug, summary, tags, days_ago/published, optional
  id to update, optional cover). Without a cover it uploads a generated
  gradient cover.
- `tools/addimages.py content/plan.tsv` fetches topic photos from Unsplash or
  Pexels (keys in `.env`), uploads them, sets the cover and inserts two inline
  figures with photographer credit; `--fix ID SLOT SOURCE "query|word,word"`
  swaps one photo. `content/posts/` holds the sources of the posts published
  so far and `content/plan.tsv` the photo queries used for them.
- Both talk to the running container on `127.0.0.1:8092` with the admin
  credentials from `.env`; the `Origin` header is set so the same-origin check
  passes.
