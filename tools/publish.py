#!/usr/bin/env python3
"""Publish (or update) posts from markdown files with a small front matter.

    tools/publish.py content/posts/*.md

Front matter keys: title, slug, summary, tags (comma separated), and one of
published (ISO date/time) or days_ago (int). An `id` key updates that post
instead of creating a new one. A `cover` key sets the cover URL; without
one, a generated gradient cover with the title is uploaded (needs Pillow).
"""
import datetime
import io
import random
import re
import sys

from blogapi import Blog

PALETTES = [((20, 20, 30), (120, 80, 255)), ((10, 40, 60), (0, 180, 200)), ((60, 10, 40), (255, 120, 90)),
            ((15, 35, 25), (60, 200, 120)), ((40, 20, 10), (255, 180, 60)), ((25, 25, 45), (160, 130, 255)),
            ((30, 10, 10), (230, 70, 70)), ((10, 30, 40), (90, 200, 255)), ((30, 30, 10), (200, 220, 80))]


def parse(path):
    raw = open(path, encoding="utf-8").read()
    m = re.match(r"---\n(.*?)\n---\n(.*)", raw, re.S)
    if not m:
        raise SystemExit(f"{path}: missing front matter")
    meta = dict(line.split(": ", 1) for line in m.group(1).splitlines() if ": " in line)
    return meta, m.group(2).strip()


def gradient_cover(title, seed):
    from PIL import Image, ImageDraw, ImageFont
    W, H = 1600, 800
    a, b = PALETTES[seed % len(PALETTES)]
    img = Image.new("RGB", (W, H))
    px = img.load()
    for y in range(H):
        for x in range(0, W, 4):
            t = x / W * 0.6 + y / H * 0.4
            c = tuple(int(a[k] + (b[k] - a[k]) * t) for k in range(3))
            for dx in range(4):
                px[x + dx, y] = c
    d = ImageDraw.Draw(img, "RGBA")
    rnd = random.Random(seed)
    for _ in range(6):
        cx, cy, r = rnd.randint(0, W), rnd.randint(0, H), rnd.randint(120, 380)
        d.ellipse((cx - r, cy - r, cx + r, cy + r), fill=(255, 255, 255, 18))
    try:
        font = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf", 64)
        small = ImageFont.truetype("/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", 28)
    except OSError:
        font = small = ImageFont.load_default()
    lines, cur = [], ""
    for w in title.split():
        t = (cur + " " + w).strip()
        if d.textlength(t, font=font) > W - 200:
            lines.append(cur)
            cur = w
        else:
            cur = t
    lines.append(cur)
    y = H - 120 - len(lines) * 76
    for ln in lines:
        d.text((100, y), ln, font=font, fill=(255, 255, 255, 240))
        y += 76
    d.text((100, H - 70), "blog.frazile.com", font=small, fill=(255, 255, 255, 150))
    out = io.BytesIO()
    img.save(out, "PNG", optimize=True)
    return out.getvalue()


def when(meta, seed):
    if "published" in meta:
        return datetime.datetime.fromisoformat(meta["published"])
    days = int(meta.get("days_ago", 0))
    return datetime.datetime.now() - datetime.timedelta(days=days, hours=random.Random(seed).randint(1, 9))


def main(paths):
    blog = Blog().login()
    for i, path in enumerate(paths):
        meta, body = parse(path)
        fields = {"id": meta.get("id", ""), "title": meta["title"], "slug": meta.get("slug", ""),
                  "summary": meta.get("summary", ""), "tags": meta.get("tags", ""), "body": body,
                  "action": "save" if meta.get("id") else "publish", "comments_enabled": "on",
                  "published_at": when(meta, i).strftime("%Y-%m-%dT%H:%M")}
        files = {}
        if meta.get("cover"):
            fields["cover"] = meta["cover"]
        elif not meta.get("id"):
            files["cover_file"] = ("cover.png", gradient_cover(meta["title"], i), "image/png")
        st, loc, _ = blog.post("/admin/posts/save", fields, files)
        print(f"{path}: {st} {loc}")


if __name__ == "__main__":
    if len(sys.argv) < 2:
        raise SystemExit(__doc__)
    main(sys.argv[1:])
