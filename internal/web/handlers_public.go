package web

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fzserver/frazile-blog/internal/render"
	"github.com/fzserver/frazile-blog/internal/stats"
	"github.com/fzserver/frazile-blog/internal/store"
)

func (s *Server) routes() {
	m := s.mux
	sub, _ := fs.Sub(staticFS, "static")
	m.Handle("GET /static/", http.StripPrefix("/static/", cacheControl(http.FileServerFS(sub))))
	m.HandleFunc("GET /media/", s.serveMedia)
	m.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ok %s\n", time.Since(s.started).Truncate(time.Second))
	})
	m.HandleFunc("GET /favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/static/favicon.svg", http.StatusMovedPermanently)
	})

	m.HandleFunc("GET /{$}", s.home)
	m.HandleFunc("GET /p/{slug}", s.post)
	m.HandleFunc("POST /p/{slug}/comment", s.authed("", s.addComment))
	m.HandleFunc("POST /comments/{id}/delete", s.authed("", s.deleteComment))
	m.HandleFunc("POST /comments/{id}/edit", s.authed("", s.editComment))
	m.HandleFunc("POST /api/posts/{id}/like", s.authed("", s.likePost))
	m.HandleFunc("GET /tag/{slug}", s.tag)
	m.HandleFunc("GET /tags", s.tags)
	m.HandleFunc("GET /search", s.search)
	m.HandleFunc("GET /archive", s.archive)
	m.HandleFunc("GET /archive/{year}/{month}", s.archiveMonth)
	m.HandleFunc("GET /about", s.about)
	m.HandleFunc("GET /u/{username}", s.profile)
	m.HandleFunc("GET /feed.xml", s.feed)
	m.HandleFunc("GET /rss", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/feed.xml", http.StatusMovedPermanently)
	})
	m.HandleFunc("GET /sitemap.xml", s.sitemap)
	m.HandleFunc("GET /robots.txt", s.robots)

	m.HandleFunc("GET /login", s.loginForm)
	m.HandleFunc("POST /login", s.login)
	m.HandleFunc("GET /register", s.registerForm)
	m.HandleFunc("POST /register", s.register)
	m.HandleFunc("GET /verify", s.verifyForm)
	m.HandleFunc("POST /verify", s.verify)
	m.HandleFunc("POST /verify/resend", s.verifyResend)
	m.HandleFunc("GET /forgot", s.forgotForm)
	m.HandleFunc("POST /forgot", s.forgot)
	m.HandleFunc("GET /reset", s.resetForm)
	m.HandleFunc("POST /reset", s.reset)
	m.HandleFunc("POST /logout", s.logout)
	m.HandleFunc("GET /settings", s.authed("", s.settingsForm))
	m.HandleFunc("POST /settings/profile", s.authed("", s.settingsProfile))
	m.HandleFunc("POST /settings/avatar", s.authed("", s.settingsAvatar))
	m.HandleFunc("POST /settings/password", s.authed("", s.settingsPassword))
	m.HandleFunc("POST /settings/email", s.authed("", s.settingsEmail))
	m.HandleFunc("POST /settings/email/confirm", s.authed("", s.settingsEmailConfirm))
	m.HandleFunc("POST /settings/logout-all", s.authed("", s.logoutAll))

	m.HandleFunc("GET /admin", s.authed("writer", s.adminHome))
	m.HandleFunc("GET /admin/{$}", s.authed("writer", s.adminHome))
	m.HandleFunc("GET /admin/api/summary", s.authed("admin", s.adminSummary))
	m.HandleFunc("GET /admin/traffic", s.authed("admin", s.adminTraffic))
	m.HandleFunc("GET /admin/traffic.csv", s.authed("admin", s.adminTrafficCSV))
	m.HandleFunc("GET /admin/posts", s.authed("writer", s.adminPosts))
	m.HandleFunc("GET /admin/posts/new", s.authed("writer", s.adminPostForm))
	m.HandleFunc("GET /admin/posts/{id}", s.authed("writer", s.adminPostForm))
	m.HandleFunc("POST /admin/posts/save", s.authed("writer", s.adminPostSave))
	m.HandleFunc("POST /admin/posts/{id}/delete", s.authed("writer", s.adminPostDelete))
	m.HandleFunc("POST /admin/api/preview", s.authed("writer", s.adminPreview))
	m.HandleFunc("POST /admin/api/upload", s.authed("writer", s.adminUploadAPI))
	m.HandleFunc("GET /admin/comments", s.authed("admin", s.adminComments))
	m.HandleFunc("POST /admin/comments/{id}/{action}", s.authed("admin", s.adminCommentAction))
	m.HandleFunc("GET /admin/users", s.authed("admin", s.adminUsers))
	m.HandleFunc("POST /admin/users/{id}/{action}", s.authed("admin", s.adminUserAction))
	m.HandleFunc("GET /admin/media", s.authed("writer", s.adminMedia))
	m.HandleFunc("POST /admin/media/upload", s.authed("writer", s.adminMediaUpload))
	m.HandleFunc("POST /admin/media/{id}/delete", s.authed("writer", s.adminMediaDelete))
	m.HandleFunc("GET /admin/settings", s.authed("admin", s.adminSettings))
	m.HandleFunc("POST /admin/settings", s.authed("admin", s.adminSettingsSave))

	m.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { s.notFound(w, r) })
}

