---
title: How this blog is built: one Go binary, SQLite and no build step
slug: how-this-blog-is-built
summary: The site you are reading is a single Go program with templates and CSS embedded in it, a SQLite file for posts and a second one for traffic. No JavaScript toolchain, no external database, a 20 MB container. What it does, what it deliberately does not, and the parts that took longest.
tags: homelab, go, sqlite, self-hosting, blog
days_ago: 0
---
Every few years I try a blogging platform, write two posts, and abandon it when an update breaks a theme or a plugin wants a database upgrade. This time the blog is a program I can read end to end. It is written in Go, stores everything in SQLite, and ships as one static binary in a container built from `scratch`. This post is the design and the trade-offs.

![Detailed view of Ruby on Rails code highlighting software development intricacies.](/media/2026/09/89c0cd76fd7db5c2.jpg)
*Photo by [Digital Buggu](https://www.pexels.com/@digitalbuggu) on [Pexels](https://www.pexels.com)*

## What is in the binary

The entire site is one process. It serves the pages, renders Markdown, handles accounts, records traffic, and hosts the admin panel. The layout of the repository is the layout of the program:

```
cmd/blogd         entrypoint, config, graceful shutdown
internal/store    SQLite: users, sessions, posts, tags, comments, likes, media, settings
internal/stats    visit recorder + dashboard queries
internal/render   goldmark + bluemonday, excerpts, reading time
internal/mail     Resend client for codes, ntfy for pings
internal/web      handlers, middleware, templates/, static/
```

Templates and static assets are compiled in with `embed`, so the container has no filesystem to speak of. Uploaded media goes to a bind-mounted directory and is served straight from disk. The Dockerfile runs `go vet` and the tests before building, so a broken change never becomes a running container.

Dependencies are few on purpose: goldmark for Markdown (GFM, footnotes, code fences), bluemonday to sanitise the result, and a pure-Go SQLite driver so cross-compiling and the `scratch` image both work without cgo. Everything else is the standard library.

## Why SQLite, and why two files

Posts, users, comments and settings live in `blog.db`. Traffic lives in `stats.db`, with a JSON-lines journal alongside it that the recorder appends to before the batch insert, so a crash mid-batch loses nothing. Keeping traffic in a separate file means the main database stays small and a busy day of page views never contends with someone saving a post.

Full-text search is SQLite's FTS5, kept in sync with triggers. Related posts are the ones sharing the most tags, computed at request time; at this scale a query that would frighten a Postgres admin returns in under a millisecond. The database is backed up by copying a file. That sentence alone is most of the argument for SQLite on a personal site.

## The parts that took longest

**Accounts.** A blog does not need accounts until it has comments, and comments without accounts are spam. Registration sends a six-digit code by e-mail; the code is stored hashed, expires in ten minutes, and locks after five wrong attempts. Sessions are random tokens, also stored hashed, so a copy of the database does not hand anyone a login. Password reset and e-mail change reuse the same code machinery. This was more code than the posts themselves.

**Doing the right thing with HTML.** Markdown gets rendered by goldmark and then filtered by bluemonday with an allow list. A small allow list of raw HTML survives so that a YouTube embed or a `<figure>` works; anything else is escaped and shown as text. The Content-Security-Policy is strict enough that the admin editor's live preview had to be written without inline scripts.

**Traffic without a tracker.** Every page view is recorded server-side: path, referrer, UTM parameters, country and city from a local DB-IP database, browser and OS parsed from the user agent. Unique visitors are counted from a daily hash of IP and user agent that is never stored raw. The dashboard shows timelines, top pages, sources, countries and devices, plus a full log with filters and CSV export. It answers every question I ever asked an analytics product, with no script on the page and nothing leaving the server.


## What it deliberately does not do

- **No JavaScript build.** The few scripts that exist (theme toggle, editor preview, drag-and-drop upload) are plain files served as-is.
- **No plugins, no themes.** The look is one CSS file with a handful of variables; the accent colour is a setting in the admin panel.
- **No image processing beyond what is needed.** Avatars are cropped to 256 px on upload. Post images are stored as uploaded; browsers are good at scaling.
- **No comments system as a service.** Comments are rows in the same database, threaded one level, plain text only.

![From below of long thin blue cables connected to row of small white connectors on system block in data center](/media/2026/09/bfc7792910855950.jpg)
*Photo by [Brett Sayles](https://www.pexels.com/@brett-sayles) on [Pexels](https://www.pexels.com)*

## Publishing

Posts are written as Markdown files with a little front matter and pushed to the site by a script that logs in as the admin and submits the editor form, the same way a browser would. A second script asks Unsplash or Pexels for photos matching a query, uploads them, sets the cover and inserts figures with credits. Both scripts are in the repository, so the Markdown files are the source of truth and the database is a build artefact that happens to be live.

## Running it

```sh
cp .env.example .env    # RESEND_API_KEY, ADMIN_EMAIL, ADMIN_PASSWORD
docker compose up -d --build
```

It listens on localhost, joins the shared Docker network, and is reached through the same [Cloudflare Tunnel](/p/cloudflare-tunnel-zero-open-ports) as everything else here. Memory use sits around 30 MB. The container restarts in under a second, which is the kind of number that makes you stop worrying about deployment.

---
*Cover photo by [Pixabay](https://www.pexels.com/@pixabay) on [Pexels](https://www.pexels.com).*
