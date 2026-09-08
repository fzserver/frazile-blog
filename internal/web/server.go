// Package web is the HTTP surface: public blog pages, account flows, the
// comment and like endpoints, media serving, and the admin panel. Templates
// and static assets are embedded so the binary is the whole deployment.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fzserver/frazile-blog/internal/config"
	"github.com/fzserver/frazile-blog/internal/mail"
	"github.com/fzserver/frazile-blog/internal/stats"
	"github.com/fzserver/frazile-blog/internal/store"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static
var staticFS embed.FS

const sessionCookie = "fzb_session"
const flashCookie = "fzb_flash"

type Server struct {
	cfg     config.Config
	db      *store.Store
	stats   *stats.Store
	mailer  *mail.Sender
	log     *slog.Logger
	tpls    map[string]*template.Template
	set     atomic.Pointer[store.Settings]
	mux     *http.ServeMux
	lim     *limiter
	started time.Time

	// viewsSeen dedupes post view counting per (post, ip) for an hour.
	mu        sync.Mutex
	viewsSeen map[string]int64
}

func New(cfg config.Config, db *store.Store, st *stats.Store, mailer *mail.Sender, log *slog.Logger) (*Server, error) {
	s := &Server{cfg: cfg, db: db, stats: st, mailer: mailer, log: log, mux: http.NewServeMux(),
		lim: newLimiter(), viewsSeen: map[string]int64{}, started: time.Now()}
	settings, err := db.LoadSettings(context.Background())
	if err != nil {
		return nil, err
	}
	s.set.Store(&settings)
	if err := s.parseTemplates(); err != nil {
		return nil, err
	}
	s.routes()
	go s.housekeeping()
	return s, nil
}

func (s *Server) Settings() store.Settings      { return *s.set.Load() }
func (s *Server) setSettings(st store.Settings) { s.set.Store(&st) }

func (s *Server) housekeeping() {
	for {
		time.Sleep(time.Hour)
		ctx := context.Background()
		s.db.PurgeSessions(ctx)
		if n, _ := s.db.PurgeUnverified(ctx, 24*time.Hour); n > 0 {
			s.log.Info("purged unverified accounts", "n", n)
		}
		s.mu.Lock()
		cut := time.Now().Unix() - 3600
		for k, t := range s.viewsSeen {
			if t < cut {
				delete(s.viewsSeen, k)
			}
		}
		s.mu.Unlock()
	}
}

// ---- templates ----

func (s *Server) parseTemplates() error {
	funcs := template.FuncMap{
		"date":     func(ts int64) string { return time.Unix(ts, 0).Format("Jan 2, 2006") },
		"datetime": func(ts int64) string { return time.Unix(ts, 0).Format("Jan 2, 2006 15:04") },
		"iso":      func(ts int64) string { return time.Unix(ts, 0).UTC().Format(time.RFC3339) },
		"ago":      ago,
		"num":      humanNum,
		"bytes":    humanBytes,
		"add":      func(a, b int) int { return a + b },
		"sub":      func(a, b int) int { return a - b },
		"mul":      func(a, b int) int { return a * b },
		"pct": func(a, b int) int {
			if b == 0 {
				return 0
			}
			return a * 100 / b
		},
		"seq": func(n int) []int {
			out := make([]int, n)
			for i := range out {
				out[i] = i + 1
			}
			return out
		},
		"safe":     func(s string) template.HTML { return template.HTML(s) },
		"safeAttr": func(s string) template.HTMLAttr { return template.HTMLAttr(s) },
		"css":      func(s string) template.CSS { return template.CSS(s) },
		"lower":    strings.ToLower,
		"initials": initials,
		"join":     strings.Join,
		"q":        url.QueryEscape,
		"trunc": func(s string, n int) string {
			if len(s) > n {
				return s[:n] + "…"
			}
			return s
		},
		"pageURL": func(base string, page int) string {
			if strings.Contains(base, "?") {
				return fmt.Sprintf("%s&page=%d", base, page)
			}
			return fmt.Sprintf("%s?page=%d", base, page)
		},
		"dict": func(kv ...any) map[string]any {
			m := map[string]any{}
			for i := 0; i+1 < len(kv); i += 2 {
				m[fmt.Sprint(kv[i])] = kv[i+1]
			}
			return m
		},
		"hasPrefix": strings.HasPrefix,
		"json": func(v any) template.JS {
			b, _ := json.Marshal(v)
			return template.JS(b)
		},
		"tagNames": func(tags []store.Tag) string {
			names := make([]string, len(tags))
			for i, t := range tags {
				names[i] = t.Name
			}
			return strings.Join(names, ", ")
		},
		"dateInput": func(ts int64) string {
			if ts == 0 {
				return ""
			}
			return time.Unix(ts, 0).Format("2006-01-02T15:04")
		},
	}
	base, err := template.New("base").Funcs(funcs).ParseFS(templateFS, "templates/base.html", "templates/partials.html")
	if err != nil {
		return err
	}
	entries, err := fs.ReadDir(templateFS, "templates")
	if err != nil {
		return err
	}
	s.tpls = map[string]*template.Template{}
	for _, e := range entries {
		name := e.Name()
		if name == "base.html" || name == "partials.html" {
			continue
		}
		t, err := base.Clone()
		if err != nil {
			return err
		}
		if _, err := t.ParseFS(templateFS, "templates/"+name); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		s.tpls[strings.TrimSuffix(name, ".html")] = t
	}
	return nil
}

