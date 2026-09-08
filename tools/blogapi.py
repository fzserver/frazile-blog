"""Shared helpers for the publishing tools: log in to the blog as the admin
and post multipart forms the way the browser would.

Configuration comes from environment variables, falling back to the .env
next to docker-compose.yml: BLOG_URL (default http://127.0.0.1:8092),
ADMIN_USERNAME, ADMIN_PASSWORD.
"""
import io
import os
import pathlib
import urllib.request
import uuid

ROOT = pathlib.Path(__file__).resolve().parent.parent


def load_env(path=ROOT / ".env"):
    out = {}
    if path.exists():
        for line in path.read_text().splitlines():
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                out[k] = v.strip().strip('"')
    out.update({k: v for k, v in os.environ.items() if k in out or k.startswith(("ADMIN_", "BLOG_", "UNSPLASH_", "PEXELS_"))})
    return out


class _NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *a, **k):
        return None


class Blog:
    def __init__(self, env=None):
        self.env = env or load_env()
        self.base = self.env.get("BLOG_URL", "http://127.0.0.1:8092").rstrip("/")
        self.opener = urllib.request.build_opener(_NoRedirect())
        self.cookie = ""

    def post(self, path, fields, files=None):
        """Multipart POST. Returns (status, location, body)."""
        bnd = uuid.uuid4().hex
        body = io.BytesIO()
        for k, v in fields.items():
            body.write(f'--{bnd}\r\nContent-Disposition: form-data; name="{k}"\r\n\r\n'.encode())
            body.write(str(v).encode())
            body.write(b"\r\n")
        for k, (fn, data, ct) in (files or {}).items():
            body.write(f'--{bnd}\r\nContent-Disposition: form-data; name="{k}"; filename="{fn}"\r\nContent-Type: {ct}\r\n\r\n'.encode())
            body.write(data)
            body.write(b"\r\n")
        body.write(f"--{bnd}--\r\n".encode())
        headers = {"Content-Type": f"multipart/form-data; boundary={bnd}", "Origin": self.base}
        if self.cookie:
            headers["Cookie"] = self.cookie
        try:
            r = self.opener.open(urllib.request.Request(self.base + path, data=body.getvalue(), headers=headers))
        except urllib.error.HTTPError as e:
            r = e
        sc = r.headers.get("Set-Cookie", "")
        if "fzb_session=" in sc:
            self.cookie = sc.split(";")[0]
        return getattr(r, "status", None) or r.code, r.headers.get("Location", ""), r.read()

    def login(self):
        st, _, _ = self.post("/login", {"login": self.env["ADMIN_USERNAME"], "password": self.env["ADMIN_PASSWORD"], "next": "/admin"})
        if not self.cookie:
            raise SystemExit(f"login failed ({st}); check ADMIN_USERNAME/ADMIN_PASSWORD")
        return self

    def upload(self, data, filename="upload.jpg", mime="image/jpeg"):
        import json
        st, _, body = self.post("/admin/api/upload", {}, {"file": (filename, data, mime)})
        j = json.loads(body)
        if st != 200 or "url" not in j:
            raise RuntimeError(f"upload failed: {st} {j}")
        return j