func cacheControl(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=86400")
		h.ServeHTTP(w, r)
	})
}

// serveMedia serves uploads with range support (video seeking) and long
// caching; names are random so a changed file gets a new URL anyway.
func (s *Server) serveMedia(w http.ResponseWriter, r *http.Request) {
	name := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/media/"))
	if strings.Contains(name, "..") || name == "/" {
		http.NotFound(w, r)
		return
	}
	abs := filepath.Join(s.cfg.MediaDir, filepath.FromSlash(name))
	f, err := os.Open(abs)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=2592000")
	w.Header().Set("Accept-Ranges", "bytes")
	http.ServeContent(w, r, st.Name(), st.ModTime(), f)
}

// ---- listings ----

func (s *Server) listPage(w http.ResponseWriter, r *http.Request, f store.PostFilter, d map[string]any, base string) {
	st := s.Settings()
	f.Status = store.StatusPublished
	f.Page = intParam(r, "page", 1)
	f.PerPage = st.PerPage
	posts, total, err := s.db.ListPosts(r.Context(), f)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	pages := (total + f.PerPage - 1) / f.PerPage
	if f.Page > 1 && f.Page > pages {
		s.notFound(w, r)
		return
	}
	d["Posts"] = posts
	d["Total"] = total
	d["Page"] = f.Page
	d["Pages"] = pages
	d["Base"] = base
	s.sidebar(r, d)
	s.render(w, r, "list", d)
}

func (s *Server) sidebar(r *http.Request, d map[string]any) {
	recent, _, _ := s.db.ListPosts(r.Context(), store.PostFilter{Status: store.StatusPublished, PerPage: 6})
	d["Recent"] = recent
	tags, _ := s.db.Tags(r.Context())
	if len(tags) > 20 {
		tags = tags[:20]
	}
	d["TagCloud"] = tags
}

func (s *Server) home(w http.ResponseWriter, r *http.Request) {
	d := s.data(r, "")
	d["Home"] = true
	d["Canonical"] = s.cfg.PublicURL + "/"
	s.listPage(w, r, store.PostFilter{Pinned: true}, d, "/")
}

func (s *Server) tag(w http.ResponseWriter, r *http.Request) {
	t, err := s.db.TagBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		s.notFound(w, r)
		return
	}
	d := s.data(r, "Tagged "+t.Name)
	d["Heading"] = "#" + t.Name
	d["Tag"] = t
	s.listPage(w, r, store.PostFilter{Tag: t.Slug}, d, "/tag/"+t.Slug)
}

func (s *Server) tags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.db.Tags(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := s.data(r, "Tags")
	d["Tags"] = tags
	s.render(w, r, "tags", d)
}