type flash struct {
	Kind string
	Msg  string
}

// data is the base view model every page starts from.
func (s *Server) data(r *http.Request, title string) map[string]any {
	st := s.Settings()
	d := map[string]any{
		"Site":      st,
		"User":      s.user(r),
		"Path":      r.URL.Path,
		"Title":     title,
		"Desc":      st.Description,
		"Canonical": s.cfg.PublicURL + r.URL.Path,
		"Public":    s.cfg.PublicURL,
		"Year":      time.Now().Year(),
		"MailOn":    s.mailer != nil,
		"Query":     r.URL.Query().Get("q"),
	}
	if c, err := r.Cookie(flashCookie); err == nil && c.Value != "" {
		if v, err := url.QueryUnescape(c.Value); err == nil {
			kind, msg, _ := strings.Cut(v, ":")
			d["Flash"] = flash{Kind: kind, Msg: msg}
		}
	}
	return d
}

func (s *Server) renderStatus(w http.ResponseWriter, r *http.Request, status int, name string, d map[string]any) {
	t, ok := s.tpls[name]
	if !ok {
		s.fail(w, r, fmt.Errorf("no template %q", name))
		return
	}
	if _, has := d["Flash"]; has {
		// Consume the flash: it was rendered.
		http.SetCookie(w, &http.Cookie{Name: flashCookie, Value: "", Path: "/", MaxAge: -1})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := t.ExecuteTemplate(w, "base", d); err != nil {
		s.log.Error("render", "tpl", name, "err", err)
	}
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, d map[string]any) {
	s.renderStatus(w, r, http.StatusOK, name, d)
}

func (s *Server) notFound(w http.ResponseWriter, r *http.Request) {
	d := s.data(r, "Not found")
	d["NoIndex"] = true
	s.renderStatus(w, r, http.StatusNotFound, "error", map[string]any{
		"Site": d["Site"], "User": d["User"], "Path": d["Path"], "Title": "Not found", "Public": d["Public"], "Year": d["Year"], "NoIndex": true,
		"Code": 404, "Message": "That page does not exist, or has been unpublished."})
}

func (s *Server) forbidden(w http.ResponseWriter, r *http.Request) {
	d := s.data(r, "Forbidden")
	d["Code"], d["Message"], d["NoIndex"] = 403, "You do not have permission to do that.", true
	s.renderStatus(w, r, http.StatusForbidden, "error", d)
}

func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	s.log.Error("request failed", "path", r.URL.Path, "err", err)
	d := s.data(r, "Error")
	d["Code"], d["Message"], d["NoIndex"] = 500, "Something went wrong on our side. Please try again in a moment.", true
	s.renderStatus(w, r, http.StatusInternalServerError, "error", d)
}

