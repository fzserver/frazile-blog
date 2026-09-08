#!/usr/bin/env python3
"""Give posts real photos from Unsplash or Pexels.

    tools/addimages.py plan.tsv          apply a plan (see below)
    tools/addimages.py --fix ID SLOT SOURCE "query|must,words"

Plan rows (tab separated): post id, markdown file, source (unsplash|pexels),
cover query, inline query 1, inline query 2. A query may carry "|w1,w2":
the first result whose description mentions one of the words is used.

The cover is set on the post; inline photos are inserted after the intro
and before the closing section, each with a photographer credit line, and
the markdown file is rewritten so it stays the source of truth. --fix
replaces one slot ('cover', 1 or 2) of an already-processed post.

Needs UNSPLASH_ACCESS_KEY and/or PEXELS_API_KEY in the environment or .env.
Unsplash's demo tier allows 50 requests/hour; each photo costs two.
"""
import datetime
import json
import re
import sqlite3
import sys
import urllib.parse
import urllib.request

from blogapi import Blog, load_env, ROOT  # noqa: F401 (ROOT used for plan lookup)

ENV = load_env()
UTM = "utm_source=frazile_blog&utm_medium=referral"
DB = ENV.get("BLOG_DB", "/mnt/fzhnas1/frazileserver/frazile-blog/blog.db")  # read only: published_at and cover lookups
used = set()


def get(url, headers=None):
    h = {"User-Agent": "frazile-blog/1.0 (+https://blog.frazile.com)"}
    h.update(headers or {})
    return urllib.request.urlopen(urllib.request.Request(url, headers=h), timeout=120).read()


def split(q):
    q, _, m = q.partition("|")
    return q, [w for w in m.split(",") if w]


def ok(alt, must):
    return not must or any(w in alt.lower() for w in must)


def unsplash(q):
    q, must = split(q)
    key = ENV["UNSPLASH_ACCESS_KEY"]
    d = json.loads(get("https://api.unsplash.com/search/photos?" + urllib.parse.urlencode(
        {"query": q, "per_page": 8, "orientation": "landscape", "content_filter": "high"}), {"Authorization": "Client-ID " + key}))
    for r in d["results"]:
        alt = r.get("alt_description") or r.get("description") or q
        if r["id"] in used or not ok(alt, must):
            continue
        get(r["links"]["download_location"], {"Authorization": "Client-ID " + key})  # API guideline: register the download
        return {"id": r["id"], "alt": alt, "data": get(r["urls"]["raw"] + "&w=1600&q=82&fm=jpg&fit=max"),
                "author": r["user"]["name"], "author_url": r["user"]["links"]["html"] + "?" + UTM,
                "site": "Unsplash", "site_url": "https://unsplash.com/?" + UTM}


def pexels(q):
    q, must = split(q)
    d = json.loads(get("https://api.pexels.com/v1/search?" + urllib.parse.urlencode(
        {"query": q, "per_page": 8, "orientation": "landscape"}), {"Authorization": ENV["PEXELS_API_KEY"]}))
    for p in d["photos"]:
        alt = p.get("alt") or q
        if p["id"] in used or not ok(alt, must):
            continue
        return {"id": p["id"], "alt": alt, "data": get(p["src"]["large2x"]), "author": p["photographer"],
                "author_url": p["photographer_url"], "site": "Pexels", "site_url": "https://www.pexels.com"}


def fetch(blog, source, q):
    p = (unsplash if source == "unsplash" else pexels)(q)
    if not p:
        return None
    used.add(p["id"])
    p["url"] = blog.upload(p["data"])["url"]
    print(f"    {q!r:42} -> {source} {p['id']} | {p['alt'][:60]}")
    return p


def figure(p):
    return f"![{p['alt']}]({p['url']})\n*Photo by [{p['author']}]({p['author_url']}) on [{p['site']}]({p['site_url']})*"


def credit(p):
    return f"\n\n---\n*Cover photo by [{p['author']}]({p['author_url']}) on [{p['site']}]({p['site_url']}).*"


def read_post(path):
    raw = open(path, encoding="utf-8").read()
    m = re.match(r"---\n(.*?)\n---\n(.*)", raw, re.S)
    meta = dict(line.split(": ", 1) for line in m.group(1).splitlines() if ": " in line)
    return m.group(1), meta, m.group(2).strip()


def save(blog, pid, path, front, meta, body, cover):
    db = sqlite3.connect(DB)
    pub = db.execute("select published_at from posts where id=?", (pid,)).fetchone()[0]
    st, loc, _ = blog.post("/admin/posts/save", {
        "id": pid, "title": meta["title"], "slug": meta.get("slug", ""), "summary": meta.get("summary", ""),
        "tags": meta.get("tags", ""), "body": body, "action": "save", "comments_enabled": "on", "cover": cover,
        "published_at": datetime.datetime.fromtimestamp(pub).strftime("%Y-%m-%dT%H:%M")})
    open(path, "w", encoding="utf-8").write("---\n" + front + "\n---\n" + body + "\n")
    print(f"  post {pid}: {st} {loc}")


def insert_positions(body):
    return [m.start() for m in re.finditer(r"^## ", body, re.M)]


def apply_plan(plan_path):
    blog = Blog().login()
    for line in open(plan_path):
        if not line.strip() or line.startswith("#"):
            continue
        pid, path, source, *queries = line.rstrip("\n").split("\t")
        pid = int(pid)
        print(f"[{pid}] {path}")
        front, meta, body = read_post(path)
        pics = [p for p in (fetch(blog, source, q) for q in queries) if p]
        heads = insert_positions(body)
        inserts = []
        if len(pics) > 1 and heads:
            inserts.append((heads[0], figure(pics[1])))
        if len(pics) > 2 and len(heads) >= 3:
            inserts.append((heads[-2], figure(pics[2])))
        for pos, txt in sorted(inserts, reverse=True):
            body = body[:pos] + txt + "\n\n" + body[pos:]
        cover = pics[0]["url"] if pics else ""
        if pics:
            body += credit(pics[0])
        save(blog, pid, path, front, meta, body, cover)


def fix(pid, slot, source, query):
    blog = Blog().login()
    for line in open(ROOT / "content/plan.tsv"):
        if line.startswith(f"{pid}\t"):
            path = line.split("\t")[1]
            break
    else:
        raise SystemExit(f"post {pid} not in content/plan.tsv")
    front, meta, body = read_post(path)
    p = fetch(blog, source, query)
    if not p:
        raise SystemExit("no matching photo")
    figs = list(re.finditer(r"!\[[^\]]*\]\([^)]+\)\n\*Photo by [^\n]*\*", body))
    cover = sqlite3.connect(DB).execute("select cover from posts where id=?", (pid,)).fetchone()[0]
    if slot == "cover":
        cover = p["url"]
        body = re.sub(r"\n\n---\n\*Cover photo by [^\n]*\*$", "", body) + credit(p)
    else:
        n = int(slot)
        if n <= len(figs):
            m = figs[n - 1]
            body = body[:m.start()] + figure(p) + body[m.end():]
        else:
            heads = insert_positions(body)
            pos = heads[-2] if len(heads) >= 3 else len(body)
            body = body[:pos] + figure(p) + "\n\n" + body[pos:]
    save(blog, pid, path, front, meta, body, cover)


if __name__ == "__main__":
    if len(sys.argv) == 6 and sys.argv[1] == "--fix":
        fix(int(sys.argv[2]), sys.argv[3], sys.argv[4], sys.argv[5])
    elif len(sys.argv) == 2:
        apply_plan(sys.argv[1])
    else:
        raise SystemExit(__doc__)