func (s *Server) search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	d := s.data(r, "Search")
	d["NoIndex"] = true
	if len(q) > 200 {
		q = q[:200]
	}
	if q == "" {
		d["Heading"] = "Search"
		d["Posts"] = []store.Post{}
		d["Page"], d["Pages"] = 1, 0
		s.sidebar(r, d)
		s.render(w, r, "list", d)
		return
	}
	d["Heading"] = "Results for “" + q + "”"
	d["Title"] = "Search: " + q
	s.listPage(w, r, store.PostFilter{Query: q}, d, "/search?q="+strings.ReplaceAll(q, " ", "+"))
}

func (s *Server) archive(w http.ResponseWriter, r *http.Request) {
	months, err := s.db.Archive(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := s.data(r, "Archive")
	d["Months"] = months
	s.render(w, r, "archive", d)
}

func (s *Server) archiveMonth(w http.ResponseWriter, r *http.Request) {
	y, _ := strconv.Atoi(r.PathValue("year"))
	mo, _ := strconv.Atoi(r.PathValue("month"))
	if y < 2000 || mo < 1 || mo > 12 {
		s.notFound(w, r)
		return
	}
	d := s.data(r, fmt.Sprintf("%s %d", time.Month(mo), y))
	d["Heading"] = fmt.Sprintf("%s %d", time.Month(mo), y)
	s.listPage(w, r, store.PostFilter{Year: y, Month: mo}, d, fmt.Sprintf("/archive/%d/%02d", y, mo))
}

func (s *Server) about(w http.ResponseWriter, r *http.Request) {
	d := s.data(r, "About")
	d["About"] = s.Settings().AboutHTML
	s.render(w, r, "about", d)
}

// ---- post page ----

func (s *Server) post(w http.ResponseWriter, r *http.Request) {
	p, err := s.db.PostBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		s.notFound(w, r)
		return
	}
	u := s.user(r)
	canSee := p.IsPublished() || (u != nil && (u.IsAdmin() || u.ID == p.AuthorID))
	if !canSee {
		s.notFound(w, r)
		return
	}
	setPost(r, p.ID)
	if p.IsPublished() && !stats.IsBot(r.UserAgent()) {
		s.countView(p.ID, s.ip(r))
	} else {
		skipTrack(r)
	}
	if u != nil {
		p.Liked = s.db.UserLiked(r.Context(), p.ID, u.ID)
	}
	comments, n, err := s.db.CommentsForPost(r.Context(), p.ID, userID(u), u.IsAdmin())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	related, _ := s.db.Related(r.Context(), p.ID, 4)
	d := s.data(r, p.Title)
	d["Post"] = p
	d["Comments"] = comments
	d["CommentCount"] = n
	d["Related"] = related
	d["Canonical"] = s.cfg.PublicURL + p.URL()
	if p.Summary != "" {
		d["Desc"] = p.Summary
	} else {
		d["Desc"] = render.Excerpt(render.Text(string(p.BodyHTML)), 160)
	}
	if p.Cover != "" {
		d["Image"] = absURL(s.cfg.PublicURL, p.Cover)
	}
	d["NoIndex"] = !p.IsPublished()
	s.sidebar(r, d)
	s.render(w, r, "post", d)
}

func userID(u *store.User) int64 {
	if u == nil {
		return 0
	}
	return u.ID
}

func absURL(public, u string) string {
	if strings.HasPrefix(u, "http") {
		return u
	}
	return public + u
}

func (s *Server) countView(postID int64, ip string) {
	key := strconv.FormatInt(postID, 10) + "|" + ip
	now := time.Now().Unix()
	s.mu.Lock()
	last, seen := s.viewsSeen[key]
	if !seen || now-last > 3600 {
		s.viewsSeen[key] = now
	}
	s.mu.Unlock()
	if !seen || now-last > 3600 {
		go s.db.IncView(contextBG(), postID)
	}
}

func (s *Server) likePost(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	id := pathID(r, "id")
	p, err := s.db.PostByID(r.Context(), id)
	if err != nil || !p.IsPublished() {
		s.writeJSON(w, http.StatusNotFound, map[string]any{"error": "no such post"})
		return
	}
	if !s.limit("like:"+strconv.FormatInt(u.ID, 10), 60, time.Minute) {
		s.writeJSON(w, http.StatusTooManyRequests, map[string]any{"error": "slow down"})
		return
	}
	liked, n, err := s.db.ToggleLike(r.Context(), id, u.ID)
	if err != nil {
		s.writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "failed"})
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"liked": liked, "likes": n})
}