func (s *Server) flash(w http.ResponseWriter, kind, msg string) {
	http.SetCookie(w, &http.Cookie{Name: flashCookie, Value: url.QueryEscape(kind + ":" + msg), Path: "/", MaxAge: 60, HttpOnly: true, SameSite: http.SameSiteLaxMode})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// ---- request helpers ----

type ctxKey int

const (
	userKey ctxKey = iota
	visitKey
)

type visitInfo struct {
	postID int64
	skip   bool
}

func (s *Server) user(r *http.Request) *store.User {
	u, _ := r.Context().Value(userKey).(*store.User)
	return u
}

func (s *Server) ip(r *http.Request) string {
	if s.cfg.TrustProxy {
		if v := r.Header.Get("CF-Connecting-IP"); v != "" {
			return v
		}
		if v := r.Header.Get("X-Forwarded-For"); v != "" {
			return strings.TrimSpace(strings.Split(v, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", MaxAge: 90 * 24 * 3600,
		HttpOnly: true, Secure: s.cfg.Secure(), SameSite: http.SameSiteLaxMode})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.cfg.Secure(), SameSite: http.SameSiteLaxMode})
}

// limit is a per-key token bucket: at most n events per window.
func (s *Server) limit(key string, n int, window time.Duration) bool {
	return s.lim.allow(key, n, window)
}

// sendCode issues a code for the address and e-mails it.
func (s *Server) sendCode(ctx context.Context, email, purpose string) error {
	if s.mailer == nil {
		return errors.New("mail is not configured")
	}
	code, err := s.db.IssueCode(ctx, email, purpose)
	if err != nil {
		return err
	}
	subject, text, html := mail.Code(s.Settings().SiteName, purpose, code, 10)
	_, err = s.mailer.Send(ctx, mail.Message{To: email, Subject: subject, Text: text, HTML: html, ReplyTo: s.cfg.MailReplyTo})
	return err
}

func intParam(r *http.Request, name string, def int) int {
	if v, err := strconv.Atoi(r.URL.Query().Get(name)); err == nil && v > 0 {
		return v
	}
	return def
}

func pathID(r *http.Request, name string) int64 {
	id, _ := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id
}

func formBool(r *http.Request, name string) bool {
	switch r.FormValue(name) {
	case "1", "on", "true", "yes":
		return true
	}
	return false
}

// ---- middleware ----

// withUser resolves the session cookie into a user on the context.
func (s *Server) withUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c, err := r.Cookie(sessionCookie); err == nil && c.Value != "" {
			u, err := s.db.UserBySession(r.Context(), c.Value)
			if err == nil && !u.Banned {
				s.db.TouchLastSeen(r.Context(), u.ID)
				r = r.WithContext(context.WithValue(r.Context(), userKey, u))
			} else {
				s.clearSessionCookie(w)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// csrf rejects state-changing requests that did not originate from this
// site. Every browser that matters sends Sec-Fetch-Site; older ones send
// Origin on POST. With SameSite=Lax cookies this is belt and braces.
func (s *Server) csrf(next http.Handler) http.Handler {
	self := s.cfg.Host()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Method != http.MethodOptions {
			if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
				http.Error(w, "cross-site request refused", http.StatusForbidden)
				return
			}
			if o := r.Header.Get("Origin"); o != "" && o != "null" {
				if u, err := url.Parse(o); err != nil || !strings.EqualFold(u.Host, self) && !strings.EqualFold(u.Host, r.Host) {
					http.Error(w, "origin mismatch", http.StatusForbidden)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// track records page views for HTML GETs outside the admin/static/API paths.
func (s *Server) track(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := r.URL.Path
		if r.Method != http.MethodGet || strings.HasPrefix(p, "/static/") || strings.HasPrefix(p, "/media/") ||
			strings.HasPrefix(p, "/admin") || strings.HasPrefix(p, "/api/") || p == "/healthz" || p == "/favicon.ico" ||
			strings.HasSuffix(p, ".xml") || p == "/robots.txt" {
			next.ServeHTTP(w, r)
			return
		}
		vi := &visitInfo{}
		r = r.WithContext(context.WithValue(r.Context(), visitKey, vi))
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		if vi.skip || sw.status >= 400 || s.stats == nil {
			return
		}
		attr := stats.Attribute(r.Referer(), s.cfg.Host(), r.URL.Query())
		v := stats.Visit{TS: time.Now().Unix(), IP: s.ip(r), UA: r.UserAgent(), Path: p, PostID: vi.postID,
			Attribution: attr, Landing: !attr.Internal}
		if u := s.user(r); u != nil {
			v.UserID, v.User = u.ID, u.Username
		}
		s.stats.Record(v)
	})
}

func setPost(r *http.Request, id int64) {
	if vi, ok := r.Context().Value(visitKey).(*visitInfo); ok {
		vi.postID = id
	}
}

func skipTrack(r *http.Request) {
	if vi, ok := r.Context().Value(visitKey).(*visitInfo); ok {
		vi.skip = true
	}
}

// authed gates a handler: any verified account, "writer" (author/admin) or "admin".
func (s *Server) authed(role string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := s.user(r)
		if u == nil {
			if r.Method == http.MethodGet {
				http.Redirect(w, r, "/login?next="+url.QueryEscape(r.URL.RequestURI()), http.StatusSeeOther)
			} else {
				http.Error(w, "login required", http.StatusUnauthorized)
			}
			return
		}
		switch role {
		case "admin":
			if !u.IsAdmin() {
				s.forbidden(w, r)
				return
			}
		case "writer":
			if !u.CanWrite() {
				s.forbidden(w, r)
				return
			}
		}
		next(w, r)
	}
}

func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "SAMEORIGIN")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' https: data:; media-src 'self' https:; frame-src https://www.youtube.com https://www.youtube-nocookie.com; style-src 'self' 'unsafe-inline'; script-src 'self'; object-src 'none'; base-uri 'self'; form-action 'self'")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Handler() http.Handler {
	var h http.Handler = s.mux
	h = s.track(h)
	h = s.csrf(h)
	h = s.withUser(h)
	h = secure(h)
	return h
}

// ---- small formatting helpers ----

func ago(ts int64) string {
	d := time.Since(time.Unix(ts, 0))
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return time.Unix(ts, 0).Format("Jan 2, 2006")
	}
}

