---
title: Serving a wallpaper library from Cloudflare R2
slug: wallpapers-cloudflare-r2
summary: A few hundred thousand wallpapers live on the NAS, but the copies people download come from an R2 bucket behind a custom domain. Why object storage beat serving from home, what the s3fs mount is good for and what it is not, and the upload path that actually works for large files.
tags: homelab, cloudflare, r2, storage, wallpapers
days_ago: 0
---
The wallpaper site is the busiest thing I run, and none of its downloads touch the home connection. The library is scraped, sorted and thumbnailed on the server here, then the files that get served live in a Cloudflare R2 bucket with a custom domain in front. This post is the why and the how, including the part where the obvious upload method broke.

![Detailed view of a server rack with a focus on technology and data storage.](/media/2026/09/dcc3d3f8ec37de5f.jpg)
*Photo by [panumas nikhomkhai](https://www.pexels.com/@cookiecutter) on [Pexels](https://www.pexels.com)*

## Why not serve from home

The site's API runs at home behind the tunnel and answers small JSON requests quickly. Files are different. A 4K wallpaper is 5 to 15 MB, a live wallpaper video can be 100 MB, and a popular category page might trigger dozens of downloads a minute. Pushing that through a residential uplink means every visitor shares the same few tens of megabits, and the tunnel adds latency on top.

Object storage fixes both. R2 has no egress fee, which is the entire reason to pick it over the alternatives for a download-heavy site: the bill is storage plus operations, and a month of a few terabytes served costs less than a coffee. With a custom domain on the bucket the files are on Cloudflare's edge, cached close to the visitor, and the home connection is not involved at all.

## The bucket and the domain

The bucket is named after what it holds and has public access through a custom hostname only; the default `r2.dev` URL is disabled so there is exactly one way to reach a file. Keys mirror the library layout on the NAS:

```
gaming/by-game/<game>/<hash>.jpg
gaming/by-category/<category>/<hash>.jpg
gaming/live/<hash>.mp4
```

The API stores the key with each record and builds the download URL from a configured base, so if the domain or the bucket ever changes it is one setting. Cache headers are set at upload time: images are immutable (the name is a content hash), so a year-long `max-age` is safe and the edge does most of the serving.

## The s3fs mount, and where it stops

For browsing, the bucket is mounted on the server with s3fs:

```
s3fs wallpapers /mnt/cflr2 -o url=https://<account>.r2.cloudflarestorage.com \
    -o passwd_file=/etc/passwd-s3fs -o use_path_request_style
```

That gives `ls`, `du`, and the ability for the scraper to check whether a file already exists with ordinary filesystem calls. It runs as a container like everything else. Small files copy through it fine.

Large files do not. Anything above roughly 10 to 25 MB fails on close with "Input/output error": s3fs switches to multipart upload for big objects, and on this mount that path does not complete. With the mount configured by root inside a container and no appetite to debug FUSE, the fix was to stop treating the mount as the upload path.

## Uploading with wrangler

Cloudflare's CLI uploads directly to the bucket:

```sh
wrangler r2 object put "wallpapers/gaming/live/$hash.mp4" \
    --file "$path" --content-type video/mp4 \
    --cache-control "public, max-age=31536000, immutable" --remote
```

This has been reliable for files up to a couple of hundred megabytes. Wrangler caps a single upload at 300 MiB, so anything bigger needs either splitting or true S3 multipart with an access key and secret, which is a different kind of credential from the OAuth login wrangler uses. For wallpapers that limit is never reached.

The scraper therefore does three things per file: writes it to the NAS, uploads it with wrangler, and records the key in the database. The mount is used only to read.

One operational note: wrangler's login is an OAuth token that expires every so often. A scheduled job that suddenly fails with "Failed to fetch auth token" is not broken; it needs `wrangler login` run once in a browser. For unattended use an API token in the environment avoids the surprise.

![High-tech gaming setup featuring a curved monitor, RGB keyboard, and vibrant lighting.](/media/2026/09/9f35078fdfee9a4c.jpg)
*Photo by [Ron Lach](https://www.pexels.com/@ron-lach) on [Pexels](https://www.pexels.com)*

## Thumbnails stay at home

Thumbnails are the one thing not in R2. They are generated on the server, around 250 thousand small JPEGs, and served by the API from an SSD. They are tiny, they change when the library is re-sorted, and keeping them local means a re-thumbnail pass does not become a quarter of a million object writes. The trade-off is that listing pages do depend on the home connection, which is fine for 20 KB images and would not be fine for 10 MB ones.

A side effect worth knowing: a media server that shares the NAS will happily index a thumbnail cache as 250 thousand photos. A `.ignore` file in the directory tells Jellyfin to leave it alone, which was learned the slow way.

## What I would change

Thumbnails could go to R2 too, in a separate prefix, with the re-sort problem solved by naming them after the source hash rather than the category. And the s3fs mount could be dropped in favour of a small listing script using the S3 API, which would remove a FUSE container and a class of "the mount went stale" alerts. Neither is urgent. The site serves terabytes a month for almost nothing, and the home uplink carries only the API. That was the goal.

---
*Cover photo by [Ylanite Koppens](https://www.pexels.com/@nietjuhart) on [Pexels](https://www.pexels.com).*