// ---- comments ----

func (s *Server) addComment(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	p, err := s.db.PostBySlug(r.Context(), r.PathValue("slug"))
	if err != nil || !p.IsPublished() {
		s.notFound(w, r)
		return
	}
	st := s.Settings()
	if !st.CommentsOpen || !p.CommentsEnabled {
		s.flash(w, "err", "Comments are closed on this post.")
		http.Redirect(w, r, p.URL(), http.StatusSeeOther)
		return
	}
	body := strings.TrimSpace(r.FormValue("body"))
	if len(body) < 2 || len(body) > 4000 {
		s.flash(w, "err", "Comments must be between 2 and 4000 characters.")
		http.Redirect(w, r, p.URL()+"#comment-form", http.StatusSeeOther)
		return
	}
	if !s.limit("comment:"+strconv.FormatInt(u.ID, 10), 10, 10*time.Minute) || s.db.RecentCommentCountByUser(r.Context(), u.ID, 60) >= 3 {
		s.flash(w, "err", "You are commenting too fast. Try again in a minute.")
		http.Redirect(w, r, p.URL()+"#comments", http.StatusSeeOther)
		return
	}
	parent, _ := strconv.ParseInt(r.FormValue("parent"), 10, 64)
	status := store.CommentVisible
	if st.CommentsModerated && !u.CanWrite() {
		status = store.CommentPending
	}
	id, err := s.db.AddComment(r.Context(), p.ID, u.ID, parent, body, status)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	if st.NotifyComments {
		go s.notify("New comment on "+p.Title, fmt.Sprintf("%s: %s\n%s%s#c%d", u.Name(), render.Excerpt(body, 200), s.cfg.PublicURL, p.URL(), id), "speech_balloon")
	}
	if status == store.CommentPending {
		s.flash(w, "ok", "Thanks! Your comment is awaiting moderation.")
	}
	http.Redirect(w, r, fmt.Sprintf("%s#c%d", p.URL(), id), http.StatusSeeOther)
}

func (s *Server) commentOwnerOrAdmin(r *http.Request) (*store.Comment, bool) {
	u := s.user(r)
	c, err := s.db.CommentByID(r.Context(), pathID(r, "id"))
	if err != nil {
		return nil, false
	}
	return c, u.IsAdmin() || c.UserID == u.ID
}

func (s *Server) deleteComment(w http.ResponseWriter, r *http.Request) {
	c, ok := s.commentOwnerOrAdmin(r)
	if !ok {
		s.forbidden(w, r)
		return
	}
	if err := s.db.DeleteComment(r.Context(), c.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, "/p/"+c.PostSlug+"#comments", http.StatusSeeOther)
}

