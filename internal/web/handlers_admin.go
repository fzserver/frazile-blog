package web

import (
	"encoding/csv"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/fzserver/frazile-blog/internal/render"
	"github.com/fzserver/frazile-blog/internal/stats"
	"github.com/fzserver/frazile-blog/internal/store"
)

var accentRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (s *Server) adminData(r *http.Request, title, section string) map[string]any {
	d := s.data(r, title)
	d["Admin"] = true
	d["Section"] = section
	d["NoIndex"] = true
	d["Pending"] = s.db.CountComments(r.Context(), store.CommentPending)
	return d
}

// ---- dashboard ----

func (s *Server) adminHome(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	if !u.IsAdmin() {
		http.Redirect(w, r, "/admin/posts", http.StatusSeeOther)
		return
	}
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "30d"
	}
	sm, err := s.stats.Summary(r.Context(), period)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	slugs := make([]string, 0, len(sm.TopPosts))
	for _, p := range sm.TopPosts {
		slugs = append(slugs, strings.TrimPrefix(p.Name, "/p/"))
	}
	titles := s.db.PostTitles(r.Context(), slugs)
	type topPost struct {
		Path, Title string
		Count       int
	}
	var tops []topPost
	for _, p := range sm.TopPosts {
		t := titles[strings.TrimPrefix(p.Name, "/p/")]
		if t == "" {
			t = p.Name
		}
		tops = append(tops, topPost{p.Name, t, p.Count})
	}
	day := time.Now().Add(-24 * time.Hour).Unix()
	week := time.Now().Add(-7 * 24 * time.Hour).Unix()
	d := s.adminData(r, "Dashboard", "dashboard")
	d["S"] = sm
	d["TopPosts"] = tops
	d["Period"] = period
	d["Totals"] = map[string]any{
		"Posts":      s.db.CountPosts(r.Context(), store.StatusPublished),
		"Drafts":     s.db.CountPosts(r.Context(), store.StatusDraft),
		"Users":      s.db.CountUsers(r.Context(), 0),
		"UsersWeek":  s.db.CountUsers(r.Context(), week),
		"Comments":   s.db.CountComments(r.Context(), ""),
		"Pending":    s.db.CountComments(r.Context(), store.CommentPending),
		"Visits":     s.stats.Total(r.Context()),
		"ViewsToday": s.viewsSince(r, day),
	}
	n, size := s.db.MediaTotals(r.Context())
	d["MediaCount"], d["MediaSize"] = n, size
	d["Geo"] = s.stats.GeoEnabled()
	recent, _, _ := s.stats.Search(r.Context(), stats.Query{PerPage: 15})
	d["Recent"] = recent
	comments, _, _ := s.db.ListComments(r.Context(), "", 1, 6)
	d["RecentComments"] = comments
	s.render(w, r, "admin_dashboard", d)
}

func (s *Server) viewsSince(r *http.Request, since int64) int {
	_, n, _ := s.stats.Search(r.Context(), stats.Query{Since: since, PerPage: 1})
	return n
}

func (s *Server) adminSummary(w http.ResponseWriter, r *http.Request) {
	sm, err := s.stats.Summary(r.Context(), r.URL.Query().Get("period"))
	if err != nil {
		s.writeJSON(w, 500, map[string]any{"error": err.Error()})
		return
	}
	s.writeJSON(w, 200, sm)
}

func (s *Server) trafficQuery(r *http.Request) stats.Query {
	q := r.URL.Query()
	tq := stats.Query{IP: q.Get("ip"), Path: q.Get("path"), Country: q.Get("country"), User: q.Get("user"),
		Device: q.Get("device"), Source: q.Get("source"), Bots: q.Get("bots") == "1", Page: intParam(r, "page", 1), PerPage: 50}
	switch q.Get("since") {
	case "24h":
		tq.Since = time.Now().Add(-24 * time.Hour).Unix()
	case "7d":
		tq.Since = time.Now().Add(-7 * 24 * time.Hour).Unix()
	case "30d":
		tq.Since = time.Now().Add(-30 * 24 * time.Hour).Unix()
	}
	return tq
}

