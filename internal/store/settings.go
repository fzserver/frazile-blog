package store

import (
	"context"
	"strconv"
)

// Settings is everything the admin can change from the panel without a
// restart. Defaults apply for any key that has never been saved.
type Settings struct {
	SiteName          string
	Tagline           string
	Description       string
	AboutMD           string
	AboutHTML         string
	Footer            string
	RegistrationOpen  bool
	CommentsOpen      bool
	CommentsModerated bool // new comments wait for approval
	PerPage           int
	Twitter           string
	GitHub            string
	Instagram         string
	YouTube           string
	Accent            string
	NotifyComments    bool // ntfy on new comment
	NotifyUsers       bool // ntfy on new registration
}

func DefaultSettings() Settings {
	return Settings{
		SiteName:         "Frazile Blog",
		Tagline:          "Notes from the Frazile homelab, AI and web experiments.",
		Description:      "Frazile Blog: writing on self-hosting, AI image generation, Go, and the projects behind frazile.com.",
		Footer:           "© Frazile",
		RegistrationOpen: true,
		CommentsOpen:     true,
		PerPage:          10,
		Accent:           "#ec4899",
		NotifyComments:   true,
		NotifyUsers:      true,
	}
}

func (s *Store) LoadSettings(ctx context.Context) (Settings, error) {
	st := DefaultSettings()
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return st, err
		}
		b := v == "1"
		switch k {
		case "site_name":
			st.SiteName = v
		case "tagline":
			st.Tagline = v
		case "description":
			st.Description = v
		case "about_md":
			st.AboutMD = v
		case "about_html":
			st.AboutHTML = v
		case "footer":
			st.Footer = v
		case "registration_open":
			st.RegistrationOpen = b
		case "comments_open":
			st.CommentsOpen = b
		case "comments_moderated":
			st.CommentsModerated = b
		case "per_page":
			if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
				st.PerPage = n
			}
		case "twitter":
			st.Twitter = v
		case "github":
			st.GitHub = v
		case "instagram":
			st.Instagram = v
		case "youtube":
			st.YouTube = v
		case "accent":
			st.Accent = v
		case "notify_comments":
			st.NotifyComments = b
		case "notify_users":
			st.NotifyUsers = b
		}
	}
	return st, rows.Err()
}

func (s *Store) SaveSettings(ctx context.Context, st Settings) error {
	kv := map[string]string{
		"site_name":          st.SiteName,
		"tagline":            st.Tagline,
		"description":        st.Description,
		"about_md":           st.AboutMD,
		"about_html":         st.AboutHTML,
		"footer":             st.Footer,
		"registration_open":  strconv.Itoa(b2i(st.RegistrationOpen)),
		"comments_open":      strconv.Itoa(b2i(st.CommentsOpen)),
		"comments_moderated": strconv.Itoa(b2i(st.CommentsModerated)),
		"per_page":           strconv.Itoa(st.PerPage),
		"twitter":            st.Twitter,
		"github":             st.GitHub,
		"instagram":          st.Instagram,
		"youtube":            st.YouTube,
		"accent":             st.Accent,
		"notify_comments":    strconv.Itoa(b2i(st.NotifyComments)),
		"notify_users":       strconv.Itoa(b2i(st.NotifyUsers)),
	}
	for k, v := range kv {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, k, v); err != nil {
			return err
		}
	}
	return nil
}