func humanNum(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1e6)
	case n >= 10_000:
		return fmt.Sprintf("%.0fk", float64(n)/1e3)
	case n >= 1000:
		return fmt.Sprintf("%.1fk", float64(n)/1e3)
	}
	return strconv.Itoa(n)
}

func humanBytes(n int64) string {
	const u = 1024
	switch {
	case n >= u*u*u:
		return fmt.Sprintf("%.1f GB", float64(n)/(u*u*u))
	case n >= u*u:
		return fmt.Sprintf("%.1f MB", float64(n)/(u*u))
	case n >= u:
		return fmt.Sprintf("%.0f KB", float64(n)/u)
	}
	return fmt.Sprintf("%d B", n)
}

func initials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "?"
	}
	out := strings.ToUpper(string([]rune(parts[0])[0]))
	if len(parts) > 1 {
		out += strings.ToUpper(string([]rune(parts[len(parts)-1])[0]))
	}
	return out
}

// limiter is an in-memory sliding-window counter per key.
type limiter struct {
	mu sync.Mutex
	m  map[string][]int64
}

func newLimiter() *limiter { return &limiter{m: map[string][]int64{}} }

func (l *limiter) allow(key string, n int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now().UnixNano()
	cut := now - window.Nanoseconds()
	ts := l.m[key][:0:0]
	for _, t := range l.m[key] {
		if t > cut {
			ts = append(ts, t)
		}
	}
	if len(ts) >= n {
		l.m[key] = ts
		return false
	}
	l.m[key] = append(ts, now)
	if len(l.m) > 50_000 { // crude memory bound
		for k := range l.m {
			delete(l.m, k)
			break
		}
	}
	return true
}