func (s *Server) editComment(w http.ResponseWriter, r *http.Request) {
	c, ok := s.commentOwnerOrAdmin(r)
	if !ok {
		s.forbidden(w, r)
		return
	}
	body := strings.TrimSpace(r.FormValue("body"))
	if len(body) < 2 || len(body) > 4000 {
		s.flash(w, "err", "Comments must be between 2 and 4000 characters.")
	} else if err := s.db.EditComment(r.Context(), c.ID, body); err != nil {
		s.fail(w, r, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/p/%s#c%d", c.PostSlug, c.ID), http.StatusSeeOther)
}

// ---- profiles ----

func (s *Server) profile(w http.ResponseWriter, r *http.Request) {
	u, err := s.db.UserByUsername(r.Context(), r.PathValue("username"))
	if err != nil || !u.Verified {
		s.notFound(w, r)
		return
	}
	s.db.ProfileCounts(r.Context(), u)
	d := s.data(r, u.Name())
	d["Profile"] = u
	if u.CanWrite() {
		posts, _, _ := s.db.ListPosts(r.Context(), store.PostFilter{Status: store.StatusPublished, AuthorID: u.ID, PerPage: 20})
		d["Posts"] = posts
	}
	comments, _ := s.db.UserComments(r.Context(), u.ID, 20)
	d["Comments"] = comments
	if u.Bio != "" {
		d["Desc"] = u.Bio
	}
	s.render(w, r, "profile", d)
}

// ---- feeds ----

type rssItem struct {
	Title   string   `xml:"title"`
	Link    string   `xml:"link"`
	GUID    string   `xml:"guid"`
	PubDate string   `xml:"pubDate"`
	Desc    string   `xml:"description"`
	Creator string   `xml:"dc:creator"`
	Cats    []string `xml:"category"`
}

func (s *Server) feed(w http.ResponseWriter, r *http.Request) {
	posts, _, err := s.db.ListPosts(r.Context(), store.PostFilter{Status: store.StatusPublished, PerPage: 30})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	st := s.Settings()
	type channel struct {
		Title   string    `xml:"title"`
		Link    string    `xml:"link"`
		Desc    string    `xml:"description"`
		Lang    string    `xml:"language"`
		Updated string    `xml:"lastBuildDate"`
		Items   []rssItem `xml:"item"`
	}
	type rss struct {
		XMLName xml.Name `xml:"rss"`
		Version string   `xml:"version,attr"`
		DC      string   `xml:"xmlns:dc,attr"`
		Channel channel  `xml:"channel"`
	}
	out := rss{Version: "2.0", DC: "http://purl.org/dc/elements/1.1/", Channel: channel{Title: st.SiteName, Link: s.cfg.PublicURL + "/", Desc: st.Description, Lang: "en", Updated: time.Now().UTC().Format(time.RFC1123Z)}}
	for _, p := range posts {
		desc := p.Summary
		if desc == "" {
			desc = render.Excerpt(render.Text(string(p.BodyHTML)), 300)
		}
		it := rssItem{Title: p.Title, Link: s.cfg.PublicURL + p.URL(), GUID: s.cfg.PublicURL + p.URL(), PubDate: p.Date().UTC().Format(time.RFC1123Z), Desc: desc, Creator: p.AuthorName}
		for _, t := range p.Tags {
			it.Cats = append(it.Cats, t.Name)
		}
		out.Channel.Items = append(out.Channel.Items, it)
	}
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(out)
}

func (s *Server) sitemap(w http.ResponseWriter, r *http.Request) {
	posts, _, err := s.db.ListPosts(r.Context(), store.PostFilter{Status: store.StatusPublished, PerPage: 5000})
	if err != nil {
		s.fail(w, r, err)
		return
	}
	tags, _ := s.db.Tags(r.Context())
	type u struct {
		Loc     string `xml:"loc"`
		LastMod string `xml:"lastmod,omitempty"`
	}
	type set struct {
		XMLName xml.Name `xml:"urlset"`
		NS      string   `xml:"xmlns,attr"`
		URLs    []u      `xml:"url"`
	}
	out := set{NS: "http://www.sitemaps.org/schemas/sitemap/0.9"}
	out.URLs = append(out.URLs, u{Loc: s.cfg.PublicURL + "/"}, u{Loc: s.cfg.PublicURL + "/about"}, u{Loc: s.cfg.PublicURL + "/tags"}, u{Loc: s.cfg.PublicURL + "/archive"})
	for _, p := range posts {
		out.URLs = append(out.URLs, u{Loc: s.cfg.PublicURL + p.URL(), LastMod: time.Unix(p.Updated, 0).UTC().Format("2006-01-02")})
	}
	for _, t := range tags {
		out.URLs = append(out.URLs, u{Loc: s.cfg.PublicURL + "/tag/" + t.Slug})
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Write([]byte(xml.Header))
	xml.NewEncoder(w).Encode(out)
}

func (s *Server) robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "User-agent: *\nDisallow: /admin\nDisallow: /settings\nDisallow: /login\nDisallow: /register\nDisallow: /verify\nDisallow: /search\nDisallow: /api/\n\nSitemap: %s/sitemap.xml\n", s.cfg.PublicURL)
}

func (s *Server) notify(title, body string, tags ...string) {
	if s.cfg.NtfyURL == "" {
		return
	}
	notifyFn(s.cfg.NtfyURL, title, body, tags...)
}

var errNoMail = errors.New("mail not configured")
