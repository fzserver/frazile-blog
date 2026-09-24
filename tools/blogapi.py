"""Shared helpers for the publishing tools: log in to the blog as the admin
and post multipart forms the way the browser would.

Configuration comes from environment variables, falling back to the .env
next to docker-compose.yml: BLOG_URL (default http://127.0.0.1:8092),
ADMIN_USERNAME, ADMIN_PASSWORD.

A log-in takes the password and then the code the blog e-mails. The session
is kept in tools/.session so one code covers every later run. Without a
terminal to type the code into, log in in two steps first:

    tools/blogapi.py login          sends the code
    tools/blogapi.py code 123456    finishes, saves the session
"""
import io
import os
import pathlib
import sys
import urllib.request
import uuid

ROOT = pathlib.Path(__file__).resolve().parent.parent
SESSION = ROOT / "tools" / ".session"
PENDING = ROOT / "tools" / ".login-pending"


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
        self.pending = ""  # fzb_login cookie of a log-in waiting on its code

    def get(self, path):
        headers = {"Cookie": self.cookie} if self.cookie else {}
        try:
            r = self.opener.open(urllib.request.Request(self.base + path, headers=headers))
        except urllib.error.HTTPError as e:
            r = e
        return getattr(r, "status", None) or r.code, r.headers.get("Location", "")

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
        cookies = [c for c in (self.cookie, self.pending if path.startswith("/login") else "") if c]
        if cookies:
            headers["Cookie"] = "; ".join(cookies)
        try:
            r = self.opener.open(urllib.request.Request(self.base + path, data=body.getvalue(), headers=headers))
        except urllib.error.HTTPError as e:
            r = e
        for sc in r.headers.get_all("Set-Cookie") or []:
            if sc.startswith("fzb_session="):
                self.cookie = sc.split(";")[0]
            elif sc.startswith("fzb_login="):
                self.pending = sc.split(";")[0]
        return getattr(r, "status", None) or r.code, r.headers.get("Location", ""), r.read()

    def login(self):
        """Reuse the saved session, or log in with the password and the mailed code."""
        if SESSION.exists():
            self.cookie = SESSION.read_text().strip()
            if self.get("/admin")[0] == 200:
                return self
            self.cookie = ""
        self.send_code()
        if not sys.stdin.isatty():
            raise SystemExit("log-in code sent; run tools/blogapi.py code <code>, then run this again")
        return self.finish(input("Log-in code from the e-mail: "))

    def send_code(self):
        st, loc, _ = self.post("/login", {"login": self.env["ADMIN_USERNAME"], "password": self.env["ADMIN_PASSWORD"], "next": "/admin"})
        if self.cookie:  # an instance without mail signs in on the password alone
            self._save()
            return
        if not loc.startswith("/login/code") or not self.pending:
            raise SystemExit(f"login failed ({st}); check ADMIN_USERNAME/ADMIN_PASSWORD")
        PENDING.write_text(self.pending)
        PENDING.chmod(0o600)

    def finish(self, code):
        if not self.pending and PENDING.exists():
            self.pending = PENDING.read_text().strip()
        st, _, _ = self.post("/login/code", {"code": code.strip(), "next": "/admin"})
        if not self.cookie:
            raise SystemExit(f"code rejected ({st}); a code is good for one log-in, in the run that asked for it")
        PENDING.unlink(missing_ok=True)
        self._save()
        return self

    def _save(self):
        SESSION.write_text(self.cookie)
        SESSION.chmod(0o600)

    def upload(self, data, filename="upload.jpg", mime="image/jpeg"):
        import json
        st, _, body = self.post("/admin/api/upload", {}, {"file": (filename, data, mime)})
        j = json.loads(body)
        if st != 200 or "url" not in j:
            raise RuntimeError(f"upload failed: {st} {j}")
        return j


if __name__ == "__main__":
    if sys.argv[1:2] == ["login"]:
        Blog().send_code()
        print("log-in code sent" if PENDING.exists() else "signed in")
    elif sys.argv[1:2] == ["code"] and len(sys.argv) == 3:
        Blog().finish(sys.argv[2])
        print("signed in; session saved to tools/.session")
    else:
        raise SystemExit(__doc__)