func (s *Server) adminTraffic(w http.ResponseWriter, r *http.Request) {
	tq := s.trafficQuery(r)
	rows, total, err := s.stats.Search(r.Context(), tq)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := s.adminData(r, "Traffic log", "traffic")
	d["Rows"], d["Total"], d["Q"] = rows, total, tq
	d["Page"], d["Pages"] = tq.Page, (total+tq.PerPage-1)/tq.PerPage
	qs := r.URL.Query()
	qs.Del("page")
	d["Base"] = "/admin/traffic?" + qs.Encode()
	d["Since"] = r.URL.Query().Get("since")
	s.render(w, r, "admin_traffic", d)
}

func (s *Server) adminTrafficCSV(w http.ResponseWriter, r *http.Request) {
	tq := s.trafficQuery(r)
	tq.PerPage = 10000
	rows, _, err := s.stats.Search(r.Context(), tq)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="traffic.csv"`)
	cw := csv.NewWriter(w)
	cw.Write([]string{"time", "ip", "path", "user", "country", "region", "city", "provider", "vpn", "browser", "os", "device", "referrer", "source", "medium", "landing", "ua"})
	for _, x := range rows {
		cw.Write([]string{x.Time().UTC().Format(time.RFC3339), x.IP, x.Path, x.User, x.Country, x.Region, x.City, x.Provider, strconv.FormatBool(x.VPN), x.Browser, x.OS, x.Device, x.Referrer, x.Source, x.Medium, strconv.FormatBool(x.Landing), x.UA})
	}
	cw.Flush()
}

// ---- posts ----

func (s *Server) adminPosts(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	f := store.PostFilter{Status: r.URL.Query().Get("status"), Page: intParam(r, "page", 1), PerPage: 25}
	if !u.IsAdmin() {
		f.AuthorID = u.ID
	}
	posts, total, err := s.db.ListPosts(r.Context(), f)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := s.adminData(r, "Posts", "posts")
	d["Posts"], d["Total"], d["Status"] = posts, total, f.Status
	d["Page"], d["Pages"] = f.Page, (total+f.PerPage-1)/f.PerPage
	d["Base"] = "/admin/posts?status=" + f.Status
	d["Views7d"] = s.stats.PostViews(r.Context(), time.Now().Add(-7*24*time.Hour).Unix())
	s.render(w, r, "admin_posts", d)
}

func (s *Server) adminPostForm(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	p := &store.Post{Status: store.StatusDraft, CommentsEnabled: true, AuthorID: u.ID}
	if id := pathID(r, "id"); id > 0 {
		var err error
		p, err = s.db.PostByID(r.Context(), id)
		if err != nil {
			s.notFound(w, r)
			return
		}
		if !u.IsAdmin() && p.AuthorID != u.ID {
			s.forbidden(w, r)
			return
		}
	}
	d := s.adminData(r, "Edit post", "posts")
	if p.ID == 0 {
		d["Title"] = "New post"
	}
	d["P"] = p
	s.render(w, r, "admin_edit", d)
}

func (s *Server) adminPostSave(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	if err := r.ParseMultipartForm(int64(s.cfg.MaxImageMB+2) << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		s.flash(w, "err", "Upload too large.")
		http.Redirect(w, r, "/admin/posts", http.StatusSeeOther)
		return
	}
	id, _ := strconv.ParseInt(r.FormValue("id"), 10, 64)
	p := &store.Post{ID: id, AuthorID: u.ID, CommentsEnabled: true}
	if id > 0 {
		var err error
		p, err = s.db.PostByID(r.Context(), id)
		if err != nil {
			s.notFound(w, r)
			return
		}
		if !u.IsAdmin() && p.AuthorID != u.ID {
			s.forbidden(w, r)
			return
		}
	}
	p.Title = strings.TrimSpace(r.FormValue("title"))
	p.Summary = strings.TrimSpace(r.FormValue("summary"))
	p.BodyMD = strings.ReplaceAll(r.FormValue("body"), "\r\n", "\n")
	p.CommentsEnabled = formBool(r, "comments_enabled")
	if u.IsAdmin() {
		p.Pinned = formBool(r, "pinned")
	}
	if p.Title == "" {
		p.Title = "Untitled"
	}
	if slug := store.Slugify(r.FormValue("slug")); slug != "" {
		p.Slug = slug
	} else if p.Slug == "" {
		p.Slug = store.Slugify(p.Title)
		if p.Slug == "" {
			p.Slug = "post"
		}
	}
	if cover := strings.TrimSpace(r.FormValue("cover")); cover != "" || formBool(r, "remove_cover") {
		p.Cover = cover
		if formBool(r, "remove_cover") {
			p.Cover = ""
		}
	}
	if _, fh, err := r.FormFile("cover_file"); err == nil && fh.Size > 0 {
		m, err := s.saveUpload(r, fh, false, u.ID)
		if err != nil {
			s.flash(w, "err", "Cover upload failed: "+uploadErr(err))
		} else {
			p.Cover = m.URL()
		}
	}
	p.BodyHTML = template.HTML(render.HTML(p.BodyMD))
	p.ReadingMin = render.ReadingMinutes(render.Text(string(p.BodyHTML)))
	action := r.FormValue("action")
	switch action {
	case "publish":
		p.Status = store.StatusPublished
		if t, err := time.ParseInLocation("2006-01-02T15:04", r.FormValue("published_at"), time.Local); err == nil {
			p.PublishedAt = t.Unix()
		} else if p.PublishedAt == 0 {
			p.PublishedAt = time.Now().Unix()
		}
	case "unpublish":
		p.Status = store.StatusDraft
	default: // save draft, or save an already-published post without changing status
		if p.Status == "" {
			p.Status = store.StatusDraft
		}
		if p.Status == store.StatusPublished {
			if t, err := time.ParseInLocation("2006-01-02T15:04", r.FormValue("published_at"), time.Local); err == nil {
				p.PublishedAt = t.Unix()
			}
		}
	}
	tags := strings.Split(r.FormValue("tags"), ",")
	if err := s.db.SavePost(r.Context(), p, tags); err != nil {
		s.fail(w, r, err)
		return
	}
	if p.Status == store.StatusPublished {
		if p.Scheduled() {
			s.flash(w, "ok", "Scheduled for "+time.Unix(p.PublishedAt, 0).Format("Jan 2, 2006 15:04")+".")
		} else {
			s.flash(w, "ok", "Published.")
		}
	} else {
		s.flash(w, "ok", "Draft saved.")
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/posts/%d", p.ID), http.StatusSeeOther)
}

func (s *Server) adminPostDelete(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	p, err := s.db.PostByID(r.Context(), pathID(r, "id"))
	if err != nil {
		s.notFound(w, r)
		return
	}
	if !u.IsAdmin() && p.AuthorID != u.ID {
		s.forbidden(w, r)
		return
	}
	if err := s.db.DeletePost(r.Context(), p.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	s.flash(w, "ok", "Deleted “"+p.Title+"”.")
	http.Redirect(w, r, "/admin/posts", http.StatusSeeOther)
}

func (s *Server) adminPreview(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<20)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "too large", http.StatusRequestEntityTooLarge)
		return
	}
	html := render.HTML(r.FormValue("body"))
	s.writeJSON(w, 200, map[string]any{"html": html, "minutes": render.ReadingMinutes(render.Text(html))})
}

func uploadErr(err error) string {
	switch {
	case errors.Is(err, errTooBig):
		return "file is too large"
	case errors.Is(err, errBadType):
		return "unsupported file type (images: jpg/png/gif/webp; video: mp4/webm/mov)"
	}
	return "could not save the file"
}

// adminUploadAPI backs the editor's insert-image / insert-video buttons.
func (s *Server) adminUploadAPI(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	r.Body = http.MaxBytesReader(w, r.Body, int64(s.cfg.MaxVideoMB+4)<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.writeJSON(w, http.StatusRequestEntityTooLarge, map[string]any{"error": "file is too large"})
		return
	}
	_, fh, err := r.FormFile("file")
	if err != nil {
		s.writeJSON(w, 400, map[string]any{"error": "no file"})
		return
	}
	m, err := s.saveUpload(r, fh, true, u.ID)
	if err != nil {
		s.writeJSON(w, 400, map[string]any{"error": uploadErr(err)})
		return
	}
	s.writeJSON(w, 200, map[string]any{"url": m.URL(), "kind": m.Kind, "mime": m.Mime, "width": m.Width, "height": m.Height, "name": m.OriginalName})
}

// ---- comments ----

func (s *Server) adminComments(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	page := intParam(r, "page", 1)
	list, total, err := s.db.ListComments(r.Context(), status, page, 40)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := s.adminData(r, "Comments", "comments")
	d["Comments"], d["Total"], d["Status"] = list, total, status
	d["Page"], d["Pages"] = page, (total+39)/40
	d["Base"] = "/admin/comments?status=" + status
	s.render(w, r, "admin_comments", d)
}

func (s *Server) adminCommentAction(w http.ResponseWriter, r *http.Request) {
	id := pathID(r, "id")
	var err error
	switch r.PathValue("action") {
	case "approve":
		err = s.db.SetCommentStatus(r.Context(), id, store.CommentVisible)
	case "hide":
		err = s.db.SetCommentStatus(r.Context(), id, store.CommentHidden)
	case "delete":
		err = s.db.DeleteComment(r.Context(), id)
	default:
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	back := r.Referer()
	if back == "" {
		back = "/admin/comments"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// ---- users ----

func (s *Server) adminUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	role := r.URL.Query().Get("role")
	page := intParam(r, "page", 1)
	users, total, err := s.db.ListUsers(r.Context(), q, role, page, 40)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := s.adminData(r, "Users", "users")
	d["Users"], d["Total"], d["Q"], d["Role"] = users, total, q, role
	d["Page"], d["Pages"] = page, (total+39)/40
	d["Base"] = "/admin/users?q=" + q + "&role=" + role
	s.render(w, r, "admin_users", d)
}

func (s *Server) adminUserAction(w http.ResponseWriter, r *http.Request) {
	me := s.user(r)
	id := pathID(r, "id")
	target, err := s.db.UserByID(r.Context(), id)
	if err != nil {
		s.notFound(w, r)
		return
	}
	if target.ID == me.ID {
		s.flash(w, "err", "You cannot change your own account here.")
		http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
		return
	}
	switch r.PathValue("action") {
	case "role":
		role := r.FormValue("role")
		if role != store.RoleReader && role != store.RoleAuthor && role != store.RoleAdmin {
			s.notFound(w, r)
			return
		}
		err = s.db.SetRole(r.Context(), id, role)
	case "ban":
		err = s.db.SetBanned(r.Context(), id, true)
	case "unban":
		err = s.db.SetBanned(r.Context(), id, false)
	case "verify":
		err = s.db.MarkVerified(r.Context(), id)
	case "delete":
		if target.IsAdmin() {
			s.flash(w, "err", "Demote the admin before deleting the account.")
			http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
			return
		}
		err = s.db.DeleteUser(r.Context(), id)
	default:
		s.notFound(w, r)
		return
	}
	if err != nil {
		s.fail(w, r, err)
		return
	}
	back := r.Referer()
	if back == "" {
		back = "/admin/users"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// ---- media ----

func (s *Server) adminMedia(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	page := intParam(r, "page", 1)
	list, total, err := s.db.ListMedia(r.Context(), kind, page, 48)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	d := s.adminData(r, "Media", "media")
	d["Media"], d["Total"], d["Kind"] = list, total, kind
	d["Page"], d["Pages"] = page, (total+47)/48
	d["Base"] = "/admin/media?kind=" + kind
	d["MaxImageMB"], d["MaxVideoMB"] = s.cfg.MaxImageMB, s.cfg.MaxVideoMB
	s.render(w, r, "admin_media", d)
}

func (s *Server) adminMediaUpload(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	r.Body = http.MaxBytesReader(w, r.Body, int64(s.cfg.MaxVideoMB+4)<<20)
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		s.flash(w, "err", "Upload too large.")
		http.Redirect(w, r, "/admin/media", http.StatusSeeOther)
		return
	}
	files := r.MultipartForm.File["files"]
	ok, failed := 0, 0
	var lastErr string
	for _, fh := range files {
		if _, err := s.saveUpload(r, fh, true, u.ID); err != nil {
			failed++
			lastErr = uploadErr(err)
		} else {
			ok++
		}
	}
	if failed > 0 {
		s.flash(w, "err", fmt.Sprintf("%d uploaded, %d failed: %s", ok, failed, lastErr))
	} else {
		s.flash(w, "ok", fmt.Sprintf("%d file(s) uploaded.", ok))
	}
	http.Redirect(w, r, "/admin/media", http.StatusSeeOther)
}

func (s *Server) adminMediaDelete(w http.ResponseWriter, r *http.Request) {
	u := s.user(r)
	m, err := s.db.MediaByID(r.Context(), pathID(r, "id"))
	if err != nil {
		s.notFound(w, r)
		return
	}
	if !u.IsAdmin() && m.UploaderID != u.ID {
		s.forbidden(w, r)
		return
	}
	if err := s.db.DeleteMedia(r.Context(), m.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	s.removeMediaFile(m.Name)
	s.flash(w, "ok", "Deleted "+m.OriginalName+".")
	http.Redirect(w, r, "/admin/media", http.StatusSeeOther)
}

// ---- settings ----

func (s *Server) adminSettings(w http.ResponseWriter, r *http.Request) {
	d := s.adminData(r, "Site settings", "settings")
	d["St"] = s.Settings()
	d["MailFrom"] = s.cfg.MailFrom
	d["Ntfy"] = s.cfg.NtfyURL
	d["AdminEmail"] = s.cfg.AdminEmail
	s.render(w, r, "admin_settings", d)
}

func (s *Server) adminSettingsSave(w http.ResponseWriter, r *http.Request) {
	st := s.Settings()
	st.SiteName = strings.TrimSpace(r.FormValue("site_name"))
	if st.SiteName == "" {
		st.SiteName = "Blog"
	}
	st.Tagline = strings.TrimSpace(r.FormValue("tagline"))
	st.Description = strings.TrimSpace(r.FormValue("description"))
	st.AboutMD = strings.ReplaceAll(r.FormValue("about_md"), "\r\n", "\n")
	st.Nav = strings.ReplaceAll(strings.TrimSpace(r.FormValue("nav")), "\r\n", "\n")
	if st.Nav == "" {
		st.Nav = store.DefaultNav
	}
	st.ContactMD = strings.ReplaceAll(r.FormValue("contact_md"), "\r\n", "\n")
	st.ContactHTML = render.HTML(st.ContactMD)
	if e := strings.TrimSpace(r.FormValue("contact_email")); e == "" || validEmail(e) {
		st.ContactEmail = e
	}
	st.AboutHTML = render.HTML(st.AboutMD)
	st.Footer = strings.TrimSpace(r.FormValue("footer"))
	st.RegistrationOpen = formBool(r, "registration_open")
	st.CommentsOpen = formBool(r, "comments_open")
	st.CommentsModerated = formBool(r, "comments_moderated")
	st.NotifyComments = formBool(r, "notify_comments")
	st.NotifyUsers = formBool(r, "notify_users")
	if n, err := strconv.Atoi(r.FormValue("per_page")); err == nil && n > 0 && n <= 50 {
		st.PerPage = n
	}
	st.Twitter = strings.TrimSpace(r.FormValue("twitter"))
	st.GitHub = strings.TrimSpace(r.FormValue("github"))
	st.Instagram = strings.TrimSpace(r.FormValue("instagram"))
	st.YouTube = strings.TrimSpace(r.FormValue("youtube"))
	if a := strings.TrimSpace(r.FormValue("accent")); accentRe.MatchString(a) {
		st.Accent = a
	}
	if err := s.db.SaveSettings(r.Context(), st); err != nil {
		s.fail(w, r, err)
		return
	}
	s.setSettings(st)
	s.flash(w, "ok", "Settings saved.")
	http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
}
